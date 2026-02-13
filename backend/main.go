package main

import (
	"log"
	"os"

	"conexiuni-cluj/database"
	"conexiuni-cluj/handlers"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/static"
)

func main() {
	config := Load()
	log.Printf("Starting Conexiuni Cluj server in %s mode on port %s", config.Environment, config.Port)

	if err := database.Connect(config.DatabasePath); err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.DB.Close()

	if err := database.InitSchemas(); err != nil {
		log.Fatalf("Failed to initialize database schemas: %v", err)
	}

	app := fiber.New(fiber.Config{
		AppName: "Conexiuni Cluj",
	})

	vehicleHandler := handlers.NewVehicleHandler(database.DB)

	// API routes
	api := app.Group("/api")
	api.Get("/vehicles", vehicleHandler.GetAllVehicles)

	// Serve static files
	if _, err := os.Stat("./dist"); err == nil {
		// Serve static files
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
