package main

import (
	"bufio"
	"conexiuni-cluj/database"
	"conexiuni-cluj/handlers"
	"conexiuni-cluj/models"
	ctpcj "conexiuni-cluj/services/ctp-cj"
	"conexiuni-cluj/services/tranzy"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/fiber/v3/middleware/static"
)

func main() {
	config := Load()

	logs, err := setupLogging(config.LogDir, config.LogRetentionDays)
	if err != nil {
		log.Fatalf("Failed to initialize logging: %v", err)
	}
	defer func() {
		if err := logs.close(); err != nil {
			log.Printf("Warning: failed to close log file: %v", err)
		}
	}()

	log.Printf("Starting Conexiuni Cluj server in %s mode on port %s", config.Environment, config.Port)

	if err := database.Connect(config.DatabasePath); err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer func(DB *sql.DB) {
		_ = DB.Close()
	}(database.DB)

	if err := database.InitSchemas(); err != nil {
		log.Fatalf("Failed to initialize database schemas: %v", err)
	}
	database.StartVacuumScheduler()
	database.StartDSTCacheInvalidationScheduler()

	tranzyAPIKey := config.TranzyApiKey
	if tranzyAPIKey == "" {
		log.Fatal("TRANZY_API_KEY not set in environment")
	}

	tranzyClient := tranzy.NewClient(config.TranzyBaseUrl, tranzyAPIKey, config.ClujAgencyId, config.TranzyRateLimit, config.TranzyVehiclesDailyQuota, config.TranzyDefaultDailyQuota, dbQuotaPersister{})
	log.Printf("Tranzy quota on startup: vehicles=%d/%d used",
		tranzyClient.VehiclesQuotaLimit()-tranzyClient.VehiclesQuotaRemaining(), tranzyClient.VehiclesQuotaLimit())
	ctpCjClient := ctpcj.NewClient(config.CtpCsvBaseUrl, config.CtpCjRateLimit)

	weekdaySlots, err := handlers.ParseSchedule(config.VehicleSchedule)
	if err != nil {
		log.Fatalf("VEHICLE_SCHEDULE: %v", err)
	}
	weekendSlots, err := handlers.ParseSchedule(config.VehicleScheduleWeekend)
	if err != nil {
		log.Fatalf("VEHICLE_SCHEDULE_WEEKEND: %v", err)
	}
	handlers.InitVehicleHub(tranzyClient, handlers.VehicleIntervalConfig{
		Weekday:     weekdaySlots,
		Weekend:     weekendSlots,
		MinInterval: config.VehicleMinInterval,
		MaxInterval: config.VehicleMaxInterval,
	})

	app := fiber.New(fiber.Config{
		AppName: "Conexiuni Cluj",
	})
	app.Use(logger.New(logger.Config{
		Stream:     logs.accessOut,
		Format:     "${time} | ${status} | ${latency} | ${ip} | ${method} | ${path} | ${queryParams} | ${body}\n",
		TimeFormat: StandardLogTimestampLayout,
		TimeZone:   "Local",
	}))
	app.Use(cors.New(cors.Config{
		AllowHeaders: []string{"Origin", "Content-Type", "Accept"},
	}))

	api := app.Group("/api")
	api.Get("/routes", func(c fiber.Ctx) error {
		filter := handlers.RouteFilter{}

		if routeIDStr := c.Query("route_id"); routeIDStr != "" {
			routeID, err := strconv.Atoi(routeIDStr)
			if err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid route_id"})
			}
			filter.RouteID = &routeID
		}
		if routeShortName := c.Query("route_short_name"); routeShortName != "" {
			filter.RouteShortName = &routeShortName
		}

		data, err := handlers.GetRoutes(tranzyClient, config.RouteCacheShelfLife, filter)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		if filter.RouteID == nil && filter.RouteShortName == nil && handlers.Availability.IsReady() {
			filtered := make([]models.Route, 0, len(data))
			for _, r := range data {
				if handlers.Availability.RouteHasTimetable(r.RouteShortName) {
					filtered = append(filtered, r)
				}
			}
			data = filtered
		}
		return c.JSON(data)
	})
	api.Get("/stops", func(c fiber.Ctx) error {
		filter := handlers.StopFilter{}

		if stopIDStr := c.Query("stop_id"); stopIDStr != "" {
			stopID, err := strconv.Atoi(stopIDStr)
			if err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid stop_id"})
			}
			filter.StopID = &stopID
		}

		data, err := handlers.GetStops(tranzyClient, config.StopCacheShelfLife, filter)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		if filter.StopID == nil && handlers.Availability.IsReady() {
			filtered := make([]models.Stop, 0, len(data))
			for _, s := range data {
				if handlers.Availability.StopHasBuses(s.StopID) {
					filtered = append(filtered, s)
				}
			}
			data = filtered
		}
		return c.JSON(data)
	})
	api.Get("/shapes", func(c fiber.Ctx) error {
		filter := handlers.ShapeFilter{}

		if shapeIDStr := c.Query("shape_id"); shapeIDStr != "" {
			filter.ShapeID = &shapeIDStr
		}

		if shapeIDsStr := c.Query("shape_ids"); shapeIDsStr != "" {
			for _, id := range strings.Split(shapeIDsStr, ",") {
				if id = strings.TrimSpace(id); id != "" {
					filter.ShapeIDs = append(filter.ShapeIDs, id)
				}
			}
		}

		data, err := handlers.GetShapes(tranzyClient, config.ShapeCacheShelfLife, filter)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(data)
	})
	api.Get("/vehicles", func(c fiber.Ctx) error {
		filter := handlers.VehicleFilter{}

		if routeIDStr := c.Query("route_id"); routeIDStr != "" {
			routeID, err := strconv.Atoi(routeIDStr)
			if err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid route_id"})
			}
			filter.RouteID = &routeID
		}

		if tripIDStr := c.Query("trip_id"); tripIDStr != "" {
			filter.TripID = &tripIDStr
		}

		if tripIDsStr := c.Query("trip_ids"); tripIDsStr != "" {
			for _, id := range strings.Split(tripIDsStr, ",") {
				if id = strings.TrimSpace(id); id != "" {
					filter.TripIDs = append(filter.TripIDs, id)
				}
			}
		}

		data, err := handlers.GetVehicles(tranzyClient, handlers.VehicleHub.CurrentInterval(), filter)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(data)
	})
	api.Get("/vehicles/stream", func(c fiber.Ctx) error {
		var tripIDs []string
		if s := c.Query("trip_ids"); s != "" {
			for _, id := range strings.Split(s, ",") {
				if id = strings.TrimSpace(id); id != "" {
					tripIDs = append(tripIDs, id)
				}
			}
		}
		if len(tripIDs) == 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "trip_ids required"})
		}

		c.Set("Content-Type", "text/event-stream")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")
		// Required for SSE behind nginx.
		c.Set("X-Accel-Buffering", "no")

		sub, id := handlers.VehicleHub.Subscribe(tripIDs)

		return c.SendStreamWriter(func(w *bufio.Writer) {
			defer handlers.VehicleHub.Unsubscribe(id)

			keep := time.NewTicker(25 * time.Second)
			defer keep.Stop()

			for {
				select {
				case batch, ok := <-sub.Ch():
					if !ok {
						return
					}
					data, err := json.Marshal(batch)
					if err != nil {
						continue
					}
					if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
						return
					}
					if err := w.Flush(); err != nil {
						return
					}
				case <-keep.C:
					if _, err := w.WriteString(": ping\n\n"); err != nil {
						return
					}
					if err := w.Flush(); err != nil {
						return
					}
				}
			}
		})
	})
	api.Get("/trips", func(c fiber.Ctx) error {
		filter := handlers.TripFilter{}

		if routeIDStr := c.Query("route_id"); routeIDStr != "" {
			routeID, err := strconv.Atoi(routeIDStr)
			if err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid route_id"})
			}
			filter.RouteID = &routeID
		}
		if tripIDStr := c.Query("trip_id"); tripIDStr != "" {
			filter.TripID = &tripIDStr
		}

		data, err := handlers.GetTrips(tranzyClient, config.TripCacheShelfLife, filter)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(data)
	})
	api.Get("/stop_times", func(c fiber.Ctx) error {
		filter := handlers.StopTimeFilter{}
		if routeShortName := c.Query("route_short_name"); routeShortName != "" {
			filter.RouteShortName = &routeShortName
		}

		data, err := handlers.GetStopTimes(tranzyClient, models.CacheTimes{
			ShapeCacheShelfLife:       config.ShapeCacheShelfLife,
			RouteCacheShelfLife:       config.RouteCacheShelfLife,
			TripCacheShelfLife:        config.TripCacheShelfLife,
			StopCacheShelfLife:        config.StopCacheShelfLife,
			StopTimeCacheShelfLife:    config.StopTimeCacheShelfLife,
			APIStopTimeCacheShelfLife: config.APIStopTimeCacheShelfLife,
		}, filter)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(data)
	})
	api.Get("/stop_info", func(c fiber.Ctx) error {
		filter := handlers.StopFilter{}
		if stopIDStr := c.Query("stop_id"); stopIDStr != "" {
			stopID, err := strconv.Atoi(stopIDStr)
			if err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid stop_id"})
			}
			filter.StopID = &stopID
		}

		data, err := handlers.GetStopInfo(tranzyClient, ctpCjClient, models.CacheTimes{
			ShapeCacheShelfLife:       config.ShapeCacheShelfLife,
			RouteCacheShelfLife:       config.RouteCacheShelfLife,
			TripCacheShelfLife:        config.TripCacheShelfLife,
			StopCacheShelfLife:        config.StopCacheShelfLife,
			StopTimeCacheShelfLife:    config.StopTimeCacheShelfLife,
			APIStopTimeCacheShelfLife: config.APIStopTimeCacheShelfLife,
			StopInfoCacheShelfLife:    config.StopInfoCacheShelfLife,
			TimetableCacheShelfLife:   config.TimetableCacheShelfLife,
		}, filter)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(data)
	})

	api.Get("/timetable", func(c fiber.Ctx) error {
		routeShortName := c.Query("route_short_name")
		data, err := handlers.GetTimetable(ctpCjClient, config.TimetableCacheShelfLife, routeShortName)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(data)
	})

	if _, err := os.Stat("./dist"); err == nil {
		app.Use("/", static.New("./dist", static.Config{
			Browse: false,
		}))

		app.Use("*", func(c fiber.Ctx) error {
			return c.SendFile("./dist/index.html")
		})

		log.Println("Serving frontend from ./dist")
	} else {
		log.Println("Frontend dist folder not found. Run 'npm run build' in frontend directory.")
	}

	handlers.StartWarmup(tranzyClient, ctpCjClient, models.CacheTimes{
		ShapeCacheShelfLife:       config.ShapeCacheShelfLife,
		RouteCacheShelfLife:       config.RouteCacheShelfLife,
		TripCacheShelfLife:        config.TripCacheShelfLife,
		StopCacheShelfLife:        config.StopCacheShelfLife,
		StopTimeCacheShelfLife:    config.StopTimeCacheShelfLife,
		APIStopTimeCacheShelfLife: config.APIStopTimeCacheShelfLife,
		StopInfoCacheShelfLife:    config.StopInfoCacheShelfLife,
		TimetableCacheShelfLife:   config.TimetableCacheShelfLife,
	})

	log.Printf("Server listening on http://localhost:%s", config.Port)
	log.Fatal(app.Listen(":" + config.Port))
}
