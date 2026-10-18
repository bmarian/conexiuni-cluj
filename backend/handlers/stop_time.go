package handlers

import (
	"conexiuni-cluj/models"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"
)

type StopTimeFilter struct {
	RouteShortName *string
}

func (city *City) GetStopTimes(filter StopTimeFilter) ([]models.StopTime, error) {
	return city.GetStopTimesAt(filter, time.Now().In(city.Location))
}

func (city *City) GetStopTimesAt(filter StopTimeFilter, refTime time.Time) ([]models.StopTime, error) {
	stopTimes, routeID, err := city.getBaseStopTimes(filter)
	if err != nil || routeID == 0 {
		return stopTimes, err
	}
	return city.applySegmentProfilesToStopTimes(stopTimes, routeID, refTime), nil
}

func (city *City) dayOfType(from time.Time, dayType string) time.Time {
	for i := 0; i < 7; i++ {
		if d := from.AddDate(0, 0, i); city.scheduleDay(d) == dayType {
			return d
		}
	}
	return from
}

// A routeID of 0 means the route is unknown and the stop times carry no learned profiles.
func (city *City) getBaseStopTimes(filter StopTimeFilter) ([]models.StopTime, int, error) {
	if filter.RouteShortName == nil {
		return nil, 0, fmt.Errorf("route_short_name is required")
	}
	stopTimes, err := city.getStopTimesFromDB(filter)
	if err != nil {
		return nil, 0, err
	}
	routes, err := city.GetRoutes(RouteFilter{RouteShortName: filter.RouteShortName})
	if err != nil || len(routes) == 0 {
		return stopTimes, 0, nil
	}
	return stopTimes, routes[0].RouteID, nil
}

func (city *City) applySegmentProfilesToStopTimes(stopTimes []models.StopTime, routeID int, refTime time.Time) []models.StopTime {
	if len(stopTimes) == 0 {
		return stopTimes
	}

	out := make([]models.StopTime, len(stopTimes))
	copy(out, stopTimes)
	dayType, bucket := city.profileDay(refTime), segmentBucketStartMin(refTime)

	type directionProfile struct {
		segments map[stopPair]segmentProfileEstimate
		delay    float64
		hasDelay bool
	}
	byDirection := make(map[int]directionProfile)
	groupByTrip := make(map[string][]int)
	for i, st := range out {
		groupByTrip[st.TripID] = append(groupByTrip[st.TripID], i)
	}

	for tripID, indexes := range groupByTrip {
		directionID, ok := directionIDFromTripID(tripID)
		if !ok {
			continue
		}
		profile, exists := byDirection[directionID]
		if !exists {
			profile.segments = city.snapshotSegmentProfiles(routeID, directionID, dayType, bucket)
			profile.delay, profile.hasDelay = city.snapshotScheduleDelay(routeID, directionID, dayType, bucket)
			byDirection[directionID] = profile
		}

		sort.Slice(indexes, func(i, j int) bool {
			return out[indexes[i]].StopSequence < out[indexes[j]].StopSequence
		})
		applySegmentProfilesToTrip(out, indexes, profile.segments)
		// Every timetable consumer sums from the first stop; live ETAs never read it.
		if profile.hasDelay && len(indexes) > 0 {
			out[indexes[0]].OffsetArrivalTime = profile.delay
		}
	}
	return out
}

// ordered indexes one trip's rows in stop_sequence order.
func applySegmentProfilesToTrip(stopTimes []models.StopTime, ordered []int, profiles map[stopPair]segmentProfileEstimate) {
	for pos := 1; pos < len(ordered); pos++ {
		prev := stopTimes[ordered[pos-1]]
		curr := &stopTimes[ordered[pos]]
		if estimate, ok := profiles[stopPair{FromStopID: prev.StopID, ToStopID: curr.StopID}]; ok && estimate.DurationSec > 0 {
			curr.OffsetArrivalTime = estimate.DurationSec
			curr.OffsetConfidence = estimate.Confidence
		}
	}
}

func scanStopTime(rows *sql.Rows) (models.StopTime, error) {
	var st models.StopTime
	err := rows.Scan(&st.TripID, &st.StopID, &st.OffsetArrivalTime, &st.StopSequence, &st.StopHeadsign, &st.RouteShortName, &st.StopLat, &st.StopLon)
	return st, err
}

func (city *City) getStopTimesForTrips(tripIDs []string) ([]models.StopTime, error) {
	if len(tripIDs) == 0 {
		return nil, nil
	}
	args := make([]any, len(tripIDs))
	for i, id := range tripIDs {
		args[i] = id
	}
	return queryRows(city.DB, `
		SELECT trip_id, stop_id, offset_arrival_time, stop_sequence, stop_headsign, route_short_name, stop_lat, stop_lon
		FROM stop_times
		WHERE trip_id IN (`+strings.TrimSuffix(strings.Repeat("?,", len(tripIDs)), ",")+`)
		ORDER BY trip_id, stop_sequence`,
		args, scanStopTime)
}

func (city *City) getStopTimesFromDB(filter StopTimeFilter) ([]models.StopTime, error) {
	if filter.RouteShortName == nil {
		return queryRows(city.DB, `SELECT * FROM stop_times ORDER BY trip_id, stop_sequence`, nil, scanStopTime)
	}
	// Through routes/trips: stop_times.route_short_name goes stale when Tranzy renumbers routes.
	return queryRows(city.DB, `
		SELECT st.trip_id, st.stop_id, st.offset_arrival_time, st.stop_sequence,
		       st.stop_headsign, st.route_short_name, st.stop_lat, st.stop_lon
		FROM stop_times st
		JOIN trips t  ON st.trip_id = t.trip_id
		JOIN routes r ON t.route_id = r.route_id
		WHERE r.route_short_name = ?
		ORDER BY st.trip_id, st.stop_sequence`,
		[]any{*filter.RouteShortName}, scanStopTime)
}

// Rows are keyed by stop_sequence, so a trip that lost stops would keep its old tail without the delete.
func (city *City) storeStopTimesInDB(stopTimes []models.StopTime) error {
	clear := func(tx *sql.Tx) error {
		for _, id := range tripIDsOf(stopTimes) {
			if _, err := tx.Exec(`DELETE FROM stop_times WHERE trip_id = ?`, id); err != nil {
				return fmt.Errorf("error clearing stop times: %w", err)
			}
		}
		return nil
	}
	err := batchReplace(city.DB, clear, `
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
	if err != nil {
		return err
	}
	city.archiveStopTimes(stopTimes)
	return nil
}

// Deletes stop times of trips that left the snapshot.
func (city *City) ScrubStopTimes() (int64, error) {
	res, err := city.DB.Exec(`DELETE FROM stop_times WHERE trip_id NOT IN (SELECT trip_id FROM trips)`)
	if err != nil {
		return 0, fmt.Errorf("scrub orphans: %w", err)
	}
	return res.RowsAffected()
}

func filterAndSortStopTimesBySuffix(rows []models.StopTime, suffix string) []models.StopTime {
	var out []models.StopTime
	for _, st := range rows {
		if strings.HasSuffix(st.TripID, suffix) {
			out = append(out, st)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StopSequence < out[j].StopSequence })
	return out
}
