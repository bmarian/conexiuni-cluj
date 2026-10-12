package handlers

import (
	"conexiuni-cluj/models"
	"conexiuni-cluj/predictions"
	"fmt"
	"math"
	"testing"
	"time"
)

const turnRouteID = 9992

// Direction 0 runs north over 100 points; direction 1 runs the same line back, so its
// first stop sits where direction 0 ends.
func installTurnaroundTrackers(t *testing.T) (*tripSegmentTracker, *tripSegmentTracker) {
	t.Helper()
	line := func(id string, reverse bool) []models.Shape {
		shape := make([]models.Shape, 100)
		for i := range shape {
			k := i
			if reverse {
				k = 99 - i
			}
			shape[i] = models.Shape{ShapeID: id, ShapePtLat: 46.77 + float64(k)*0.0001, ShapePtLon: 23.6, ShapePtSequence: i}
		}
		return shape
	}
	build := func(tripID string, dir int, shape []models.Shape, ids []int, idx []int) *tripSegmentTracker {
		stops := make([]trackedSegmentStop, len(ids))
		for i := range ids {
			stops[i] = trackedSegmentStop{StopID: ids[i], ShapeIdx: idx[i], Lat: shape[idx[i]].ShapePtLat, Lon: shape[idx[i]].ShapePtLon}
		}
		return &tripSegmentTracker{TripID: tripID, RouteID: turnRouteID, DirectionID: dir, Shape: shape,
			Cumulative: buildCumulativeShapeDistance(shape), Stops: stops}
	}
	out := build(fmt.Sprintf("%d_0", turnRouteID), 0, line("f", false), []int{1, 2, 3}, []int{0, 40, 90})
	in := build(fmt.Sprintf("%d_1", turnRouteID), 1, line("r", true), []int{4, 5, 6}, []int{0, 50, 99})
	tripSegmentTrackers.Lock()
	tripSegmentTrackers.byTrip[out.TripID] = out
	tripSegmentTrackers.byTrip[in.TripID] = in
	tripSegmentTrackers.Unlock()
	t.Cleanup(func() {
		tripSegmentTrackers.Lock()
		delete(tripSegmentTrackers.byTrip, out.TripID)
		delete(tripSegmentTrackers.byTrip, in.TripID)
		tripSegmentTrackers.Unlock()
		clearPendingTurnaround(992001)
		turnaroundWrites.Lock()
		turnaroundWrites.samples = nil
		turnaroundWrites.Unlock()
	})
	return out, in
}

func vehicleOn(id int, tracker *tripSegmentTracker, idx int, at time.Time) models.Vehicle {
	return models.Vehicle{
		ID: id, Latitude: tracker.Shape[idx].ShapePtLat, Longitude: tracker.Shape[idx].ShapePtLon,
		Timestamp: at.Format(time.RFC3339), RouteID: tracker.RouteID, TripID: tracker.TripID,
	}
}

func TestTripFlipClosesTheLastSegmentAndTimesTheStand(t *testing.T) {
	withTestDB(t)
	out, in := installTurnaroundTrackers(t)
	start := time.Now().Add(-8 * time.Minute).Truncate(time.Second)
	segmentClock = func() time.Time { return start.Add(8 * time.Minute) }
	t.Cleanup(func() { segmentClock = time.Now })

	// Leaves the middle stop, and 90 s later Tranzy has it on direction 1, 110 m short of the end.
	_, _ = observeVehicleSegment(time.Local, vehicleOn(992001, out, 40, start))
	_, _ = observeVehicleSegment(time.Local, vehicleOn(992001, out, 60, start.Add(45*time.Second)))
	flipAt := start.Add(90 * time.Second)
	flipped := vehicleOn(992001, in, 0, flipAt)
	flipped.Latitude = out.Shape[89].ShapePtLat
	sample, status := observeVehicleSegment(time.Local, flipped)
	if status != segmentObservedAccepted {
		t.Fatalf("flip status = %s, want accepted", status)
	}
	if sample.Key.DirectionID != 0 || sample.Key.FromStopID != 2 || sample.Key.ToStopID != 3 || math.Abs(sample.Duration.Seconds()-90) > 0.5 {
		t.Fatalf("flip closed %+v, want stop 2 -> 3 in 90 s", sample)
	}

	// Stands at the new first stop for two minutes, then runs to the next stop.
	at := flipAt
	for i := 0; i < 4; i++ {
		at = at.Add(30 * time.Second)
		_, _ = observeVehicleSegment(time.Local, vehicleOn(992001, in, 0, at))
	}
	departedAt := at
	_, _ = observeVehicleSegment(time.Local, vehicleOn(992001, in, 25, at.Add(30*time.Second)))
	sample, status = observeVehicleSegment(time.Local, vehicleOn(992001, in, 50, at.Add(60*time.Second)))
	if status != segmentObservedAccepted || sample.Key.DirectionID != 1 || sample.Key.FromStopID != 4 {
		t.Fatalf("first segment of the new trip: status=%s key=%+v", status, sample.Key)
	}

	turnaroundWrites.Lock()
	samples := append([]turnaroundSample(nil), turnaroundWrites.samples...)
	turnaroundWrites.Unlock()
	if len(samples) != 1 {
		t.Fatalf("turnaround samples = %d, want 1", len(samples))
	}
	got := samples[0]
	if got.RouteID != turnRouteID || got.DirectionID != 1 || !got.ArrivedAt.Equal(flipAt) || !got.DepartedAt.Equal(departedAt) {
		t.Fatalf("turnaround = %+v, want arrival %s departure %s on direction 1", got, flipAt, departedAt)
	}
	if stored := flushTurnaroundSamples(); stored != 1 {
		t.Fatalf("stored %d turnaround samples, want 1", stored)
	}
	profile := loadTerminusProfile(turnRouteID, 1)
	if profile.Departures != 1 || profile.Arrivals != 1 {
		t.Fatalf("profile = %+v, want one arrival that departed", profile)
	}
}

