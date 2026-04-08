package handlers

import (
	"conexiuni-cluj/database"
	"conexiuni-cluj/models"
	"conexiuni-cluj/services/tranzy"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	VehicleCacheId = "VEHICLES"
)

type VehicleFilter struct {
	RouteID *int
}

func GetVehicles(tranzyClient *tranzy.Client, cacheShelfLife time.Duration, filter VehicleFilter) ([]models.Vehicle, error) {
	opts := CacheOpts[[]models.Vehicle]{}

	if filter.RouteID != nil {
		f := filter
		opts.PostProcess = func(vs []models.Vehicle) []models.Vehicle {
			var out []models.Vehicle
			for _, v := range vs {
				if f.RouteID != nil && v.RouteID != *f.RouteID {
					continue
				}
				out = append(out, v)
			}
			return out
		}
	}

	return HandleCached(VehicleCacheId, cacheShelfLife,
		func() ([]models.Vehicle, error) { return getVehiclesFromDB(filter) },
		func() ([]models.Vehicle, error) { return requestVehicles(tranzyClient) },
		storeVehiclesInDB,
		opts,
	)
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

func getVehiclesFromDB(filter VehicleFilter) ([]models.Vehicle, error) {
	query := `SELECT * FROM vehicles`
	var args []any
	var conditions []string

	if filter.RouteID != nil {
		conditions = append(conditions, "route_id = ?")
		args = append(args, *filter.RouteID)
	}
	conditions = append(conditions, "trip_id != '-1'")
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	rows, err := database.DB.Query(query, args...)
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
