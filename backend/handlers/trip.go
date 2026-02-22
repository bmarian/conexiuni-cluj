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
	TripCacheId = "TRIPS"
)

type TripFilter struct {
	RouteID *int
	TripID  *string
}

func GetTrips(tranzyClient *tranzy.Client, cacheShelfLife time.Duration, filter TripFilter) ([]models.Trip, error) {
	opts := CacheOpts[[]models.Trip]{}

	if filter.RouteID != nil || filter.TripID != nil {
		f := filter
		opts.PostProcess = func(ts []models.Trip) []models.Trip {
			var out []models.Trip
			for _, t := range ts {
				if f.RouteID != nil && t.RouteID != *f.RouteID {
					continue
				}
				if f.TripID != nil && t.TripID != *f.TripID {
					continue
				}
				out = append(out, t)
			}
			return out
		}
	} else {
		opts.Optimize = true
	}

	return HandleCached(TripCacheId, cacheShelfLife,
		func() ([]models.Trip, error) { return getTripsFromDB(filter) },
		func() ([]models.Trip, error) { return requestTrips(tranzyClient) },
		storeTripsInDB,
		opts,
	)
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

func getTripsFromDB(filter TripFilter) ([]models.Trip, error) {
	query := `SELECT * FROM trips`
	var args []any
	var conditions []string

	if filter.RouteID != nil {
		conditions = append(conditions, "route_id = ?")
		args = append(args, *filter.RouteID)
	}
	if filter.TripID != nil {
		conditions = append(conditions, "trip_id = ?")
		args = append(args, *filter.TripID)
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	rows, err := database.DB.Query(query, args...)
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
