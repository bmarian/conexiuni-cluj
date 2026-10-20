package transit

import (
	"conexiuni-cluj/database"
	"conexiuni-cluj/models"
	"database/sql"
	"fmt"
	"sort"
	"strings"
)

type StopTimeFilter struct {
	RouteShortName *string
}

// A routeID of 0 means the route is unknown and the stop times carry no learned profiles.
func (d *Data) BaseStopTimes(filter StopTimeFilter) ([]models.StopTime, int, error) {
	if filter.RouteShortName == nil {
		return nil, 0, fmt.Errorf("route_short_name is required")
	}
	stopTimes, err := d.StopTimes(filter)
	if err != nil {
		return nil, 0, err
	}
	routes, err := d.Routes(RouteFilter{RouteShortName: filter.RouteShortName})
	if err != nil || len(routes) == 0 {
		return stopTimes, 0, nil
	}
	return stopTimes, routes[0].RouteID, nil
}

func scanStopTime(rows *sql.Rows) (models.StopTime, error) {
	var st models.StopTime
	err := rows.Scan(&st.TripID, &st.StopID, &st.OffsetArrivalTime, &st.StopSequence, &st.StopHeadsign, &st.RouteShortName, &st.StopLat, &st.StopLon)
	return st, err
}

func (d *Data) StopTimesForTrips(tripIDs []string) ([]models.StopTime, error) {
	if len(tripIDs) == 0 {
		return nil, nil
	}
	args := make([]any, len(tripIDs))
	for i, id := range tripIDs {
		args[i] = id
	}
	return database.QueryRows(d.db, `
		SELECT trip_id, stop_id, offset_arrival_time, stop_sequence, stop_headsign, route_short_name, stop_lat, stop_lon
		FROM stop_times
		WHERE trip_id IN (`+strings.TrimSuffix(strings.Repeat("?,", len(tripIDs)), ",")+`)
		ORDER BY trip_id, stop_sequence`,
		args, scanStopTime)
}

func (d *Data) StopTimes(filter StopTimeFilter) ([]models.StopTime, error) {
	if filter.RouteShortName == nil {
		return database.QueryRows(d.db, `SELECT * FROM stop_times ORDER BY trip_id, stop_sequence`, nil, scanStopTime)
	}
	// Through routes/trips: stop_times.route_short_name goes stale when Tranzy renumbers routes.
	return database.QueryRows(d.db, `
		SELECT st.trip_id, st.stop_id, st.offset_arrival_time, st.stop_sequence,
		       st.stop_headsign, st.route_short_name, st.stop_lat, st.stop_lon
		FROM stop_times st
		JOIN trips t  ON st.trip_id = t.trip_id
		JOIN routes r ON t.route_id = r.route_id
		WHERE r.route_short_name = ?
		ORDER BY st.trip_id, st.stop_sequence`,
		[]any{*filter.RouteShortName}, scanStopTime)
}

// SortedBySuffix is the stop times of the pattern ending in suffix, in order.
func SortedBySuffix(rows []models.StopTime, suffix string) []models.StopTime {
	var out []models.StopTime
	for _, st := range rows {
		if strings.HasSuffix(st.TripID, suffix) {
			out = append(out, st)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StopSequence < out[j].StopSequence })
	return out
}

// TripStopTimes are the stop times of one pattern, in order.
func TripStopTimes(stopTimes []models.StopTime, tripID string) []models.StopTime {
	var trip []models.StopTime
	for _, st := range stopTimes {
		if st.TripID == tripID {
			trip = append(trip, st)
		}
	}
	sort.Slice(trip, func(i, j int) bool { return trip[i].StopSequence < trip[j].StopSequence })
	return trip
}
