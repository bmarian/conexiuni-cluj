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
	StopTimesCacheId    = "STOP_TIMES"
	APIStopTimesCacheId = "API_STOP_TIMES"
)

type StopTimeFilter struct {
	RouteShortName *string
	TripID         *string
}

func GetStopTimes(tranzyClient *tranzy.Client, cacheTimes models.CacheTimes, filter StopTimeFilter) ([]models.StopTime, error) {
	opts := CacheOpts[[]models.StopTime]{}

	if filter.TripID != nil || filter.RouteShortName != nil {
		f := filter
		opts.PostProcess = func(ts []models.StopTime) []models.StopTime {
			var out []models.StopTime
			for _, t := range ts {
				if f.TripID != nil && t.TripID != *f.TripID {
					continue
				}
				if f.RouteShortName != nil && t.RouteShortName != *f.RouteShortName {
					continue
				}
				out = append(out, t)
			}
			return out
		}
	}

	cacheID := fmt.Sprintf("%s_%s", StopTimesCacheId, *filter.RouteShortName)
	return HandleCached(cacheID, cacheTimes.StopTimeCacheShelfLife,
		func() ([]models.StopTime, error) { return getStopTimesFromDB(filter) },
		func() ([]models.StopTime, error) { return requestStopTimes(tranzyClient, filter, cacheTimes) },
		storeStopTimesInDB,
		opts,
	)
}

func requestStopTimes(tranzyClient *tranzy.Client, filter StopTimeFilter, cacheTimes models.CacheTimes) ([]models.StopTime, error) {
	routes, errRoutes := GetRoutes(tranzyClient, cacheTimes.RouteCacheShelfLife, RouteFilter{RouteShortName: filter.RouteShortName})
	if errRoutes != nil || len(routes) == 0 {
		return nil, errRoutes
	}
	route := routes[0]

	trips, errTrips := GetTrips(tranzyClient, cacheTimes.TripCacheShelfLife, TripFilter{RouteID: &route.RouteID})
	if errTrips != nil || len(trips) == 0 {
		return nil, errTrips
	}

	apiStopTimes, errApiStopTimes := getAPIStopTimes(tranzyClient, cacheTimes.APIStopTimeCacheShelfLife, APIStopTimeFilter{})
	if errApiStopTimes != nil {
		return nil, errApiStopTimes
	}

	groupedRaw := make(map[string][]models.RequestStopTime)
	count := 0
	for _, t := range trips {
		tripID := t.TripID
		for _, ast := range apiStopTimes {
			if ast.TripID == tripID {
				groupedRaw[tripID] = append(groupedRaw[tripID], ast)
				count++
			}
		}
	}

	out := make([]models.StopTime, 0, count)
	for tripID, gr := range groupedRaw {
		var shapeID string
		for _, t := range trips {
			if t.TripID == tripID {
				shapeID = t.ShapeID
				break
			}
		}

		var shapes []models.Shape
		if shapeID != "" {
			shapes, _ = GetShapes(tranzyClient, cacheTimes.ShapeCacheShelfLife, ShapeFilter{ShapeID: &shapeID})
		}

		var previousStop *models.Stop
		for _, st := range gr {
			stopHeadsign := ""
			offsetArrivalTime := 0.0
			stopLat := 0.0
			stopLon := 0.0
			var currentStop models.Stop

			stops, errStops := GetStops(tranzyClient, cacheTimes.StopCacheShelfLife, StopFilter{StopID: &st.StopID})
			if errStops == nil && len(stops) != 0 {
				currentStop = stops[0]
				stopHeadsign = currentStop.StopName
				stopLat = currentStop.StopLat
				stopLon = currentStop.StopLon
			}

			if previousStop != nil && st.StopSequence != 0 && len(shapes) > 0 {
				offsetArrivalTime = calculateStopOffset(*previousStop, currentStop, shapes)
			}

			out = append(out, models.StopTime{
				TripID:            st.TripID,
				StopID:            st.StopID,
				OffsetArrivalTime: offsetArrivalTime,
				StopSequence:      st.StopSequence,
				StopHeadsign:      stopHeadsign,
				RouteShortName:    *filter.RouteShortName,
				StopLat:           stopLat,
				StopLon:           stopLon,
			})

			previousStop = &currentStop
		}
	}

	return out, nil
}

