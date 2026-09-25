package handlers

import (
	"conexiuni-cluj/database"
	"conexiuni-cluj/models"
	"fmt"
	"math"
	"testing"
	"time"
)

const delayRouteID = 9991

var delayZone = time.FixedZone("EEST", 3*3600)

// Route with four stops two minutes apart and a departure every 20 min, 06:00-20:00.
func installDelayRoute(t *testing.T) []models.StopTime {
	t.Helper()
	withTestDB(t)
	if _, err := database.DB.Exec(`INSERT INTO routes (route_id, agency_id, route_short_name, route_long_name, route_type, route_desc, route_color)
		VALUES (?, 2, 'T1', 'A - B', 3, '', '#000000')`, delayRouteID); err != nil {
		t.Fatalf("route: %v", err)
	}
	var entries []models.TimetableEntry
	for m := 6 * 60; m <= 20*60; m += 20 {
		entries = append(entries, models.TimetableEntry{DepartureIn: fmt.Sprintf("%02d:%02d", m/60, m%60)})
	}
	if err := storeTimetableInDB(&models.Timetable{RouteShortName: "T1", InStopName: "A", OutStopName: "B",
		Weekdays: models.DaySchedule{Entries: entries}}); err != nil {
		t.Fatalf("timetable: %v", err)
	}
	stops := make([]models.StopTime, 4)
	for i := range stops {
		offset := 120.0
		if i == 0 {
			offset = 0
		}
		stops[i] = models.StopTime{TripID: fmt.Sprintf("%d_0", delayRouteID), StopID: 100 + i, OffsetArrivalTime: offset,
			StopSequence: i, StopHeadsign: fmt.Sprintf("S%d", i), RouteShortName: "T1"}
	}
	if err := storeStopTimesInDB(stops); err != nil {
		t.Fatalf("stop times: %v", err)
	}
	return stops
}

// One run that left the first stop at `left` and took two minutes per stop.
func recordRun(t *testing.T, left time.Time) {
	t.Helper()
	start := left
	for i := 0; i < 3; i++ {
		end := start.Add(2 * time.Minute)
		if _, err := database.DB.Exec(`INSERT INTO segment_travel_time_samples
			(route_id, direction_id, from_stop_id, to_stop_id, day_type, bucket_start_min, duration_sec, observed_at)
			VALUES (?, 0, ?, ?, ?, ?, ?, ?)`,
			delayRouteID, 100+i, 101+i, segmentDayType(end), segmentBucketStartMin(end), end.Sub(start).Seconds(), end.Unix()); err != nil {
			t.Fatalf("sample: %v", err)
		}
		start = end
	}
}

func weekdayAt(day, hour, minute int) time.Time {
	return time.Date(2026, time.September, day, hour, minute, 0, 0, delayZone)
}

func TestScheduleDelayLearnsHowLateRunsArrive(t *testing.T) {
	stops := installDelayRoute(t)
	// Eight weekdays of the 08:00 run leaving three minutes late.
	for _, day := range []int{10, 11, 14, 15, 16, 17, 18, 21} {
		recordRun(t, weekdayAt(day, 8, 3))
	}
	// Halfway between two departures: not a run the timetable can vouch for.
	recordRun(t, weekdayAt(22, 8, 10))

	if _, err := recomputeScheduleDelays(delayZone, weekdayAt(23, 12, 0)); err != nil {
		t.Fatalf("recompute: %v", err)
	}
	delay, ok := loadScheduleDelay(delayRouteID, 0, "weekday", 8*60)
	if want := 180.0 * 8 / 18; !ok || math.Abs(delay-want) > 0.01 {
		t.Fatalf("delay = %.2f (found %t), want %.2f: 3 min late, shrunk for 8 runs", delay, ok, want)
	}

	served := applySegmentProfilesToStopTimes(stops, delayRouteID, weekdayAt(23, 8, 10))
	if got := served[0].OffsetArrivalTime; got != 80 {
		t.Fatalf("first stop offset = %v, want 80", got)
	}
	for _, st := range served[1:] {
		if st.OffsetArrivalTime != 120 {
			t.Fatalf("stop %d offset = %v, want the untouched 120", st.StopID, st.OffsetArrivalTime)
		}
	}
}

func TestScheduleDelayNeedsEnoughRuns(t *testing.T) {
	stops := installDelayRoute(t)
	for _, day := range []int{15, 16, 17, 18} {
		recordRun(t, weekdayAt(day, 8, 3))
	}
	if _, err := recomputeScheduleDelays(delayZone, weekdayAt(23, 12, 0)); err != nil {
		t.Fatalf("recompute: %v", err)
	}
	if _, ok := loadScheduleDelay(delayRouteID, 0, "weekday", 8*60); ok {
		t.Fatal("learned a delay from four runs")
	}
	if got := applySegmentProfilesToStopTimes(stops, delayRouteID, weekdayAt(23, 8, 10))[0].OffsetArrivalTime; got != 0 {
		t.Fatalf("first stop offset = %v, want 0", got)
	}
}

func TestMatchDepartureAcrossMidnight(t *testing.T) {
	deps := []float64{23*60 + 50, 24*60 + 10}
	if got, ok := matchDeparture(deps, 5); !ok || got != 24*60+10 {
		t.Fatalf("00:05 matched %v (%t), want the 24:10 run", got, ok)
	}
	if _, ok := matchDeparture(deps, 24*60); ok {
		t.Fatal("00:00 sits 10 min from both runs and should not match")
	}
}
