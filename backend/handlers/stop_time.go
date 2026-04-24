package handlers

import (
	"conexiuni-cluj/models"
	"conexiuni-cluj/services/tranzy"
	"database/sql"
	"fmt"
	"time"
)

const (
	StopTimesCacheId    = "STOP_TIMES"
	APIStopTimesCacheId = "API_STOP_TIMES"
)

type StopTimeFilter struct {
	RouteShortName *string
}

func GetStopTimes(tranzyClient *tranzy.Client, cacheTimes models.CacheTimes, filter StopTimeFilter) ([]models.StopTime, error) {
	if filter.RouteShortName == nil {
		return nil, fmt.Errorf("route_short_name is required")
	}
	cacheID := fmt.Sprintf("%s_%s", StopTimesCacheId, *filter.RouteShortName)
	return HandleCached(cacheID, cacheTimes.StopTimeCacheShelfLife,
		func() ([]models.StopTime, error) { return getStopTimesFromDB(filter) },
		func() ([]models.StopTime, error) { return requestStopTimes(tranzyClient, filter, cacheTimes) },
		storeStopTimesInDB,
		CacheOpts[[]models.StopTime]{},
	)
}

func requestStopTimes(tranzyClient *tranzy.Client, filter StopTimeFilter, cacheTimes models.CacheTimes) ([]models.StopTime, error) {
	routes, err := GetRoutes(tranzyClient, cacheTimes.RouteCacheShelfLife, RouteFilter{RouteShortName: filter.RouteShortName})
	if err != nil || len(routes) == 0 {
		return nil, err
	}

	trips, err := GetTrips(tranzyClient, cacheTimes.TripCacheShelfLife, TripFilter{RouteID: &routes[0].RouteID})
	if err != nil || len(trips) == 0 {
		return nil, err
	}

	apiStopTimes, err := getAPIStopTimes(tranzyClient, cacheTimes.APIStopTimeCacheShelfLife, APIStopTimeFilter{})
	if err != nil {
		return nil, err
	}

	// Index: trip_id → []RequestStopTime
	astByTrip := make(map[string][]models.RequestStopTime, len(trips))
	for _, ast := range apiStopTimes {
		astByTrip[ast.TripID] = append(astByTrip[ast.TripID], ast)
	}

	// Index: trip_id → shape_id, collect unique shape IDs
	shapeByTrip := make(map[string]string, len(trips))
	shapeIDsSeen := make(map[string]struct{})
	for _, t := range trips {
		shapeByTrip[t.TripID] = t.ShapeID
		if t.ShapeID != "" {
			shapeIDsSeen[t.ShapeID] = struct{}{}
		}
	}

	// Batch-fetch all shapes needed for this route
	shapesByID := make(map[string][]models.Shape)
	if len(shapeIDsSeen) > 0 {
		shapeIDs := make([]string, 0, len(shapeIDsSeen))
		for id := range shapeIDsSeen {
			shapeIDs = append(shapeIDs, id)
		}
		if allShapes, err := GetShapes(tranzyClient, cacheTimes.ShapeCacheShelfLife, ShapeFilter{ShapeIDs: shapeIDs}); err == nil {
			for _, s := range allShapes {
				shapesByID[s.ShapeID] = append(shapesByID[s.ShapeID], s)
			}
		}
	}

	// Fetch all stops once and index by ID — avoids N+1 queries
	allStops, err := GetStops(tranzyClient, cacheTimes.StopCacheShelfLife, StopFilter{})
	if err != nil {
		return nil, err
	}
	stopByID := make(map[int]models.Stop, len(allStops))
	for _, s := range allStops {
		stopByID[s.StopID] = s
	}

	var out []models.StopTime
	for _, t := range trips {
		gr := astByTrip[t.TripID]
		if len(gr) == 0 {
			continue
		}
		shapes := shapesByID[shapeByTrip[t.TripID]]
		var previousStop *models.Stop
		for _, st := range gr {
			currentStop := stopByID[st.StopID]
			offsetArrivalTime := 0.0
			if previousStop != nil && st.StopSequence != 0 && len(shapes) > 0 {
				offsetArrivalTime = calculateStopOffset(*previousStop, currentStop, shapes)
			}
			out = append(out, models.StopTime{
				TripID:            st.TripID,
				StopID:            st.StopID,
				OffsetArrivalTime: offsetArrivalTime,
				StopSequence:      st.StopSequence,
				StopHeadsign:      currentStop.StopName,
				RouteShortName:    *filter.RouteShortName,
				StopLat:           currentStop.StopLat,
				StopLon:           currentStop.StopLon,
			})
			previousStop = &currentStop
		}
	}
	return out, nil
}

