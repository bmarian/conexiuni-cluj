package handlers

import (
	"conexiuni-cluj/database"
	"conexiuni-cluj/models"
	"testing"
	"time"
)

const layoverTripID = "9990_0"

func installStraightTracker(t *testing.T) *tripSegmentTracker {
	t.Helper()
	shape := make([]models.Shape, 100)
	for i := range shape {
		shape[i] = models.Shape{ShapeID: layoverTripID, ShapePtLat: 46.77 + float64(i)*0.0001, ShapePtLon: 23.6, ShapePtSequence: i}
	}
	stopAt := func(id, idx int) trackedSegmentStop {
		return trackedSegmentStop{StopID: id, ShapeIdx: idx, Lat: shape[idx].ShapePtLat, Lon: shape[idx].ShapePtLon}
	}
	tracker := &tripSegmentTracker{
		TripID: layoverTripID, RouteID: 9990, DirectionID: 0,
		Shape: shape, Cumulative: buildCumulativeShapeDistance(shape),
		Stops: []trackedSegmentStop{stopAt(1, 0), stopAt(2, 40), stopAt(3, 90)},
	}
	tripSegmentTrackers.Lock()
	tripSegmentTrackers.byTrip[layoverTripID] = tracker
	tripSegmentTrackers.Unlock()
	t.Cleanup(func() {
		tripSegmentTrackers.Lock()
		delete(tripSegmentTrackers.byTrip, layoverTripID)
		tripSegmentTrackers.Unlock()
	})
	return tracker
}

func vehicleOnShape(id int, tracker *tripSegmentTracker, idx int, at time.Time) models.Vehicle {
	return models.Vehicle{
		ID: id, Latitude: tracker.Shape[idx].ShapePtLat, Longitude: tracker.Shape[idx].ShapePtLon,
		Timestamp: at.Format(time.RFC3339), RouteID: tracker.RouteID, TripID: layoverTripID,
	}
}

func TestFirstSegmentIsTimedFromLeavingTheTerminus(t *testing.T) {
	tracker := installStraightTracker(t)
	start := time.Now().Add(-9 * time.Minute).Truncate(time.Second)

	var sample pendingSegmentSample
	var status segmentObservationStatus
	at := start
	// Eight minutes of layover at the departure terminus, then a 60 s run to the next stop.
	for ; at.Before(start.Add(8 * time.Minute)); at = at.Add(30 * time.Second) {
		_, status = observeVehicleSegment(time.Local, vehicleOnShape(990001, tracker, 0, at))
	}
	left := at.Add(-30 * time.Second)
	_, _ = observeVehicleSegment(time.Local, vehicleOnShape(990001, tracker, 20, left.Add(30*time.Second)))
	sample, status = observeVehicleSegment(time.Local, vehicleOnShape(990001, tracker, 40, left.Add(60*time.Second)))

	if status != segmentObservedAccepted {
		t.Fatalf("status = %s, want accepted", status)
	}
	if got := sample.Duration; got != 60*time.Second {
		t.Fatalf("first segment took %s, want 1m0s: the layover was counted as travel", got)
	}
}

func TestIntermediateStopKeepsItsDwell(t *testing.T) {
	tracker := installStraightTracker(t)
	start := time.Now().Add(-9 * time.Minute).Truncate(time.Second)

	_, _ = observeVehicleSegment(time.Local, vehicleOnShape(990002, tracker, 30, start))
	_, _ = observeVehicleSegment(time.Local, vehicleOnShape(990002, tracker, 40, start.Add(30*time.Second)))
	_, _ = observeVehicleSegment(time.Local, vehicleOnShape(990002, tracker, 40, start.Add(90*time.Second)))
	_, _ = observeVehicleSegment(time.Local, vehicleOnShape(990002, tracker, 65, start.Add(150*time.Second)))
	sample, status := observeVehicleSegment(time.Local, vehicleOnShape(990002, tracker, 90, start.Add(210*time.Second)))

	if status != segmentObservedAccepted {
		t.Fatalf("status = %s, want accepted", status)
	}
	if got := sample.Duration; got != 180*time.Second {
		t.Fatalf("segment took %s, want 3m0s including the minute stood at the stop", got)
	}
}

func TestLayoverSamplesAreDroppedOnce(t *testing.T) {
	withTestDB(t)
	mustExec := func(q string, args ...any) {
		t.Helper()
		if _, err := database.DB.Exec(q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	mustExec(`INSERT INTO trips (trip_id, route_id, direction_id, trip_headsign, block_id, shape_id, wheelchair_accessible, bikes_allowed) VALUES ('25_0', 25, 0, '', 0, '25_0', 0, 0)`)
	for seq, stop := range []int{100, 200, 300} {
		mustExec(`INSERT INTO api_stop_times (trip_id, stop_id, stop_sequence) VALUES ('25_0', ?, ?)`, stop, seq)
	}
	for _, from := range []int{100, 200} {
		mustExec(`INSERT INTO segment_travel_time_samples (route_id, direction_id, from_stop_id, to_stop_id, day_type, bucket_start_min, duration_sec, observed_at) VALUES (25, 0, ?, ?, 'weekday', 720, 412, ?)`, from, from+100, time.Now().Unix())
		mustExec(`INSERT INTO segment_travel_time_profiles (route_id, direction_id, from_stop_id, to_stop_id, day_type, bucket_start_min, sample_count, median_sec, p75_sec, updated_at) VALUES (25, 0, ?, ?, 'weekday', 720, 9, 412, 430, ?)`, from, from+100, time.Now().Unix())
	}
	// The same stop leaving the other direction is not that direction's terminus.
	mustExec(`INSERT INTO segment_travel_time_samples (route_id, direction_id, from_stop_id, to_stop_id, day_type, bucket_start_min, duration_sec, observed_at) VALUES (25, 1, 100, 400, 'weekday', 720, 80, ?)`, time.Now().Unix())
	mustExec(`PRAGMA user_version = 0`)

	if err := database.InitSchemas(); err != nil {
		t.Fatalf("init schemas: %v", err)
	}

	count := func(table string, direction, from int) int {
		var n int
		if err := database.DB.QueryRow(`SELECT COUNT(*) FROM `+table+` WHERE direction_id = ? AND from_stop_id = ?`, direction, from).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	for _, table := range []string{"segment_travel_time_samples", "segment_travel_time_profiles"} {
		if n := count(table, 0, 100); n != 0 {
			t.Errorf("%s still has %d rows out of the terminus", table, n)
		}
		if n := count(table, 0, 200); n != 1 {
			t.Errorf("%s lost the intermediate segment: %d rows", table, n)
		}
	}
	if n := count("segment_travel_time_samples", 1, 100); n != 1 {
		t.Errorf("opposite direction sample was dropped")
	}

	mustExec(`INSERT INTO segment_travel_time_samples (route_id, direction_id, from_stop_id, to_stop_id, day_type, bucket_start_min, duration_sec, observed_at) VALUES (25, 0, 100, 200, 'weekday', 720, 70, ?)`, time.Now().Unix())
	if err := database.InitSchemas(); err != nil {
		t.Fatalf("init schemas: %v", err)
	}
	if n := count("segment_travel_time_samples", 0, 100); n != 1 {
		t.Errorf("a clean sample learned after the cleanup was dropped on restart")
	}
}
