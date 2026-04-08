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
	return HandleCached(cacheID, cacheTimes.StopInfoCacheShelfLife,
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

	stops, errStops := GetStops(tranzyClient, cacheTimes.StopCacheShelfLife, StopFilter{StopID: filter.StopID})
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

	apiStopTimes, errApiStopTimes := getAPIStopTimes(tranzyClient, cacheTimes.APIStopTimeCacheShelfLife, APIStopTimeFilter{})
	if errApiStopTimes != nil || len(apiStopTimes) == 0 {
		return nil, errApiStopTimes
	}

	var outgoingTripIds []string
	var incomingTripIds []string
	var routeIds []string
	for _, ast := range apiStopTimes {
		if ast.StopID == stopInfo.StopID {
			if strings.HasSuffix(ast.TripID, OUTGOING_SUFFIX) {
				outgoingTripIds = append(outgoingTripIds, ast.TripID)
			} else if strings.HasSuffix(ast.TripID, INCOMING_SUFFIX) {
				incomingTripIds = append(incomingTripIds, ast.TripID)
			}
			routeIds = append(routeIds, strings.TrimSuffix(strings.TrimSuffix(ast.TripID, OUTGOING_SUFFIX), INCOMING_SUFFIX))
		}
	}

	// This might not be necessary, but I am not sure if there are no stations with duplicates like 1_0 3 times...
	slices.Sort(outgoingTripIds)
	outgoingTripIds = slices.Compact(outgoingTripIds)
	stopInfo.OutgoingTripIds = outgoingTripIds

	slices.Sort(incomingTripIds)
	incomingTripIds = slices.Compact(incomingTripIds)
	stopInfo.IncomingTripIds = incomingTripIds

	slices.Sort(routeIds)
	routeIds = slices.Compact(routeIds)
	stopInfo.ShapesInfo = []models.ShapeInfo{}

	for _, shape := range routeIds {
		routeID, err := strconv.Atoi(shape)
		if err != nil {
			continue
		}

		routes, errorRoutes := GetRoutes(tranzyClient, cacheTimes.RouteCacheShelfLife, RouteFilter{RouteID: &routeID})
		if errorRoutes != nil {
			continue
		}
		route := routes[0]
		routeShortName := route.RouteShortName

		stopTimes, errStopTimes := GetStopTimes(tranzyClient, cacheTimes, StopTimeFilter{RouteShortName: &routeShortName})
		if errStopTimes != nil {
			continue
		}

		timetable, errTimetable := GetTimetable(ctpCjClient, cacheTimes.TimetableCacheShelfLife, routeShortName)
		if errTimetable != nil {
			continue
		}

		stopInfo.ShapesInfo = append(stopInfo.ShapesInfo, models.ShapeInfo{RouteShortName: routeShortName, RouteId: routeID, RouteType: route.RouteType, RouteColor: route.RouteColor, StopTimes: stopTimes, Timetable: *timetable})
	}
	return &stopInfo, nil
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

	stops, errStops := GetStops(tranzyClient, cacheTimes.StopCacheShelfLife, StopFilter{StopID: &stopInfo.StopID})
	if errStops != nil || len(stops) == 0 {
		return nil, errStops
	}
	stop := stops[0]

	stopInfo.StopName = stop.StopName
	stopInfo.StopDesc = stop.StopDesc
	stopInfo.StopLat = stop.StopLat
	stopInfo.StopLon = stop.StopLon
	stopInfo.LocationType = stop.LocationType
	stopInfo.StopCode = stop.StopCode
	stopInfo.ShapesInfo = []models.ShapeInfo{}

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

	for _, shapeShortName := range shapesShortNames {
		routes, errorRoutes := GetRoutes(tranzyClient, cacheTimes.RouteCacheShelfLife, RouteFilter{RouteShortName: &shapeShortName})
		if errorRoutes != nil {
			continue
		}
		route := routes[0]

		stopTimes, errStopTimes := GetStopTimes(tranzyClient, cacheTimes, StopTimeFilter{RouteShortName: &shapeShortName})
		if errStopTimes != nil {
			continue
		}

		timetable, errTimetable := GetTimetable(ctpCjClient, cacheTimes.TimetableCacheShelfLife, shapeShortName)
		if errTimetable != nil {
			continue
		}

		stopInfo.ShapesInfo = append(stopInfo.ShapesInfo, models.ShapeInfo{RouteShortName: shapeShortName, RouteId: route.RouteID, RouteType: route.RouteType, RouteColor: route.RouteColor, StopTimes: stopTimes, Timetable: *timetable})
	}

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
