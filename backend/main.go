package main

import (
	"conexiuni-cluj/database"
	"conexiuni-cluj/handlers"
	"conexiuni-cluj/models"
	ctpcj "conexiuni-cluj/services/ctp-cj"
	"conexiuni-cluj/services/tranzy"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/fiber/v3/middleware/static"
)

func clientIPForLog(c fiber.Ctx) string {
	for forwardedIP := range strings.SplitSeq(c.Get(fiber.HeaderXForwardedFor), ",") {
		ip := strings.TrimSpace(forwardedIP)
		if ip != "" {
			return ip
		}
	}

	return c.IP()
}

func clientIDForLog(c fiber.Ctx, salt string) string {
	ip := clientIPForLog(c)
	if ip == "" {
		return "-"
	}

	hasher := hmac.New(sha256.New, []byte(salt))
	_, _ = hasher.Write([]byte(ip))
	return hex.EncodeToString(hasher.Sum(nil)[:8])
}

func main() {
	_ = os.Setenv("TZ", "Europe/Bucharest")
	config := Load()
	logHashSalt := config.LogIPHashSalt
	if logHashSalt == "" {
		logHashSalt = "conexiuni-cluj-default-salt"
		log.Printf("Warning: LOG_IP_HASH_SALT not set, using default salt; set LOG_IP_HASH_SALT for stronger pseudonymization")
	}

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
	ctpCjClient := ctpcj.NewClient(config.CtpCsvBaseUrl, config.CtpCjRateLimit)

	weekdaySlots, err := handlers.ParseSchedule(config.VehicleSchedule)
	if err != nil {
		log.Fatalf("VEHICLE_SCHEDULE: %v", err)
	}
	weekendSlots, err := handlers.ParseSchedule(config.VehicleScheduleWeekend)
	if err != nil {
		log.Fatalf("VEHICLE_SCHEDULE_WEEKEND: %v", err)
	}
	handlers.SetOTPMaxMemory(config.OtpMaxMemory)
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
		Stream: logs.accessOut,
		Format: "${time} | status=${status} | latency=${latency} | user=${clientId} | method=${method} | url=${url} | ua=${ua} | error=${error}\n",
		CustomTags: map[string]logger.LogFunc{"clientId": func(output logger.Buffer, c fiber.Ctx, _ *logger.Data, _ string) (int, error) {
			return output.WriteString(clientIDForLog(c, logHashSalt))
		}},
		TimeFormat: StandardLogTimestampLayout,
		TimeZone:   "Europe/Bucharest",
	}))
	app.Use(cors.New(cors.Config{
		AllowHeaders: []string{"Origin", "Content-Type", "Accept"},
	}))

	cacheTimes := models.CacheTimes{
		ShapeCacheShelfLife:       config.ShapeCacheShelfLife,
		RouteCacheShelfLife:       config.RouteCacheShelfLife,
		TripCacheShelfLife:        config.TripCacheShelfLife,
		StopCacheShelfLife:        config.StopCacheShelfLife,
		StopTimeCacheShelfLife:    config.StopTimeCacheShelfLife,
		APIStopTimeCacheShelfLife: config.APIStopTimeCacheShelfLife,
		StopInfoCacheShelfLife:    config.StopInfoCacheShelfLife,
		TimetableCacheShelfLife:   config.TimetableCacheShelfLife,
	}

	api := app.Group("/api")
	handlers.RegisterAPIRoutes(api, tranzyClient, ctpCjClient, cacheTimes)

	if _, err := os.Stat("./dist"); err == nil {
		app.Get("/robots.txt", func(c fiber.Ctx) error {
			c.Set("Cache-Control", "public, max-age=86400")
			return c.SendFile("./dist/robots.txt")
		})
		app.Get("/sitemap.xml", func(c fiber.Ctx) error {
			c.Set("Cache-Control", "public, max-age=3600")
			return c.SendFile("./dist/sitemap.xml")
		})
		app.Get("/sw.js", func(c fiber.Ctx) error {
			c.Set("Cache-Control", "no-cache, no-store, must-revalidate")
			c.Set("Service-Worker-Allowed", "/")
			return c.SendFile("./dist/sw.js")
		})
		app.Get("/registerSW.js", func(c fiber.Ctx) error {
			c.Set("Cache-Control", "no-cache, no-store, must-revalidate")
			return c.SendFile("./dist/registerSW.js")
		})
		app.Get("/manifest.webmanifest", func(c fiber.Ctx) error {
			c.Set("Cache-Control", "no-cache")
			c.Set("Content-Type", "application/manifest+json")
			return c.SendFile("./dist/manifest.webmanifest")
		})

		app.Get("/route/:routeId/:direction", handlers.RouteOGHandler)
		app.Get("/stop/:stopId", handlers.StopOGHandler)
		app.Get("/plan", handlers.PlanOGHandler)

		app.Use("/", static.New("./dist", static.Config{
			Browse: false,
		}))

		app.Use("*", func(c fiber.Ctx) error {
			path := c.Path()
			if strings.HasPrefix(path, "/api/") {
				return c.SendStatus(fiber.StatusNotFound)
			}
			if ext := filepath.Ext(path); ext != "" && ext != ".html" {
				return c.SendStatus(fiber.StatusNotFound)
			}
			return c.SendFile("./dist/index.html")
		})

		log.Println("Serving frontend from ./dist")
	} else {
		log.Println("Frontend dist folder not found. Run 'npm run build' in frontend directory.")
	}

	handlers.StartWarmup(tranzyClient, ctpCjClient, cacheTimes)

	defer handlers.CleanupOTP()

	listenAddr := ":" + config.Port
	if config.Environment == "development" && runtime.GOOS == "windows" {
		listenAddr = "0.0.0.0:" + config.Port
	}
	log.Printf("Server listening on http://%s", listenAddr)
	log.Fatal(app.Listen(listenAddr))
}
