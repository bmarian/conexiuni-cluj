package live

import (
	"conexiuni-cluj/database"
	"conexiuni-cluj/predictions"
	"fmt"
	"testing"
	"time"
)

func TestAccuracyStatsKey(t *testing.T) {
	cases := []struct {
		shown  shownTime
		errSec int64
		want   string
	}{
		{shownTime{minutes: 1, kind: predictions.EtaOnTrip}, 0, "on_trip|1-2|0"},
		{shownTime{minutes: 4, kind: predictions.EtaStanding}, -1, "standing|3-5|-1"},
		{shownTime{minutes: 12, kind: predictions.EtaFeeder}, 59, "feeder|11-20|1"},
		{shownTime{minutes: 30, kind: predictions.EtaOnTrip}, 5000, "on_trip|21-30|30"},
		{shownTime{minutes: 7, kind: predictions.EtaOnTrip}, -5000, "on_trip|6-10|-30"},
	}
	for _, c := range cases {
		if got := accuracyStatsKey(c.shown, c.errSec); got != c.want {
			t.Errorf("accuracyStatsKey(%+v, %d) = %q, want %q", c.shown, c.errSec, got, c.want)
		}
	}
}

func TestShownTimesScoredOnArrival(t *testing.T) {
	eng := newTestCity(t)
	eng.accuracy.pending = make(map[accuracyKey]*pendingArrival)
	asOf := time.Unix(1_000_000, 0)
	table := func(at time.Time, eta int64) *liveTable {
		return &liveTable{AsOf: at, rowsByTrip: map[string][]liveRow{
			"5_0": {{TripID: "5_0", StopID: 10, Entries: []liveEntry{{At: at.Unix() + eta, Live: true, VehicleID: 7}}}},
		}}
	}
	eng.noteShownTimes(table(asOf, 300))
	eng.noteShownTimes(table(asOf.Add(20*time.Second), 290))
	eng.noteShownTimes(table(asOf.Add(60*time.Second), 240))
	p := eng.accuracy.pending[accuracyKey{vehicle: 7, trip: "5_0", stop: 10}]
	if p == nil || len(p.shown) != 2 {
		t.Fatalf("want 2 shown minutes (5, 4), got %+v", p)
	}
	eng.noteArrival(7, 5, "5_0", 10, asOf.Add(330*time.Second))
	if len(eng.accuracy.pending) != 0 {
		t.Fatalf("arrival left %d pending", len(eng.accuracy.pending))
	}

	eng.noteShownTimes(table(asOf, 300))
	eng.noteShownTimes(&liveTable{AsOf: asOf.Add(20 * time.Second)})
	eng.noteShownTimes(&liveTable{AsOf: asOf.Add(40 * time.Second)})
	if len(eng.accuracy.pending) != 0 {
		t.Fatalf("a bus gone from the table for two polls is still pending")
	}
}

func TestTimetableArrivalScoredAgainstOursAndCTP(t *testing.T) {
	eng := newTestCity(t)
	eng.installDelayRoute(t)
	eng.tripScheduleCache.Lock()
	eng.tripScheduleCache.departures = nil
	eng.tripScheduleCache.Unlock()
	at := weekdayAt(23, 8, 7)
	if _, err := eng.DB.Exec(`INSERT INTO schedule_delay_profiles
		(route_id, direction_id, day_type, bucket_start_min, run_count, delay_sec, spread_sec, updated_at)
		VALUES (?, 0, 'weekday', ?, 20, 180, 30, 0)`, delayRouteID, segmentBucketStartMin(at)); err != nil {
		t.Fatalf("delay: %v", err)
	}
	tracker := &tripSegmentTracker{TripID: fmt.Sprintf("%d_0", delayRouteID), RouteID: delayRouteID}

	counts := func() map[string]int {
		rows, err := eng.MetricRows("timetable_error", database.StatsDaysAgo(0))
		if err != nil {
			t.Fatalf("rows: %v", err)
		}
		out := map[string]int{}
		for _, r := range rows {
			out[r.Key] += r.Count
		}
		return out
	}
	before := counts()
	// 08:00 run at the third stop at 08:07: 3 min late by CTP, on time by our learned delay.
	eng.noteTimetableArrival(tracker, 2, 8*60, at, delayZone)
	got := map[string]int{}
	for k, n := range counts() {
		if d := n - before[k]; d != 0 {
			got[k] = d
		}
	}
	if got["ours|0"] != 1 || got["ctp|6"] != 1 || len(got) != 2 {
		t.Errorf("want ours on time and ctp 3 min late, got %v", got)
	}
}
