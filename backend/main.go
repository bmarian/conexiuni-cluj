package main

import (
	"conexiuni-cluj/handlers"
	"conexiuni-cluj/services/tranzy"
	"database/sql"
	"log"
	"os"

	"conexiuni-cluj/database"

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

	tranzyAPIKey := config.TranzyApiKey
	if tranzyAPIKey == "" {
		log.Fatal("TRANZY_API_KEY not set in environment")
	}

	tranzyClient := tranzy.NewClient(config.TranzyBaseUrl, tranzyAPIKey, config.ClujAgencyId)

	app := fiber.New(fiber.Config{
		AppName: "Conexiuni Cluj",
	})
	app.Use(logger.New())
	app.Use(cors.New(cors.Config{
		//AllowOrigins: []string{"http://localhost:5173"},
		AllowHeaders: []string{"Origin", "Content-Type", "Accept"},
	}))

	// API routes
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