func getStopTimesFromDB(filter StopTimeFilter) ([]models.StopTime, error) {
	var conditions []string
	var args []any
	if filter.RouteShortName != nil {
		conditions = append(conditions, "route_short_name = ?")
		args = append(args, *filter.RouteShortName)
	}
	return queryRows(`SELECT * FROM stop_times`+whereClause(conditions), args,
		func(rows *sql.Rows) (models.StopTime, error) {
			var st models.StopTime
			err := rows.Scan(&st.TripID, &st.StopID, &st.OffsetArrivalTime, &st.StopSequence, &st.StopHeadsign, &st.RouteShortName, &st.StopLat, &st.StopLon)
			return st, err
		})
}

func storeStopTimesInDB(stopTimes []models.StopTime) error {
	return batchExec(`
		INSERT OR REPLACE INTO stop_times
		(trip_id, stop_id, offset_arrival_time, stop_sequence, stop_headsign, route_short_name, stop_lat, stop_lon)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		func(stmt *sql.Stmt) error {
			for _, st := range stopTimes {
				if _, err := stmt.Exec(st.TripID, st.StopID, st.OffsetArrivalTime, st.StopSequence, st.StopHeadsign, st.RouteShortName, st.StopLat, st.StopLon); err != nil {
					return fmt.Errorf("error inserting stop time: %w", err)
				}
			}
			return nil
		})
}

type APIStopTimeFilter struct {
	TripID *string
}

func getAPIStopTimes(tranzyClient *tranzy.Client, cacheShelfLife time.Duration, filter APIStopTimeFilter) ([]models.RequestStopTime, error) {
	opts := CacheOpts[[]models.RequestStopTime]{}
	if filter.TripID != nil {
		f := filter
		opts.PostProcess = func(ts []models.RequestStopTime) []models.RequestStopTime {
			out := make([]models.RequestStopTime, 0)
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
		func() ([]models.RequestStopTime, error) {
			return tranzyFetch[[]models.RequestStopTime](tranzyClient, "/stop_times")
		},
		storeAPIStopTimesInDB,
		opts,
	)
}

func getAPIStopTimesFromDB(filter APIStopTimeFilter) ([]models.RequestStopTime, error) {
	var conditions []string
	var args []any
	if filter.TripID != nil {
		conditions = append(conditions, "trip_id = ?")
		args = append(args, *filter.TripID)
	}
	return queryRows(`SELECT * FROM api_stop_times`+whereClause(conditions), args,
		func(rows *sql.Rows) (models.RequestStopTime, error) {
			var st models.RequestStopTime
			err := rows.Scan(&st.TripID, &st.StopID, &st.StopSequence)
			return st, err
		})
}

func storeAPIStopTimesInDB(stopTimes []models.RequestStopTime) error {
	return batchExec(`
		INSERT OR REPLACE INTO api_stop_times (trip_id, stop_id, stop_sequence)
		VALUES (?, ?, ?)`,
		func(stmt *sql.Stmt) error {
			for _, st := range stopTimes {
				if _, err := stmt.Exec(st.TripID, st.StopID, st.StopSequence); err != nil {
					return fmt.Errorf("error inserting api stop time: %w", err)
				}
			}
			return nil
		})
}
