package handlers

import (
	"conexiuni-cluj/database"
	"conexiuni-cluj/predictions"
	"fmt"
	"testing"
	"time"
)

func TestClientKind(t *testing.T) {
	const chrome = "Mozilla/5.0 (Linux; Android 14) AppleWebKit/537.36 Chrome/129 Mobile Safari/537.36"
	cases := []struct {
		ua, fetchSite, want string
	}{
		{"ConexiuniClujMobile/1.4.0", "", "android/1.4.0"},
		{"ConexiuniClujWear/1.0.2", "", "wear/1.0.2"},
		{"ConexiuniClujMobile/<junk>", "", "android"},
		{chrome, "same-origin", "web"},
		{chrome, "", ""},
		{chrome, "cross-site", ""},
		{chrome, "none", ""},
		{"Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)", "same-origin", ""},
		{"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 HeadlessChrome/129 Safari/537.36", "same-origin", ""},
		{"python-requests/2.32", "", ""},
		{"ESP32HTTPClient", "", ""},
		{"", "", ""},
	}
	for _, c := range cases {
		if got := clientKind(c.ua, c.fetchSite); got != c.want {
			t.Errorf("clientKind(%q, %q) = %q, want %q", c.ua, c.fetchSite, got, c.want)
		}
	}
}

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

func TestErrorHistogramSummarize(t *testing.T) {
	// 6 on time, 2 a minute and a half late, 2 two minutes early.
	h := errorHistogram{0: 3, -1: 3, 3: 2, -4: 2}
	s := h.summarize("x")
	if s.N != 10 {
		t.Fatalf("N = %d", s.N)
	}
	if s.Within1 != 0.6 {
		t.Errorf("Within1 = %v, want 0.6", s.Within1)
	}
	if s.MedianAbs != 15 {
		t.Errorf("MedianAbs = %d, want 15", s.MedianAbs)
	}
	if s.P90Abs != 105 {
		t.Errorf("P90Abs = %d, want 105", s.P90Abs)
	}
	if (errorHistogram{}).summarize("empty").N != 0 {
		t.Error("empty histogram has samples")
	}
}

func TestShownTimesScoredOnArrival(t *testing.T) {
	city := newTestCity(t)
	city.accuracy.pending = make(map[accuracyKey]*pendingArrival)
	asOf := time.Unix(1_000_000, 0)
	table := func(at time.Time, eta int64) *liveTable {
		return &liveTable{AsOf: at, rowsByTrip: map[string][]liveRow{
			"5_0": {{TripID: "5_0", StopID: 10, Entries: []liveEntry{{At: at.Unix() + eta, Live: true, VehicleID: 7}}}},
		}}
	}
	city.noteShownTimes(table(asOf, 300))
	city.noteShownTimes(table(asOf.Add(20*time.Second), 290))
	city.noteShownTimes(table(asOf.Add(60*time.Second), 240))
	p := city.accuracy.pending[accuracyKey{vehicle: 7, trip: "5_0", stop: 10}]
	if p == nil || len(p.shown) != 2 {
		t.Fatalf("want 2 shown minutes (5, 4), got %+v", p)
	}
	city.noteArrival(7, "5_0", 10, asOf.Add(330*time.Second))
	if len(city.accuracy.pending) != 0 {
		t.Fatalf("arrival left %d pending", len(city.accuracy.pending))
	}

	city.noteShownTimes(table(asOf, 300))
	city.noteShownTimes(&liveTable{AsOf: asOf.Add(20 * time.Second)})
	city.noteShownTimes(&liveTable{AsOf: asOf.Add(40 * time.Second)})
	if len(city.accuracy.pending) != 0 {
		t.Fatalf("a bus gone from the table for two polls is still pending")
	}
}

func TestTimetableArrivalScoredAgainstOursAndCTP(t *testing.T) {
	city := newTestCity(t)
	city.installDelayRoute(t)
	city.tripScheduleCache.Lock()
	city.tripScheduleCache.departures = nil
	city.tripScheduleCache.Unlock()
	at := weekdayAt(23, 8, 7)
	if _, err := city.DB.Exec(`INSERT INTO schedule_delay_profiles
		(route_id, direction_id, day_type, bucket_start_min, run_count, delay_sec, spread_sec, updated_at)
		VALUES (?, 0, 'weekday', ?, 20, 180, 30, 0)`, delayRouteID, segmentBucketStartMin(at)); err != nil {
		t.Fatalf("delay: %v", err)
	}
	tracker := &tripSegmentTracker{TripID: fmt.Sprintf("%d_0", delayRouteID), RouteID: delayRouteID}

	counts := func() map[string]int {
		rows, err := city.MetricRows("timetable_error", database.StatsDaysAgo(0))
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
	city.noteTimetableArrival(tracker, 2, 8*60, at, delayZone)
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

func TestProblemLogGroupsRepeats(t *testing.T) {
	p := &problemLog{}
	_, _ = p.Write([]byte("2026/10/14 10:00:00 live hub: poll failed interval=10s subs=3 err=timeout\n"))
	_, _ = p.Write([]byte("2026/10/14 10:00:10 live hub: poll failed interval=12s subs=4 err=timeout\n"))
	_, _ = p.Write([]byte("2026/10/14 10:00:11 segment travel: snapshot vehicles=10 sample_errors=0 profile_errors=0\n"))
	got := p.Recent(time.Time{})
	if len(got) != 1 {
		t.Fatalf("want 1 grouped line, got %+v", got)
	}
	if got[0].Count != 2 || got[0].Text != "live hub: poll failed interval=12s subs=4 err=timeout" {
		t.Errorf("unexpected line %+v", got[0])
	}
}
