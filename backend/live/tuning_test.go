package live

import (
	"conexiuni-cluj/calendar"
	"strings"
	"testing"
	"time"
)

func (eng *Engine) archiveSegmentSample(t *testing.T, at time.Time, duration float64) {
	t.Helper()
	if _, err := eng.Archive.Exec(`INSERT INTO segment_travel_time_samples
		(route_id, direction_id, from_stop_id, to_stop_id, day_type, bucket_start_min, duration_sec, observed_at)
		VALUES (7, 0, 1, 2, 'weekday', 480, ?, ?)`, duration, at.Unix()); err != nil {
		t.Fatalf("archive sample: %v", err)
	}
}

func (eng *Engine) keepVacationWindow(t *testing.T) {
	t.Helper()
	prev, _ := eng.dayWindowSize(calendar.WeekdayVacation)
	t.Cleanup(func() { eng.setDayWindowSize(calendar.WeekdayVacation, prev) })
}

func (eng *Engine) installDriftingVacation(t *testing.T, today time.Time) {
	t.Helper()
	dates := eng.daysOfType(eng.currentCalendar(), today, calendar.WeekdayVacation, 20)
	for i, d := range dates {
		drift := float64(len(dates)-1-i) * 15
		for m := range 6 {
			eng.archiveSegmentSample(t, d.Add(8*time.Hour+time.Duration(m)*time.Minute), 60+drift)
		}
	}
	if _, err := eng.DB.Exec(`INSERT INTO segment_travel_time_profiles
		(route_id, direction_id, from_stop_id, to_stop_id, day_type, bucket_start_min, sample_count, median_sec, p75_sec, updated_at)
		VALUES (7, 0, 1, 2, 'weekday', 480, 10, 60, 70, 0)`); err != nil {
		t.Fatalf("term profile: %v", err)
	}
}

func TestVacationWindowTuningFollowsDriftingTravelTimes(t *testing.T) {
	eng := newTestCity(t)
	eng.withSeedCalendar(t)
	eng.keepVacationWindow(t)
	today := eng.clujAt("2026-10-31", 0)
	eng.installDriftingVacation(t, today)

	result, err := eng.evaluateVacationWindows(eng.currentCalendar(), today)
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

	eng.tuneVacationWindow(eng.clujAt("2026-10-31", 4))
	if n, _ := eng.dayWindowSize(calendar.WeekdayVacation); n != 3 {
		t.Fatalf("window %d, want 3", n)
	}
	saved, ok, err := eng.savedWindowTuning()
	if err != nil || !ok || saved.Window != 3 || !saved.Changed {
		t.Fatalf("saved %+v (%t, %v)", saved, ok, err)
	}

	eng.setDayWindowSize(calendar.WeekdayVacation, 5)
	eng.LoadWindowTuning()
	if n, _ := eng.dayWindowSize(calendar.WeekdayVacation); n != 3 {
		t.Fatalf("a restart forgot the tuned window: %d", n)
	}
}

func TestVacationWindowTuningWaitsForASchoolHoliday(t *testing.T) {
	eng := newTestCity(t)
	eng.withSeedCalendar(t)
	eng.keepVacationWindow(t)
	eng.installDriftingVacation(t, eng.clujAt("2026-10-31", 0))
	eng.tuneVacationWindow(eng.clujAt("2026-10-21", 4))
	if _, ok, _ := eng.savedWindowTuning(); ok {
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
	eng := newTestCity(t)
	eng.withSeedCalendar(t)
	for d := eng.clujAt("2026-09-01", 0); d.Before(eng.clujAt("2026-09-26", 0)); d = d.AddDate(0, 0, 1) {
		duration := 60.0
		if eng.currentCalendar().ProfileDay(d) == calendar.WeekdayVacation {
			duration = 200
		}
		for m := range 6 {
			eng.archiveSegmentSample(t, d.Add(8*time.Hour+time.Duration(m)*time.Minute), duration)
		}
	}
	var out strings.Builder
	if err := eng.RunReplay(eng.clujAt("2026-09-07", 0), eng.clujAt("2026-09-11", 0), &out); err != nil {
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