func TestFlipAwayFromTheTerminusIsJustAReset(t *testing.T) {
	withTestDB(t)
	out, in := installTurnaroundTrackers(t)
	start := time.Now().Add(-5 * time.Minute).Truncate(time.Second)
	segmentClock = func() time.Time { return start.Add(5 * time.Minute) }
	t.Cleanup(func() { segmentClock = time.Now })

	_, _ = observeVehicleSegment(time.Local, vehicleOn(992001, out, 40, start))
	// Reassigned mid-route, nowhere near the other direction's first stop.
	reassigned := vehicleOn(992001, in, 60, start.Add(60*time.Second))
	_, status := observeVehicleSegment(time.Local, reassigned)
	if status != segmentObservedReset {
		t.Fatalf("status = %s, want reset", status)
	}
	turnaroundPending.Lock()
	_, pending := turnaroundPending.byVehicle[992001]
	turnaroundPending.Unlock()
	if pending {
		t.Fatalf("a mid-route reassignment must not start a turnaround")
	}
}

func TestThroughRunningFromTheTimetable(t *testing.T) {
	withTestDB(t)
	resetTurnaroundCache()
	stop := func(trip string, id, seq int, lat float64, offset float64) models.StopTime {
		return models.StopTime{TripID: trip, StopID: id, StopSequence: seq, StopLat: lat, StopLon: 23.6, OffsetArrivalTime: offset, RouteShortName: "T7"}
	}
	out := []models.StopTime{stop("7_0", 1, 0, 46.770, 0), stop("7_0", 2, 1, 46.775, 300), stop("7_0", 3, 2, 46.780, 300)}
	in := []models.StopTime{stop("7_1", 4, 0, 46.7801, 0), stop("7_1", 5, 1, 46.775, 300), stop("7_1", 6, 2, 46.7701, 300)}
	var entries []models.TimetableEntry
	for m := 6 * 60; m < 20*60; m += 25 {
		// Direction 1 leaves 12 minutes after direction 0, two past the ten-minute ride;
		// direction 0 leaves 3 minutes after direction 1 gets back.
		entries = append(entries, models.TimetableEntry{
			DepartureIn: fmt.Sprintf("%02d:%02d", m/60, m%60), DepartureOut: fmt.Sprintf("%02d:%02d", (m+12)/60, (m+12)%60)})
	}
	timetable := &models.Timetable{Weekdays: models.DaySchedule{Entries: entries}}

	toIn := turnaroundFor(7, 1, out, in, timetable)
	if !toIn.Through || toIn.StandSec != predictions.DefaultStandSec || toIn.MaxWaitMin != 0 {
		t.Fatalf("direction 1 = %+v, want through with the default stand", toIn)
	}
	// Direction 0 departs 3 minutes after direction 1 arrives, also through.
	toOut := turnaroundFor(7, 0, in, out, timetable)
	if !toOut.Through {
		t.Fatalf("direction 0 = %+v, want through", toOut)
	}
	far := append([]models.StopTime(nil), in...)
	far[0].StopLat = 46.79
	if got := turnaroundFor(7, 1, out, far, timetable); got.Through {
		t.Fatalf("a terminus 1 km away is not a turnaround: %+v", got)
	}
}
