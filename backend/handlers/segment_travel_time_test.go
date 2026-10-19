package handlers

import (
	"conexiuni-cluj/models"
	"testing"
	"time"
)

const layoverTripID = "9990_0"

func (city *City) installStraightTracker(t *testing.T) *tripSegmentTracker {
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
	city.tripSegmentTrackers.Lock()
	city.tripSegmentTrackers.byTrip[layoverTripID] = tracker
	city.tripSegmentTrackers.Unlock()
	t.Cleanup(func() {
		city.tripSegmentTrackers.Lock()
		delete(city.tripSegmentTrackers.byTrip, layoverTripID)
		city.tripSegmentTrackers.Unlock()
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
	city := newTestCity(t)
	tracker := city.installStraightTracker(t)
	start := time.Now().Add(-9 * time.Minute).Truncate(time.Second)

	var sample pendingSegmentSample
	var status segmentObservationStatus
	at := start
	// Eight minutes of layover at the departure terminus, then a 60 s run to the next stop.
	for ; at.Before(start.Add(8 * time.Minute)); at = at.Add(30 * time.Second) {
		_, status = city.observeVehicleSegment(time.Local, vehicleOnShape(990001, tracker, 0, at))
	}
	left := at.Add(-30 * time.Second)
	_, _ = city.observeVehicleSegment(time.Local, vehicleOnShape(990001, tracker, 20, left.Add(30*time.Second)))
	sample, status = city.observeVehicleSegment(time.Local, vehicleOnShape(990001, tracker, 40, left.Add(60*time.Second)))

	if status != segmentObservedAccepted {
		t.Fatalf("status = %s, want accepted", status)
	}
	if got := sample.Duration; got != 60*time.Second {
		t.Fatalf("first segment took %s, want 1m0s: the layover was counted as travel", got)
	}
}

func TestIntermediateStopKeepsItsDwell(t *testing.T) {
	city := newTestCity(t)
	tracker := city.installStraightTracker(t)
	start := time.Now().Add(-9 * time.Minute).Truncate(time.Second)

	_, _ = city.observeVehicleSegment(time.Local, vehicleOnShape(990002, tracker, 30, start))
	_, _ = city.observeVehicleSegment(time.Local, vehicleOnShape(990002, tracker, 40, start.Add(30*time.Second)))
	_, _ = city.observeVehicleSegment(time.Local, vehicleOnShape(990002, tracker, 40, start.Add(90*time.Second)))
	_, _ = city.observeVehicleSegment(time.Local, vehicleOnShape(990002, tracker, 65, start.Add(150*time.Second)))
	sample, status := city.observeVehicleSegment(time.Local, vehicleOnShape(990002, tracker, 90, start.Add(210*time.Second)))

	if status != segmentObservedAccepted {
		t.Fatalf("status = %s, want accepted", status)
	}
	if got := sample.Duration; got != 180*time.Second {
		t.Fatalf("segment took %s, want 3m0s including the minute stood at the stop", got)
	}
}
