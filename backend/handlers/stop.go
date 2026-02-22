package handlers

import (
	"conexiuni-cluj/database"
	"conexiuni-cluj/models"
	"conexiuni-cluj/services/tranzy"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

const (
	StopsCacheId = "STOPS"
)

func GetStops(tranzyClient *tranzy.Client, cacheShelfLife time.Duration) ([]models.Stop, error) {
	return HandleCached(StopsCacheId, cacheShelfLife,
		getStopsFromDB,
		func() ([]models.Stop, error) { return requestStops(tranzyClient) },
		storeStopsInDB,
		CacheOpts[[]models.Stop]{Optimize: true},
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

func getStopsFromDB() ([]models.Stop, error) {
	rows, err := database.DB.Query(`SELECT * FROM stops`)
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
