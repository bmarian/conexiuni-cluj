package handlers

import (
	"conexiuni-cluj/calendar"
	"conexiuni-cluj/database"
	"strings"
	"testing"
	"time"
)

func archiveSegmentSample(t *testing.T, at time.Time, duration float64) {
	t.Helper()
	if _, err := database.Archive.Exec(`INSERT INTO segment_travel_time_samples
		(route_id, direction_id, from_stop_id, to_stop_id, day_type, bucket_start_min, duration_sec, observed_at)
		VALUES (7, 0, 1, 2, 'weekday', 480, ?, ?)`, duration, at.Unix()); err != nil {
		t.Fatalf("archive sample: %v", err)
	}
}

func keepVacationWindow(t *testing.T) {
	t.Helper()
	prev, _ := dayWindowSize(calendar.WeekdayVacation)
	t.Cleanup(func() { setDayWindowSize(calendar.WeekdayVacation, prev) })
}

func installDriftingVacation(t *testing.T, today time.Time) {
	t.Helper()
	dates := daysOfType(currentCalendar(), today, calendar.WeekdayVacation, 20)
	for i, d := range dates {
		drift := float64(len(dates)-1-i) * 15
		for m := range 6 {
			archiveSegmentSample(t, d.Add(8*time.Hour+time.Duration(m)*time.Minute), 60+drift)
		}
	}
	if _, err := database.DB.Exec(`INSERT INTO segment_travel_time_profiles
		(route_id, direction_id, from_stop_id, to_stop_id, day_type, bucket_start_min, sample_count, median_sec, p75_sec, updated_at)
		VALUES (7, 0, 1, 2, 'weekday', 480, 10, 60, 70, 0)`); err != nil {
		t.Fatalf("term profile: %v", err)
	}
}

func TestVacationWindowTuningFollowsDriftingTravelTimes(t *testing.T) {
	withTestDB(t)
	withTestArchive(t)
	withSeedCalendar(t)
	keepVacationWindow(t)
	today := clujAt("2026-10-31", 0)
	installDriftingVacation(t, today)

	result, err := evaluateVacationWindows(currentCalendar(), today)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if result.Days != tuningEvalDays || result.Samples != tuningEvalDays*6 {
		t.Fatalf("evaluated %d days and %d samples", result.Days, result.Samples)
	}
	s := result.Scores
	if !(s["3"] < s["5"] && s["5"] < s["7"] && s["7"] < s["10"] && s["10"] < result.Baseline) {
		t.Fatalf("scores %v, baseline %v: shorter windows should follow the drift better", s, result.Baseline)
	}

	tuneVacationWindow(clujAt("2026-10-31", 4))
	if n, _ := dayWindowSize(calendar.WeekdayVacation); n != 3 {
		t.Fatalf("window %d, want 3", n)
	}
	saved, ok, err := savedWindowTuning()
	if err != nil || !ok || saved.Window != 3 || !saved.Changed {
		t.Fatalf("saved %+v (%t, %v)", saved, ok, err)
	}

	setDayWindowSize(calendar.WeekdayVacation, 5)
	LoadWindowTuning()
	if n, _ := dayWindowSize(calendar.WeekdayVacation); n != 3 {
		t.Fatalf("a restart forgot the tuned window: %d", n)
	}
}

func TestVacationWindowTuningWaitsForASchoolHoliday(t *testing.T) {
	withTestDB(t)
	withTestArchive(t)
	withSeedCalendar(t)
	keepVacationWindow(t)
	installDriftingVacation(t, clujAt("2026-10-31", 0))
	tuneVacationWindow(clujAt("2026-10-21", 4))
	if _, ok, _ := savedWindowTuning(); ok {
		t.Fatal("tuned after a school-term day")
	}
}

func TestPickVacationWindow(t *testing.T) {
	scores := func(three, five float64) windowTuning {
		return windowTuning{Days: tuningMinDays, Scores: map[string]float64{"3": three, "5": five, "7": 100, "10": 100}}
	}
	if n, changed := pickVacationWindow(scores(40, 50), 5); n != 3 || !changed {
		t.Errorf("clear gain: %d %t", n, changed)
	}
	if n, changed := pickVacationWindow(scores(49.5, 50), 5); n != 5 || changed {
		t.Errorf("1%% gain switched to %d", n)
	}
	few := scores(10, 50)
	few.Days = tuningMinDays - 1
	if n, changed := pickVacationWindow(few, 5); n != 5 || changed {
		t.Errorf("too few days switched to %d", n)
	}
}

func TestReplayComparesWithTheOldWindow(t *testing.T) {
	withTestDB(t)
	withTestArchive(t)
	withSeedCalendar(t)
	for d := clujAt("2026-09-01", 0); d.Before(clujAt("2026-09-26", 0)); d = d.AddDate(0, 0, 1) {
		duration := 60.0
		if currentCalendar().ProfileDay(d) == calendar.WeekdayVacation {
			duration = 200
		}
		for m := range 6 {
			archiveSegmentSample(t, d.Add(8*time.Hour+time.Duration(m)*time.Minute), duration)
		}
	}
	var out strings.Builder
	if err := RunReplay(clujAt("2026-09-07", 0), clujAt("2026-09-11", 0), &out); err != nil {
		t.Fatalf("replay: %v", err)
	}
	text := out.String()
	for _, want := range []string{"Calendar windows", "Mon 7 Sep", "Fri 11 Sep", "4 days"} {
		if !strings.Contains(text, want) {
			t.Fatalf("replay output misses %q:\n%s", want, text)
		}
	}
	if !strings.Contains(text, "0 s") || !strings.Contains(text, "-100%") {
		t.Fatalf("the calendar should beat the old window on the first school days:\n%s", text)
	}
}

func TestTimetableAccuracyByKindOfDay(t *testing.T) {
	withTestDB(t)
	withSeedCalendar(t)
	for _, ago := range []int{1, 2} {
		date := database.StatsDaysAgo(ago)
		for _, key := range []string{"ours|0", "ctp|2"} {
			if _, err := database.DB.Exec(`INSERT INTO stats_daily (date, metric, key, count) VALUES (?, 'timetable_error', ?, 4)`, date, key); err != nil {
				t.Fatalf("stat: %v", err)
			}
		}
	}
	a, err := adminAccuracySection(time.Now())
	if err != nil {
		t.Fatalf("accuracy: %v", err)
	}
	if len(a.Timetable.Daily) != adminTrendDays {
		t.Fatalf("%d daily rows", len(a.Timetable.Daily))
	}
	want := 2
	if today := a.Timetable.Daily[adminTrendDays-1]; today.Ours.N+today.CTP.N > 0 {
		want++
	}
	days := 0
	for _, k := range a.Timetable.Kinds {
		days += k.Days
	}
	if days != want {
		t.Fatalf("%d days across kinds, want %d", days, want)
	}
	yesterday := a.Timetable.Daily[adminTrendDays-2]
	at, _ := time.ParseInLocation("2006-01-02", yesterday.Date, database.StatsLocation())
	if yesterday.Kind != profileDay(at) || yesterday.Ours.N != 4 || yesterday.Ours.Within1 != 1 || yesterday.CTP.Within1 != 0 {
		t.Fatalf("yesterday %+v", yesterday)
	}
}
