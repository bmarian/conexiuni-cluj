package handlers

import (
	"conexiuni-cluj/models"
	"conexiuni-cluj/services/tranzy"
	"database/sql"
	"fmt"
	"time"
)

const TripCacheId = "TRIPS"

type TripFilter struct {
	RouteID *int
	TripID  *string
}

func GetTrips(tranzyClient *tranzy.Client, cacheShelfLife time.Duration, filter TripFilter) ([]models.Trip, error) {
	opts := CacheOpts[[]models.Trip]{}
	if filter.RouteID != nil || filter.TripID != nil {
		f := filter
		opts.PostProcess = func(ts []models.Trip) []models.Trip {
			out := make([]models.Trip, 0)
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
		func() ([]models.Trip, error) { return tranzyFetch[[]models.Trip](tranzyClient, "/trips") },
		storeTripsInDB,
		opts,
	)
}

func getTripsFromDB(filter TripFilter) ([]models.Trip, error) {
	var conditions []string
	var args []any
	if filter.RouteID != nil {
		conditions = append(conditions, "route_id = ?")
		args = append(args, *filter.RouteID)
	}
	if filter.TripID != nil {
		conditions = append(conditions, "trip_id = ?")
		args = append(args, *filter.TripID)
	}
	return queryRows(`SELECT * FROM trips`+whereClause(conditions), args,
		func(rows *sql.Rows) (models.Trip, error) {
			var t models.Trip
			err := rows.Scan(&t.TripID, &t.RouteID, &t.DirectionID, &t.TripHeadsign, &t.BlockID, &t.ShapeID, &t.WheelchairAccessible, &t.BikesAllowed)
			return t, err
		})
}

func storeTripsInDB(trips []models.Trip) error {
	return batchExec(`
		INSERT OR REPLACE INTO trips
		(trip_id, route_id, direction_id, trip_headsign, block_id, shape_id, wheelchair_accessible, bikes_allowed)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		func(stmt *sql.Stmt) error {
			for _, t := range trips {
				if _, err := stmt.Exec(t.TripID, t.RouteID, t.DirectionID, t.TripHeadsign, t.BlockID, t.ShapeID, t.WheelchairAccessible, t.BikesAllowed); err != nil {
					return fmt.Errorf("error inserting trip: %w", err)
				}
			}
			return nil
		})
}
