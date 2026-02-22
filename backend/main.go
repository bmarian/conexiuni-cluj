package main

import (
	"conexiuni-cluj/database"
	"conexiuni-cluj/handlers"
	ctpcj "conexiuni-cluj/services/ctp-cj"
	"conexiuni-cluj/services/tranzy"
	"database/sql"
	"log"
	"os"
	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/fiber/v3/middleware/static"
)

func main() {
	config := Load()
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

	tranzyAPIKey := config.TranzyApiKey
	if tranzyAPIKey == "" {
		log.Fatal("TRANZY_API_KEY not set in environment")
	}

	tranzyClient := tranzy.NewClient(config.TranzyBaseUrl, tranzyAPIKey, config.ClujAgencyId)
	ctpCjClient := ctpcj.NewClient(config.CtpCsvBaseUrl)

	app := fiber.New(fiber.Config{
		AppName: "Conexiuni Cluj",
	})
	app.Use(logger.New())
	app.Use(cors.New(cors.Config{
		//AllowOrigins: []string{"http://localhost:5173"},
		AllowHeaders: []string{"Origin", "Content-Type", "Accept"},
	}))

	// Tranzy API routes
	api := app.Group("/api")
	api.Get("/routes", func(c fiber.Ctx) error {
		return handlers.GetRoutes(c, tranzyClient, config.RouteCacheShelfLife)
	})
	api.Get("/shapes", func(c fiber.Ctx) error {
		return handlers.GetShapes(c, tranzyClient, config.ShapeCacheShelfLife)
	})
	api.Get("/vehicles", func(c fiber.Ctx) error {
		return handlers.GetVehicles(c, tranzyClient, config.VehicleCacheShelfLife)
	})
	api.Get("/vehicles/:routeID", func(c fiber.Ctx) error {
		routeID, err := strconv.Atoi(c.Params("routeID"))
		if err != nil {
			return c.Status(fiber.StatusBadRequest).SendString("Invalid route ID")
		}
		return handlers.GetVehiclesByRouteID(c, tranzyClient, config.VehicleCacheShelfLife, routeID)
	})
	api.Get("/trips", func(c fiber.Ctx) error {
		return handlers.GetTrips(c, tranzyClient, config.TripCacheShelfLife)
	})
	api.Get("/trips/:routeID", func(c fiber.Ctx) error {
		routeID, err := strconv.Atoi(c.Params("routeID"))
		if err != nil {
			return c.Status(fiber.StatusBadRequest).SendString("Invalid route ID")
		}
		return handlers.GetTripsByRouteID(c, tranzyClient, config.TripCacheShelfLife, routeID)
	})

	// CTP Cj API routes
	api.Get("/timetable/:routeShortName", func(c fiber.Ctx) error {
		routeShortName := c.Params("routeShortName")
		return handlers.GetTimetable(c, ctpCjClient, config.TimetableCacheShelfLife, routeShortName)
	})

	// Serve static files
	if _, err := os.Stat("./dist"); err == nil {
		app.Use("/", static.New("./dist", static.Config{
			Browse: false,
		}))

		// SPA fallback
		app.Use("*", func(c fiber.Ctx) error {
			return c.SendFile("./dist/index.html")
		})

		log.Println("Serving frontend from ./dist")
	} else {
		log.Println("Frontend dist folder not found. Run 'npm run build' in frontend directory.")
	}

	log.Printf("Server listening on http://localhost:%s", config.Port)
	log.Fatal(app.Listen(":" + config.Port))
}
