package live

import (
	"testing"
	"time"
)

func (eng *Engine) recordTimedRun(t *testing.T, left time.Time, stretchSec ...float64) {
	t.Helper()
	start := left
	for i, sec := range stretchSec {
		end := start.Add(time.Duration(sec * float64(time.Second)))
		if _, err := eng.DB.Exec(`INSERT INTO segment_travel_time_samples
			(route_id, direction_id, from_stop_id, to_stop_id, day_type, bucket_start_min, duration_sec, observed_at)
			VALUES (?, 0, ?, ?, ?, ?, ?, ?)`,
			delayRouteID, 100+i, 101+i, eng.profileDay(end), segmentBucketStartMin(end), sec, end.Unix()); err != nil {
			t.Fatalf("sample: %v", err)
		}
		start = end
	}
}

func TestLineStatsSumsStretchMeans(t *testing.T) {
	eng := newTestCity(t)
	eng.installDelayRoute(t)
	for _, day := range []int{10, 11, 14, 15, 16} {
		eng.recordTimedRun(t, weekdayAt(day, 8, 3), 120, 300, 120)
		eng.recordTimedRun(t, weekdayAt(day, 19, 3), 120, 120, 120)
	}
	eng.recordTimedRun(t, weekdayAt(17, 12, 3), 120, 120, 120)
	for _, row := range [][]float64{{allDaySegmentBucket, 90}, {8 * 60, 120}} {
		if _, err := eng.DB.Exec(`INSERT INTO schedule_delay_profiles
			(route_id, direction_id, day_type, bucket_start_min, run_count, delay_sec, spread_sec, updated_at)
			VALUES (?, 0, 'weekday', ?, 10, ?, 0, 0)`, delayRouteID, row[0], row[1]); err != nil {
			t.Fatalf("delay: %v", err)
		}
	}

	stats, err := eng.LineStats(delayRouteID, weekdayAt(23, 12, 0))
	if err != nil {
		t.Fatal(err)
	}
	if stats.DayType != "weekday" || len(stats.Directions) != 2 {
		t.Fatalf("stats = %+v", stats)
	}
	d := stats.Directions[0]
	if d.TripSec == nil || *d.TripSec != 120+(5*300+6*120)/11.0+120 {
		t.Fatalf("trip = %v, want the summed stretch means", d.TripSec)
	}
	if d.DelaySec == nil || *d.DelaySec != 90 {
		t.Fatalf("delay = %v, want 90", d.DelaySec)
	}
	if len(d.Hours) != 12 || d.Hours[0].Hour != 8 || d.Hours[11].Hour != 19 {
		t.Fatalf("hours = %+v, want every hour from 8 to 19", d.Hours)
	}
	for _, h := range d.Hours[1:11] {
		if h.TripSec != nil {
			t.Fatalf("%d:00 timed from too few buses", h.Hour)
		}
	}
	if eight, last := d.Hours[0], d.Hours[11]; *eight.TripSec != 540 || *eight.DelaySec != 120 || *last.TripSec != 360 || last.DelaySec != nil {
		t.Fatalf("08 = %+v, 19 = %+v", eight, last)
	}
	if s := d.Slowest; s == nil || s.From != "S1" || s.To != "S2" || s.Hour != 8 || s.ExtraSec != 180 {
		t.Fatalf("slowest = %+v, want S1 → S2 three minutes slower at 8", s)
	}
	if other := stats.Directions[1]; other.TripSec != nil || len(other.Hours) != 0 {
		t.Fatalf("the route has no direction 1: %+v", other)
	}
}
