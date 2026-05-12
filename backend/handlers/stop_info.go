package handlers

import (
	"conexiuni-cluj/database"
	"conexiuni-cluj/models"
	ctpcj "conexiuni-cluj/services/ctp-cj"
	"conexiuni-cluj/services/tranzy"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"slices"
)

const (
	StopInfoCacheId = "STOP_INFO"
	OUTGOING_SUFFIX = "_0"
	INCOMING_SUFFIX = "_1"
)

func GetStopInfo(tranzyClient *tranzy.Client, ctpCjClient *ctpcj.Client, cacheTimes models.CacheTimes, filter StopFilter) (*models.StopInfo, error) {
	opts := CacheOpts[*models.StopInfo]{
		Optimize: true,
	}

	cacheID := fmt.Sprintf("%s_%d", StopInfoCacheId, *filter.StopID)
	return HandleCached(cacheID, cacheTimes.TranzyCacheShelfLife,
		func() (*models.StopInfo, error) {
			return getStopInfoFromDB(tranzyClient, ctpCjClient, cacheTimes, filter)
		},
		func() (*models.StopInfo, error) {
			return requestStopInfo(tranzyClient, ctpCjClient, cacheTimes, filter)
		},
		storeStopInfoInDB,
		opts)
}

func requestStopInfo(tranzyClient *tranzy.Client, ctpCjClient *ctpcj.Client, cacheTimes models.CacheTimes, filter StopFilter) (*models.StopInfo, error) {
	var stopInfo models.StopInfo

	stops, errStops := GetStops(tranzyClient, cacheTimes.TranzyCacheShelfLife, StopFilter{StopID: filter.StopID})
	if errStops != nil || len(stops) == 0 {
		return nil, errStops
	}
	stop := stops[0]

	stopInfo.StopID = stop.StopID
	stopInfo.StopName = stop.StopName
	stopInfo.StopDesc = stop.StopDesc
	stopInfo.StopLat = stop.StopLat
	stopInfo.StopLon = stop.StopLon
	stopInfo.LocationType = stop.LocationType
	stopInfo.StopCode = stop.StopCode

	apiStopTimes, errApiStopTimes := getAPIStopTimes(tranzyClient, cacheTimes.TranzyCacheShelfLife, APIStopTimeFilter{})
	if errApiStopTimes != nil || len(apiStopTimes) == 0 {
		return nil, errApiStopTimes
	}

	var outgoingTripIds []string
	var incomingTripIds []string
	routeIDSet := make(map[int]struct{})
	for _, ast := range apiStopTimes {
		if ast.StopID != stopInfo.StopID {
			continue
		}
		normalizedTripID := NormalizeTripID(ast.TripID)
		if strings.HasSuffix(normalizedTripID, OUTGOING_SUFFIX) {
			outgoingTripIds = append(outgoingTripIds, normalizedTripID)
		} else if strings.HasSuffix(normalizedTripID, INCOMING_SUFFIX) {
			incomingTripIds = append(incomingTripIds, normalizedTripID)
		}
		idStr := strings.TrimSuffix(strings.TrimSuffix(normalizedTripID, OUTGOING_SUFFIX), INCOMING_SUFFIX)
		if id, err := strconv.Atoi(idStr); err == nil {
			routeIDSet[id] = struct{}{}
		}
	}

	slices.Sort(outgoingTripIds)
	stopInfo.OutgoingTripIds = slices.Compact(outgoingTripIds)
	slices.Sort(incomingTripIds)
	stopInfo.IncomingTripIds = slices.Compact(incomingTripIds)

	allRoutes, errAll := GetRoutes(tranzyClient, cacheTimes.TranzyCacheShelfLife, RouteFilter{})
	if errAll != nil {
		return nil, errAll
	}
	routeByID := make(map[int]models.Route, len(allRoutes))
	for _, r := range allRoutes {
		routeByID[r.RouteID] = r
	}

	routeIDs := make([]int, 0, len(routeIDSet))
	for id := range routeIDSet {
		routeIDs = append(routeIDs, id)
	}
	slices.Sort(routeIDs)

	stopInfo.ShapesInfo = buildShapesInfoParallel(tranzyClient, ctpCjClient, cacheTimes, routeIDs, routeByID)
	return &stopInfo, nil
}

func buildShapesInfoParallel(
	tranzyClient *tranzy.Client,
	ctpCjClient *ctpcj.Client,
	cacheTimes models.CacheTimes,
	routeIDs []int,
	routeByID map[int]models.Route,
) []models.ShapeInfo {
	type slot struct {
		ok   bool
		info models.ShapeInfo
	}
	slots := make([]slot, len(routeIDs))

	var wg sync.WaitGroup
	for i, rid := range routeIDs {
		route, exists := routeByID[rid]
		if !exists {
			continue
		}
		wg.Add(1)
		go func(i int, route models.Route) {
			defer wg.Done()
			rsn := route.RouteShortName

			var (
				stopTimes []models.StopTime
				timetable *models.Timetable
				stErr     error
				ttErr     error
				inner     sync.WaitGroup
			)
			inner.Add(2)
			go func() {
				defer inner.Done()
				stopTimes, stErr = GetStopTimes(tranzyClient, cacheTimes, StopTimeFilter{RouteShortName: &rsn})
			}()
			go func() {
				defer inner.Done()
				timetable, ttErr = GetTimetable(ctpCjClient, tranzyClient, cacheTimes, rsn)
			}()
			inner.Wait()

			if stErr != nil || ttErr != nil || timetable == nil {
				return
			}
			// Ignore metadata-only timetables with no departures.
			if len(timetable.Weekdays.Entries) == 0 &&
				len(timetable.Saturday.Entries) == 0 &&
				len(timetable.Sunday.Entries) == 0 {
				return
			}
			slots[i] = slot{
				ok: true,
				info: models.ShapeInfo{
					RouteShortName: rsn,
					RouteLongName:  route.RouteLongName,
					RouteId:        route.RouteID,
					RouteType:      route.RouteType,
					RouteColor:     route.RouteColor,
					StopTimes:      stopTimes,
					Timetable:      *timetable,
				},
			}
		}(i, route)
	}
	wg.Wait()

	out := make([]models.ShapeInfo, 0, len(slots))
	for _, s := range slots {
		if s.ok {
			out = append(out, s.info)
		}
	}
	return out
}

