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
	TripCacheId = "TRIPS"
)

func GetTrips(c fiber.Ctx, tranzyClient *tranzy.Client, cacheShelfLife time.Duration) error {
	return HandleCachedData(
		c,
		TripCacheId,
		cacheShelfLife,
		func() ([]models.Trip, error) { return getTripsFromDB() },
		func() ([]models.Trip, error) { return requestTrips(tranzyClient) },
		storeTripsInDB,
		true,
	)
}

func GetTripsByRouteID(c fiber.Ctx, tranzyClient *tranzy.Client, cacheShelfLife time.Duration, routeID int) error {
	isCacheValid := database.IsCacheValid(TripCacheId)
	if isCacheValid {
		trips, err := getTripsFromDB(routeID)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": err.Error(),
			})
		}
		return c.JSON(trips)
	}

	trips, err := requestTrips(tranzyClient)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	go func() {
		_ = storeTripsInDB(trips)
		_ = database.UpdateCache(TripCacheId, cacheShelfLife.Milliseconds())
	}()

	var filteredTrips []models.Trip
	for _, trip := range trips {
		if trip.RouteID == routeID {
			filteredTrips = append(filteredTrips, trip)
		}
	}
	return c.JSON(filteredTrips)
}

func requestTrips(tranzyClient *tranzy.Client) ([]models.Trip, error) {
	data, err := tranzyClient.DoRequest("/trips", nil)
	if err != nil {
		return nil, err
	}

	var trips []models.Trip
	if err := json.Unmarshal(data, &trips); err != nil {
		return nil, fmt.Errorf("failed to unmarshal trips: %w", err)
	}

	return trips, nil
}

func getTripsFromDB(options ...int) ([]models.Trip, error) {
	var rows *sql.Rows
	var err error

	if len(options) > 0 {
		routeID := options[0]
		rows, err = database.DB.Query(`SELECT * FROM trips WHERE route_id = ?`, routeID)
	} else {
		rows, err = database.DB.Query(`SELECT * FROM trips`)
	}
	if err != nil {
		return nil, fmt.Errorf("error querying trips: %w", err)
	}

	defer func(rows *sql.Rows) {
		_ = rows.Close()
	}(rows)

	var trips []models.Trip
	for rows.Next() {
		var trip models.Trip
		err := rows.Scan(
			&trip.TripID,
			&trip.RouteID,
			&trip.DirectionID,
			&trip.TripHeadsign,
			&trip.BlockID,
			&trip.ShapeID,
			&trip.WheelchairAccessible,
			&trip.BikesAllowed,
		)
		if err != nil {
			return nil, fmt.Errorf("error scanning trip: %w", err)
		}
		trips = append(trips, trip)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error reading trips: %w", err)
	}

	return trips, nil
}

func storeTripsInDB(trips []models.Trip) error {
	stmt, err := database.DB.Prepare(`
		INSERT OR REPLACE INTO trips
		(trip_id, route_id, direction_id, trip_headsign, block_id, shape_id, wheelchair_accessible, bikes_allowed)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("error preparing statement: %w", err)
	}

	defer func(stmt *sql.Stmt) {
		_ = stmt.Close()
	}(stmt)

	for _, trip := range trips {
		if _, err := stmt.Exec(
			trip.TripID,
			trip.RouteID,
			trip.DirectionID,
			trip.TripHeadsign,
			trip.BlockID,
			trip.ShapeID,
			trip.WheelchairAccessible,
			trip.BikesAllowed,
		); err != nil {
			return fmt.Errorf("error inserting trip: %w", err)
		}
	}

	return nil
}
