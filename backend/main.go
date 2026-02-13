package main

import (
	"log"

	"conexiuni-cluj/database"
	"conexiuni-cluj/handlers"

	"github.com/gofiber/fiber/v3"
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
		AppName: "Conexiuni Cluj API",
	})

	vehicleHandler := handlers.NewVehicleHandler(database.DB)

	api := app.Group("/api")
	api.Get("/vehicles", vehicleHandler.GetAllVehicles)

	log.Printf("Server listening on http://localhost:%s", config.Port)
	log.Fatal(app.Listen(":" + config.Port))
}
