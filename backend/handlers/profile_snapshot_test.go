package handlers

import (
	"math"
	"testing"
	"time"
)

func TestLearnedOffsetsHoldForTheQuarterHour(t *testing.T) {
	city := newTestCity(t)
	stops := city.installDelayRoute(t)
	now := weekdayAt(23, 8, 1)
	profileClock = func() time.Time { return now }
	t.Cleanup(func() { profileClock = time.Now })

	setProfile := func(median float64) {
		t.Helper()
		if _, err := city.DB.Exec(`INSERT INTO segment_travel_time_profiles
			(route_id, direction_id, from_stop_id, to_stop_id, day_type, bucket_start_min, sample_count, median_sec, p75_sec, updated_at)
			VALUES (?, 0, 100, 101, 'weekday', 480, 40, ?, ?, 0)
			ON CONFLICT(route_id, direction_id, from_stop_id, to_stop_id, day_type, bucket_start_min)
			DO UPDATE SET median_sec = excluded.median_sec`, delayRouteID, median, median); err != nil {
			t.Fatalf("profile: %v", err)
		}
	}
	served := func() float64 {
		return city.applySegmentProfilesToStopTimes(stops, delayRouteID, weekdayAt(23, 8, 10))[1].OffsetArrivalTime
	}

	setProfile(150)
	if got := served(); got != 150 {
		t.Fatalf("first answer = %v, want 150", got)
	}
	setProfile(170)
	now = now.Add(10 * time.Minute)
	if got := served(); got != 150 {
		t.Fatalf("same quarter hour = %v, want 150", got)
	}
	now = now.Add(5 * time.Minute)
	if got := served(); got != 170 {
		t.Fatalf("next quarter hour = %v, want 170", got)
	}
}

func TestConfidenceIsTheSampleShareUnrounded(t *testing.T) {
	city := newTestCity(t)
	stops := city.installDelayRoute(t)
	if _, err := city.DB.Exec(`INSERT INTO segment_travel_time_profiles
		(route_id, direction_id, from_stop_id, to_stop_id, day_type, bucket_start_min, sample_count, median_sec, p75_sec, updated_at)
		VALUES (?, 0, 100, 101, 'weekday', 480, 7, 150, 150, 0)`, delayRouteID); err != nil {
		t.Fatalf("profile: %v", err)
	}
	got := city.applySegmentProfilesToStopTimes(stops, delayRouteID, weekdayAt(23, 8, 10))[1].OffsetConfidence
	if math.Abs(got-7.0/17.0) > 1e-9 {
		t.Fatalf("confidence = %v, want 7/17 for seven samples in the exact bucket", got)
	}
}
