package handlers

import (
	"conexiuni-cluj/models"
	"testing"
	"time"
)

func resetSegmentTrackerForTest(tracker *tripSegmentTracker) {
	tripSegmentTrackers.Lock()
	tripSegmentTrackers.byTrip = map[string]*tripSegmentTracker{tracker.TripID: tracker}
	tripSegmentTrackers.Unlock()

	vehicleSegmentStates.Lock()
	vehicleSegmentStates.byVehicle = make(map[int]vehicleSegmentState)
	vehicleSegmentStates.Unlock()
}

func testSegmentTracker() *tripSegmentTracker {
	shape := make([]models.Shape, 21)
	for i := range shape {
		shape[i] = models.Shape{
			ShapeID:         "test_shape",
			ShapePtLat:      46.0,
			ShapePtLon:      23.0 + float64(i)*0.001,
			ShapePtSequence: i,
		}
	}
	return &tripSegmentTracker{
		TripID:      "1_0",
		RouteID:     1,
		DirectionID: 0,
		Shape:       shape,
		Cumulative:  buildCumulativeShapeDistance(shape),
		Stops: []trackedSegmentStop{
			{StopID: 10, ShapeIdx: 0, Lat: shape[0].ShapePtLat, Lon: shape[0].ShapePtLon},
			{StopID: 20, ShapeIdx: 10, Lat: shape[10].ShapePtLat, Lon: shape[10].ShapePtLon},
			{StopID: 30, ShapeIdx: 20, Lat: shape[20].ShapePtLat, Lon: shape[20].ShapePtLon},
		},
	}
}

func testVehicleAtShapePoint(tracker *tripSegmentTracker, shapeIdx int, observedAt time.Time) models.Vehicle {
	point := tracker.Shape[shapeIdx]
	return models.Vehicle{
		ID:        100,
		Latitude:  point.ShapePtLat,
		Longitude: point.ShapePtLon,
		Timestamp: observedAt.Format(time.RFC3339),
		RouteID:   tracker.RouteID,
		TripID:    tracker.TripID,
		Speed:     20,
	}
}

func TestObserveVehicleSegmentDoesNotLearnFromMidSegmentStart(t *testing.T) {
	tracker := testSegmentTracker()
	resetSegmentTrackerForTest(tracker)

	now := time.Now()
	_, status := observeVehicleSegment(time.Local, testVehicleAtShapePoint(tracker, 5, now))
	if status != segmentObservedReset {
		t.Fatalf("first observation status = %s, want reset", status)
	}

	_, status = observeVehicleSegment(time.Local, testVehicleAtShapePoint(tracker, 10, now.Add(5*time.Minute)))
	if status == segmentObservedAccepted {
		t.Fatalf("mid-segment start was accepted as a full stop-to-stop sample")
	}
}

func TestObserveVehicleSegmentLearnsWhenSeenAtBothStops(t *testing.T) {
	tracker := testSegmentTracker()
	resetSegmentTrackerForTest(tracker)

	now := time.Now()
	_, status := observeVehicleSegment(time.Local, testVehicleAtShapePoint(tracker, 0, now))
	if status != segmentObservedReset {
		t.Fatalf("first observation status = %s, want reset", status)
	}

	sample, status := observeVehicleSegment(time.Local, testVehicleAtShapePoint(tracker, 10, now.Add(90*time.Second)))
	if status != segmentObservedAccepted {
		t.Fatalf("second observation status = %s, want accepted", status)
	}
	if sample.Key.FromStopID != 10 || sample.Key.ToStopID != 20 {
		t.Fatalf("sample pair = %d -> %d, want 10 -> 20", sample.Key.FromStopID, sample.Key.ToStopID)
	}
}
