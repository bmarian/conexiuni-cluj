package handlers

import (
	"database/sql"
	"log"

	"conexiuni-cluj/models"

	"github.com/gofiber/fiber/v3"
)

type VehicleHandler struct {
	db *sql.DB
}

func NewVehicleHandler(db *sql.DB) *VehicleHandler {
	return &VehicleHandler{db: db}
}

// GetAllVehicles returns all vehicles
func (h *VehicleHandler) GetAllVehicles(c fiber.Ctx) error {
	query := `SELECT * FROM vehicles`

	rows, err := h.db.Query(query)
	if err != nil {
		log.Printf("Error querying vehicles: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch vehicles",
		})
	}
	defer rows.Close()

	var vehicles []models.Vehicle

	for rows.Next() {
		var v models.Vehicle
		err := rows.Scan(
			&v.ID,
			&v.Label,
			&v.Latitude,
			&v.Longitude,
			&v.Timestamp,
			&v.VehicleType,
			&v.BikeAccessible,
			&v.WheelchairAccessible,
			&v.Speed,
			&v.RouteID,
			&v.TripID,
		)
		if err != nil {
			log.Printf("Error scanning vehicle: %v", err)
			continue
		}
		vehicles = append(vehicles, v)
	}

	return c.JSON(vehicles)
}
