package handlers

import (
	"conexiuni-cluj/database"
	"conexiuni-cluj/models"
	"conexiuni-cluj/services/tranzy"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v3"
)

const (
	VehicleCacheId = "VEHICLES"
)

func GetVehicles(c fiber.Ctx, tranzyClient *tranzy.Client, cacheShelfLife time.Duration) error {
	return HandleCachedData(
		c,
		VehicleCacheId,
		cacheShelfLife,
		func() ([]models.Vehicle, error) { return getVehiclesFromDB() },
		func() ([]models.Vehicle, error) { return requestVehicles(tranzyClient) },
		storeVehiclesInDB,
		false,
	)
}

func GetVehiclesByRouteID(c fiber.Ctx, tranzyClient *tranzy.Client, cacheShelfLife time.Duration, routeID int) error {
	isCacheValid := database.IsCacheValid(VehicleCacheId)
	if isCacheValid {
		vehicles, err := getVehiclesFromDB(routeID)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": err.Error(),
			})
		}
		return c.JSON(vehicles)
	}

	vehicles, err := requestVehicles(tranzyClient)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	go func() {
		_ = storeVehiclesInDB(vehicles)
		_ = database.UpdateCache(VehicleCacheId, cacheShelfLife.Milliseconds())
	}()

	var filteredVehicles []models.Vehicle
	for _, vehicle := range vehicles {
		if vehicle.RouteID == routeID {
			filteredVehicles = append(filteredVehicles, vehicle)
		}
	}
	return c.JSON(filteredVehicles)
}

func requestVehicles(tranzyClient *tranzy.Client) ([]models.Vehicle, error) {
	data, err := tranzyClient.DoRequest("/vehicles", nil)
	if err != nil {
		return nil, err
	}

	var vehicles []models.Vehicle
	if err := json.Unmarshal(data, &vehicles); err != nil {
		return nil, fmt.Errorf("failed to unmarshal vehicles: %w", err)
	}

	return vehicles, nil
}

func getVehiclesFromDB(options ...int) ([]models.Vehicle, error) {
	var rows *sql.Rows
	var err error

	if len(options) > 0 {
		routeID := options[0]
		rows, err = database.DB.Query(`SELECT * FROM vehicles where route_id = ?`, routeID)
	} else {
		rows, err = database.DB.Query(`SELECT * FROM vehicles`)
	}
	if err != nil {
		return nil, fmt.Errorf("error querying vehicles: %w", err)
	}

	defer func(rows *sql.Rows) {
		_ = rows.Close()
	}(rows)

	var vehicles []models.Vehicle
	for rows.Next() {
		var vehicle models.Vehicle
		err := rows.Scan(
			&vehicle.ID,
			&vehicle.Label,
			&vehicle.Latitude,
			&vehicle.Longitude,
			&vehicle.Timestamp,
			&vehicle.VehicleType,
			&vehicle.BikeAccessible,
			&vehicle.WheelchairAccessible,
			&vehicle.Speed,
			&vehicle.RouteID,
			&vehicle.TripID,
		)
		if err != nil {
			return nil, fmt.Errorf("error scanning vehicles: %w", err)
		}
		vehicles = append(vehicles, vehicle)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error reading vehicles: %w", err)
	}

	return vehicles, nil
}

func storeVehiclesInDB(vehicles []models.Vehicle) error {
	stmt, err := database.DB.Prepare(`
		INSERT OR REPLACE INTO vehicles
		(id, label, latitude, longitude, timestamp, vehicle_type, bike_accessible, wheelchair_accessible, speed, route_id, trip_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("error preparing statement: %w", err)
	}

	defer func(stmt *sql.Stmt) {
		_ = stmt.Close()
	}(stmt)

	for _, vehicle := range vehicles {
		if _, err := stmt.Exec(
			vehicle.ID,
			vehicle.Label,
			vehicle.Latitude,
			vehicle.Longitude,
			vehicle.Timestamp,
			vehicle.VehicleType,
			vehicle.BikeAccessible,
			vehicle.WheelchairAccessible,
			vehicle.Speed,
			vehicle.RouteID,
			vehicle.TripID,
		); err != nil {
			return fmt.Errorf("error inserting vehicle: %w", err)
		}
	}

	return nil
}
