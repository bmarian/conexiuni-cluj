package handlers

import (
	"fmt"
	"slices"
	"testing"
)

func TestHourlyStopOffsetsFollowEachHoursDelay(t *testing.T) {
	stops := installDelayRoute(t)
	for _, day := range []int{10, 11, 14, 15, 16, 17, 18, 21} {
		recordRun(t, weekdayAt(day, 8, 3))
		recordRun(t, weekdayAt(day, 14, 1))
	}
	if _, err := recomputeScheduleDelays(delayZone, weekdayAt(23, 12, 0)); err != nil {
		t.Fatalf("recompute: %v", err)
	}
	tripID := fmt.Sprintf("%d_0", delayRouteID)

	trip, ok := hourlyStopOffsets(stops, delayRouteID, weekdayAt(23, 0, 0))[tripID]
	if !ok {
		t.Fatal("trip missing")
	}
	if want := []int{100, 101, 102, 103}; !slices.Equal(trip.StopIDs, want) {
		t.Fatalf("stop ids = %v, want %v", trip.StopIDs, want)
	}
	if len(trip.HourlyOffsetSeconds) != 24 {
		t.Fatalf("got %d hours, want 24", len(trip.HourlyOffsetSeconds))
	}
	if got, want := trip.HourlyOffsetSeconds[8], []int{80, 200, 320, 440}; !slices.Equal(got, want) {
		t.Fatalf("08:00 offsets = %v, want %v", got, want)
	}
	if got, want := trip.HourlyOffsetSeconds[14], []int{27, 147, 267, 387}; !slices.Equal(got, want) {
		t.Fatalf("14:00 offsets = %v, want %v", got, want)
	}

	saturday := hourlyStopOffsets(stops, delayRouteID, weekdayAt(26, 0, 0))[tripID]
	if got, want := saturday.HourlyOffsetSeconds[8], []int{0, 120, 240, 360}; !slices.Equal(got, want) {
		t.Fatalf("saturday 08:00 offsets = %v, want %v", got, want)
	}
}

func TestDayOfType(t *testing.T) {
	wednesday := weekdayAt(23, 10, 0)
	for dayType, want := range map[string]int{"weekday": 23, "saturday": 26, "sunday": 27, "bogus": 23} {
		if got := dayOfType(wednesday, dayType).Day(); got != want {
			t.Errorf("%s: day %d, want %d", dayType, got, want)
		}
	}
}
