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
	StopsCacheId = "STOPS"
)

type StopFilter struct {
	StopID *int
}

func GetStops(tranzyClient *tranzy.Client, cacheShelfLife time.Duration, filter StopFilter) ([]models.Stop, error) {
	opts := CacheOpts[[]models.Stop]{}

	if filter.StopID != nil {
		f := filter
		opts.PostProcess = func(ss []models.Stop) []models.Stop {
			var out []models.Stop
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
		func() ([]models.Stop, error) { return requestStops(tranzyClient) },
		storeStopsInDB,
		opts,
	)
}

func requestStops(tranzyClient *tranzy.Client) ([]models.Stop, error) {
	data, err := tranzyClient.DoRequest("/stops", nil)
	if err != nil {
		return nil, err
	}

	var stops []models.Stop
	if err := json.Unmarshal(data, &stops); err != nil {
		return nil, fmt.Errorf("failed to unmarshal stops: %w", err)
	}

	return stops, nil
}

func getStopsFromDB(filter StopFilter) ([]models.Stop, error) {
	query := `SELECT * FROM stops`
	var args []any
	var conditions []string

	if filter.StopID != nil {
		conditions = append(conditions, "stop_id = ?")
		args = append(args, *filter.StopID)
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("error querying stops: %w", err)
	}

	defer func(rows *sql.Rows) {
		_ = rows.Close()
	}(rows)

	var stops []models.Stop
	for rows.Next() {
		var stop models.Stop
		if err := rows.Scan(
			&stop.StopID,
			&stop.StopName,
			&stop.StopDesc,
			&stop.StopLat,
			&stop.StopLon,
			&stop.LocationType,
			&stop.StopCode,
		); err != nil {
			return nil, fmt.Errorf("error scanning stop: %w", err)
		}
		stops = append(stops, stop)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error reading stops: %w", err)
	}

	return stops, nil
}

func storeStopsInDB(stops []models.Stop) error {
	stmt, err := database.DB.Prepare(`
		INSERT OR REPLACE INTO stops
		(stop_id, stop_name, stop_desc, stop_lat, stop_lon, location_type, stop_code)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("error preparing statement: %w", err)
	}

	defer func(stmt *sql.Stmt) {
		_ = stmt.Close()
	}(stmt)

	for _, stop := range stops {
		if _, err := stmt.Exec(
			stop.StopID,
			stop.StopName,
			stop.StopDesc,
			stop.StopLat,
			stop.StopLon,
			stop.LocationType,
			stop.StopCode,
		); err != nil {
			return fmt.Errorf("error inserting stop: %w", err)
		}
	}

	return nil
}