func getStopTimesFromDB(filter StopTimeFilter) ([]models.StopTime, error) {
	query := `SELECT * FROM stop_times`
	var args []any
	var conditions []string

	if filter.RouteShortName != nil {
		conditions = append(conditions, "route_short_name = ?")
		args = append(args, *filter.RouteShortName)
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
		return nil, fmt.Errorf("error querying stop times: %w", err)
	}

	defer func(rows *sql.Rows) {
		_ = rows.Close()
	}(rows)

	var stopTimes []models.StopTime
	for rows.Next() {
		var st models.StopTime
		if err := rows.Scan(&st.TripID, &st.StopID, &st.OffsetArrivalTime, &st.StopSequence, &st.StopHeadsign, &st.RouteShortName, &st.StopLat, &st.StopLon); err != nil {
			return nil, fmt.Errorf("error scanning stop time: %w", err)
		}
		stopTimes = append(stopTimes, st)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error reading stop times: %w", err)
	}

	return stopTimes, nil
}

func storeStopTimesInDB(stopTimes []models.StopTime) error {
	stmt, err := database.DB.Prepare(`
		INSERT OR REPLACE INTO stop_times (trip_id, stop_id, offset_arrival_time, stop_sequence, stop_headsign, route_short_name, stop_lat, stop_lon)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("error preparing statement: %w", err)
	}

	defer func(stmt *sql.Stmt) {
		_ = stmt.Close()
	}(stmt)

	for _, st := range stopTimes {
		if _, err := stmt.Exec(st.TripID, st.StopID, st.OffsetArrivalTime, st.StopSequence, st.StopHeadsign, st.RouteShortName, st.StopLat, st.StopLon); err != nil {
			return fmt.Errorf("error inserting stop time: %w", err)
		}
	}

	return nil
}

type APIStopTimeFilter struct {
	TripID *string
}

func getAPIStopTimes(tranzyClient *tranzy.Client, cacheShelfLife time.Duration, filter APIStopTimeFilter) ([]models.RequestStopTime, error) {
	opts := CacheOpts[[]models.RequestStopTime]{}

	if filter.TripID != nil {
		f := filter
		opts.PostProcess = func(ts []models.RequestStopTime) []models.RequestStopTime {
			var out []models.RequestStopTime
			for _, t := range ts {
				if f.TripID != nil && t.TripID != *f.TripID {
					continue
				}
				out = append(out, t)
			}
			return out
		}
	}

	return HandleCached(APIStopTimesCacheId, cacheShelfLife,
		func() ([]models.RequestStopTime, error) { return getAPIStopTimesFromDB(filter) },
		func() ([]models.RequestStopTime, error) { return requestAPIStopTimes(tranzyClient) },
		storeAPIStopTimesInDB,
		opts,
	)
}

func requestAPIStopTimes(tranzyClient *tranzy.Client) ([]models.RequestStopTime, error) {
	data, err := tranzyClient.DoRequest("/stop_times", nil)
	if err != nil {
		return nil, err
	}

	var raw []models.RequestStopTime
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to unmarshal api stop times: %w", err)
	}

	return raw, nil
}

func getAPIStopTimesFromDB(filter APIStopTimeFilter) ([]models.RequestStopTime, error) {
	query := `SELECT * FROM api_stop_times`
	var args []any
	var conditions []string

	if filter.TripID != nil {
		conditions = append(conditions, "trip_id = ?")
		args = append(args, *filter.TripID)
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("error querying api stop times: %w", err)
	}

	defer func(rows *sql.Rows) {
		_ = rows.Close()
	}(rows)

	var stopTimes []models.RequestStopTime
	for rows.Next() {
		var st models.RequestStopTime
		if err := rows.Scan(&st.TripID, &st.StopID, &st.StopSequence); err != nil {
			return nil, fmt.Errorf("error scanning api stop time: %w", err)
		}
		stopTimes = append(stopTimes, st)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error reading api stop times: %w", err)
	}

	return stopTimes, nil
}

func storeAPIStopTimesInDB(stopTimes []models.RequestStopTime) error {
	stmt, err := database.DB.Prepare(`
		INSERT OR REPLACE INTO api_stop_times (trip_id, stop_id, stop_sequence)
		VALUES (?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("error preparing statement: %w", err)
	}

	defer func(stmt *sql.Stmt) {
		_ = stmt.Close()
	}(stmt)

	for _, st := range stopTimes {
		if _, err := stmt.Exec(st.TripID, st.StopID, st.StopSequence); err != nil {
			return fmt.Errorf("error inserting api stop time: %w", err)
		}
	}

	return nil
}
