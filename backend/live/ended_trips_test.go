package live

import (
	"conexiuni-cluj/models"
	"testing"
	"time"
)

func TestLateFlipStillTimesTheStand(t *testing.T) {
	eng := newTestCity(t)
	out, in := eng.installTurnaroundTrackers(t)
	start := time.Now().Add(-10 * time.Minute).Truncate(time.Second)
	segmentClock = func() time.Time { return start.Add(10 * time.Minute) }
	t.Cleanup(func() { segmentClock = time.Now })
	observe := func(v models.Vehicle) models.Vehicle {
		vehicles := []models.Vehicle{v}
		eng.followEndedTrips(vehicles)
		_, _ = eng.observeVehicleSegment(time.Local, vehicles[0])
		return vehicles[0]
	}
	reportedOnOut := func(idx int, at time.Time) models.Vehicle {
		v := vehicleOn(992001, in, idx, at)
		v.TripID = out.TripID
		return v
	}

	observe(vehicleOn(992001, out, 40, start))
	arrivedAt := start.Add(time.Minute)
	got := observe(vehicleOn(992001, out, 90, arrivedAt))
	if got.TripID != in.TripID || got.PrevTripID != out.TripID || got.TripChangedAt != arrivedAt.Format(time.RFC3339) {
		t.Fatalf("at the end of its trip: %+v, want on %s since %s", got, in.TripID, arrivedAt.Format(time.RFC3339))
	}

	// Stands at the other direction's first stop and leaves, still reported on direction 0.
	departedAt := arrivedAt.Add(3 * time.Minute)
	for at := arrivedAt.Add(30 * time.Second); !at.After(departedAt); at = at.Add(30 * time.Second) {
		observe(reportedOnOut(0, at))
	}
	got = observe(reportedOnOut(50, departedAt.Add(time.Minute)))
	if got.TripID != in.TripID {
		t.Fatalf("on its way back: trip %s, want %s", got.TripID, in.TripID)
	}
	eng.turnaroundWrites.Lock()
	samples := append([]turnaroundSample(nil), eng.turnaroundWrites.samples...)
	eng.turnaroundWrites.Unlock()
	if len(samples) != 1 || !samples[0].ArrivedAt.Equal(arrivedAt) || !samples[0].DepartedAt.Equal(departedAt) {
		t.Fatalf("turnarounds = %+v, want one from %s to %s", samples, arrivedAt, departedAt)
	}

	caughtUp := vehicleOn(992001, in, 60, departedAt.Add(90*time.Second))
	caughtUp.PrevTripID, caughtUp.TripChangedAt = out.TripID, caughtUp.Timestamp
	if got = observe(caughtUp); got.TripChangedAt != arrivedAt.Format(time.RFC3339) {
		t.Fatalf("once the feed flips: changed at %s, want the arrival %s", got.TripChangedAt, arrivedAt.Format(time.RFC3339))
	}

	nextRun := vehicleOn(992001, out, 0, departedAt.Add(20*time.Minute))
	nextRun.PrevTripID, nextRun.TripChangedAt = in.TripID, nextRun.Timestamp
	if got = observe(nextRun); got.TripID != out.TripID || got.PrevTripID != in.TripID {
		t.Fatalf("next run: %+v, want on %s", got, out.TripID)
	}
}