func getStopInfoFromDB(tranzyClient *tranzy.Client, ctpCjClient *ctpcj.Client, cacheTimes models.CacheTimes, filter StopFilter) (*models.StopInfo, error) {
	var stopInfo models.StopInfo

	var tripIdsJson string
	var tripIds []string

	var shapesShortNamesJson string
	var shapesShortNames []string

	query := `SELECT * FROM stop_info WHERE stop_id = ?`
	args := []any{*filter.StopID}

	row := database.DB.QueryRow(query, args...)
	err := row.Scan(&stopInfo.StopID, &tripIdsJson, &shapesShortNamesJson)
	if err != nil {
		return nil, fmt.Errorf("error querying stop info: %w", err)
	}

	if err := json.Unmarshal([]byte(tripIdsJson), &tripIds); err != nil {
		return nil, fmt.Errorf("error unmarshalling trip IDs: %w", err)
	}

	if err := json.Unmarshal([]byte(shapesShortNamesJson), &shapesShortNames); err != nil {
		return nil, fmt.Errorf("error unmarshalling shapes short names: %w", err)
	}

	var (
		stop         models.Stop
		stopErr      error
		allRoutes    []models.Route
		allRoutesErr error
		header       sync.WaitGroup
	)
	header.Add(2)
	go func() {
		defer header.Done()
		stops, e := GetStops(tranzyClient, cacheTimes.TranzyCacheShelfLife, StopFilter{StopID: &stopInfo.StopID})
		if e != nil {
			stopErr = e
			return
		}
		if len(stops) == 0 {
			stopErr = fmt.Errorf("stop %d not found", stopInfo.StopID)
			return
		}
		stop = stops[0]
	}()
	go func() {
		defer header.Done()
		allRoutes, allRoutesErr = GetRoutes(tranzyClient, cacheTimes.TranzyCacheShelfLife, RouteFilter{})
	}()
	header.Wait()

	if stopErr != nil {
		return nil, stopErr
	}
	if allRoutesErr != nil {
		return nil, allRoutesErr
	}

	stopInfo.StopName = stop.StopName
	stopInfo.StopDesc = stop.StopDesc
	stopInfo.StopLat = stop.StopLat
	stopInfo.StopLon = stop.StopLon
	stopInfo.LocationType = stop.LocationType
	stopInfo.StopCode = stop.StopCode

	var outgoingTripIds []string
	var incomingTripIds []string
	for _, id := range tripIds {
		if strings.HasSuffix(id, OUTGOING_SUFFIX) {
			outgoingTripIds = append(outgoingTripIds, id)
		} else if strings.HasSuffix(id, INCOMING_SUFFIX) {
			incomingTripIds = append(incomingTripIds, id)
		}
	}
	stopInfo.OutgoingTripIds = outgoingTripIds
	stopInfo.IncomingTripIds = incomingTripIds

	routeByID := make(map[int]models.Route, len(allRoutes))
	routeByShortName := make(map[string]models.Route, len(allRoutes))
	for _, r := range allRoutes {
		routeByID[r.RouteID] = r
		routeByShortName[r.RouteShortName] = r
	}

	routeIDs := make([]int, 0, len(shapesShortNames))
	for _, sn := range shapesShortNames {
		if r, ok := routeByShortName[sn]; ok {
			routeIDs = append(routeIDs, r.RouteID)
		}
	}

	stopInfo.ShapesInfo = buildShapesInfoParallel(tranzyClient, ctpCjClient, cacheTimes, routeIDs, routeByID)
	return &stopInfo, nil
}

func storeStopInfoInDB(stopInfo *models.StopInfo) error {
	stmt, err := database.DB.Prepare(`
		INSERT OR REPLACE INTO stop_info (stop_id, trip_ids, shapes_short_name)
		VALUES (?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("error preparing statement: %w", err)
	}
	defer func(stmt *sql.Stmt) {
		_ = stmt.Close()
	}(stmt)

	var tripIds []string
	for _, id := range stopInfo.IncomingTripIds {
		tripIds = append(tripIds, id)
	}
	for _, id := range stopInfo.OutgoingTripIds {
		tripIds = append(tripIds, id)
	}

	tripIdsJSON, err := json.Marshal(tripIds)
	if err != nil {
		return fmt.Errorf("error marshalling trip ids: %w", err)
	}

	var shapesShortName []string
	for _, shape := range stopInfo.ShapesInfo {
		shapesShortName = append(shapesShortName, shape.RouteShortName)
	}
	shapesShortNameJson, err := json.Marshal(shapesShortName)
	if err != nil {
		return fmt.Errorf("error marshalling shapes short name: %w", err)
	}

	if _, err := stmt.Exec(stopInfo.StopID, tripIdsJSON, shapesShortNameJson); err != nil {
		return fmt.Errorf("error inserting stop info: %w", err)
	}

	return nil
}
