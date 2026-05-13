package handlers

import (
	"conexiuni-cluj/models"
	"conexiuni-cluj/services/tranzy"
	"database/sql"
	"fmt"
	"log"
	"math"
	"sort"
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
	return GetStopTimesAt(tranzyClient, cacheTimes, filter, time.Now().In(tranzyClient.Location()))
}

func GetStopTimesAt(tranzyClient *tranzy.Client, cacheTimes models.CacheTimes, filter StopTimeFilter, refTime time.Time) ([]models.StopTime, error) {
	if filter.RouteShortName == nil {
		return nil, fmt.Errorf("route_short_name is required")
	}
	cacheID := fmt.Sprintf("%s_%s", StopTimesCacheId, *filter.RouteShortName)
	stopTimes, err := HandleCached(cacheID, cacheTimes.TranzyCacheShelfLife,
		func() ([]models.StopTime, error) { return getStopTimesFromDB(filter) },
		func() ([]models.StopTime, error) { return requestStopTimes(tranzyClient, filter, cacheTimes) },
		storeStopTimesInDB,
		CacheOpts[[]models.StopTime]{},
	)
	if err != nil {
		return nil, err
	}

	routes, err := GetRoutes(tranzyClient, cacheTimes.TranzyCacheShelfLife, RouteFilter{RouteShortName: filter.RouteShortName})
	if err != nil || len(routes) == 0 {
		return stopTimes, nil
	}
	return applySegmentProfilesToStopTimes(stopTimes, routes[0].RouteID, refTime), nil
}

func requestStopTimes(tranzyClient *tranzy.Client, filter StopTimeFilter, cacheTimes models.CacheTimes) ([]models.StopTime, error) {
	routes, err := GetRoutes(tranzyClient, cacheTimes.TranzyCacheShelfLife, RouteFilter{RouteShortName: filter.RouteShortName})
	if err != nil || len(routes) == 0 {
		return nil, err
	}

	trips, err := GetTrips(tranzyClient, cacheTimes.TranzyCacheShelfLife, TripFilter{RouteID: &routes[0].RouteID})
	if err != nil || len(trips) == 0 {
		return nil, err
	}

	apiStopTimes, err := getAPIStopTimes(tranzyClient, cacheTimes.TranzyCacheShelfLife, APIStopTimeFilter{})
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
		if allShapes, err := GetShapes(tranzyClient, cacheTimes.TranzyCacheShelfLife, ShapeFilter{ShapeIDs: shapeIDs}); err == nil {
			for _, s := range allShapes {
				shapesByID[s.ShapeID] = append(shapesByID[s.ShapeID], s)
			}
		}
	}

	// Fetch all stops at once and index by ID
	allStops, err := GetStops(tranzyClient, cacheTimes.TranzyCacheShelfLife, StopFilter{})
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
		sort.Slice(gr, func(i, j int) bool {
			return gr[i].StopSequence < gr[j].StopSequence
		})
		shapes := shapesByID[shapeByTrip[t.TripID]]
		sort.Slice(shapes, func(i, j int) bool {
			return shapes[i].ShapePtSequence < shapes[j].ShapePtSequence
		})
		var previousStop *models.Stop
		stopIndex := 0
		for _, st := range gr {
			currentStop := stopByID[st.StopID]
			offsetArrivalTime := 0.0
			if previousStop != nil && st.StopSequence != 0 && len(shapes) > 0 {
				offsetArrivalTime = calculateStopOffset(*previousStop, currentStop, shapes)
				if stopIndex == 1 {
					// Generally buses do a funny and are late when leaving the first stop,
					// so I hardcoded this 🤢
					offsetArrivalTime = offsetArrivalTime + 30
				}
				// And also always round up to the nearest minute
				offsetArrivalTime = math.Ceil(offsetArrivalTime)
			}
			stopIndex++
			out = append(out, models.StopTime{
				TripID:            NormalizeTripID(st.TripID),
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

func applySegmentProfilesToStopTimes(stopTimes []models.StopTime, routeID int, refTime time.Time) []models.StopTime {
	if len(stopTimes) == 0 {
		return stopTimes
	}

	out := make([]models.StopTime, len(stopTimes))
	copy(out, stopTimes)

	profilesByDirection := make(map[int]map[stopPair]float64)
	groupByTrip := make(map[string][]int)
	for i, st := range out {
		groupByTrip[st.TripID] = append(groupByTrip[st.TripID], i)
	}

	for tripID, indexes := range groupByTrip {
		directionID, ok := directionIDFromTripID(tripID)
		if !ok {
			continue
		}
		profiles, exists := profilesByDirection[directionID]
		if !exists {
			var err error
			profiles, err = loadSegmentProfileDurations(routeID, directionID, refTime)
			if err != nil {
				log.Printf("stop_times: segment profiles route=%d direction=%d: %v", routeID, directionID, err)
				profiles = map[stopPair]float64{}
			}
			profilesByDirection[directionID] = profiles
		}
		if len(profiles) == 0 {
			continue
		}

		sort.Slice(indexes, func(i, j int) bool {
			return out[indexes[i]].StopSequence < out[indexes[j]].StopSequence
		})
		for pos := 1; pos < len(indexes); pos++ {
			prev := out[indexes[pos-1]]
			currIdx := indexes[pos]
			pair := stopPair{FromStopID: prev.StopID, ToStopID: out[currIdx].StopID}
			if duration, ok := profiles[pair]; ok && duration > 0 {
				out[currIdx].OffsetArrivalTime = math.Ceil(duration)
			}
		}
	}

	return out
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
	if filter.TripID != nil {
		normalized := NormalizeTripID(*filter.TripID)
		filter.TripID = &normalized
	}
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
				st.TripID = NormalizeTripID(st.TripID)
				if _, err := stmt.Exec(st.TripID, st.StopID, st.StopSequence); err != nil {
					return fmt.Errorf("error inserting api stop time: %w", err)
				}
			}
			return nil
		})
}
