package handlers

import (
	"conexiuni-cluj/models"
	"conexiuni-cluj/services/tranzy"
	"database/sql"
	"fmt"
	"time"
)

const StopsCacheId = "STOPS"

type StopFilter struct {
	StopID *int
}

func GetStops(tranzyClient *tranzy.Client, cacheShelfLife time.Duration, filter StopFilter) ([]models.Stop, error) {
	opts := CacheOpts[[]models.Stop]{}
	if filter.StopID != nil {
		f := filter
		opts.PostProcess = func(ss []models.Stop) []models.Stop {
			out := make([]models.Stop, 0)
			for _, s := range ss {
				if f.StopID != nil && s.StopID != *f.StopID {
					continue
				}
				out = append(out, s)
			}
			return out
		}
	} else {
		opts.Optimize = true
	}
	return HandleCached(StopsCacheId, cacheShelfLife,
		func() ([]models.Stop, error) { return getStopsFromDB(filter) },
		func() ([]models.Stop, error) { return tranzyFetch[[]models.Stop](tranzyClient, "/stops") },
		storeStopsInDB,
		opts,
	)
}

func getStopsFromDB(filter StopFilter) ([]models.Stop, error) {
	var conditions []string
	var args []any
	if filter.StopID != nil {
		conditions = append(conditions, "stop_id = ?")
		args = append(args, *filter.StopID)
	}
	return queryRows(`SELECT * FROM stops`+whereClause(conditions), args,
		func(rows *sql.Rows) (models.Stop, error) {
			var s models.Stop
			err := rows.Scan(&s.StopID, &s.StopName, &s.StopDesc, &s.StopLat, &s.StopLon, &s.LocationType, &s.StopCode)
			return s, err
		})
}

func storeStopsInDB(stops []models.Stop) error {
	return batchExec(`
		INSERT OR REPLACE INTO stops
		(stop_id, stop_name, stop_desc, stop_lat, stop_lon, location_type, stop_code)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		func(stmt *sql.Stmt) error {
			for _, s := range stops {
				if _, err := stmt.Exec(s.StopID, s.StopName, s.StopDesc, s.StopLat, s.StopLon, s.LocationType, s.StopCode); err != nil {
					return fmt.Errorf("error inserting stop: %w", err)
				}
			}
			return nil
		})
}
