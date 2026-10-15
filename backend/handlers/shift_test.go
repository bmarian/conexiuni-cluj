package handlers

import (
	"conexiuni-cluj/calendar"
	"conexiuni-cluj/database"
	"strings"
	"testing"
	"time"
)

func archiveBucketSample(t *testing.T, at time.Time, bucket int, duration float64) {
	t.Helper()
	if _, err := database.Archive.Exec(`INSERT INTO segment_travel_time_samples
		(route_id, direction_id, from_stop_id, to_stop_id, day_type, bucket_start_min, duration_sec, observed_at)
		VALUES (7, 0, 1, 2, 'weekday', ?, ?, ?)`, bucket, duration, at.Unix()); err != nil {
		t.Fatalf("archive sample: %v", err)
	}
}

func TestShiftShowsAStepOnTheDate(t *testing.T) {
	withTestArchive(t)
	withSeedCalendar(t)
	cal := currentCalendar()
	date := clujAt("2026-09-28", 0)
	for d := clujAt("2026-09-07", 0); d.Before(clujAt("2026-10-17", 0)); d = d.AddDate(0, 0, 1) {
		if cal.ProfileDay(d) != calendar.Weekday {
			continue
		}
		morning := 60.0
		if !d.Before(date) {
			morning = 90
		}
		for m := range 6 {
			archiveBucketSample(t, d.Add(8*time.Hour+time.Duration(m)*time.Minute), 480, morning)
		}
		for m := range 8 {
			archiveBucketSample(t, d.Add(18*time.Hour+time.Duration(m)*time.Minute), 1080, 100)
		}
	}

	var out strings.Builder
	if err := RunShift(date, &out); err != nil {
		t.Fatalf("shift: %v", err)
	}
	lines := map[string]string{}
	for _, line := range strings.Split(out.String(), "\n") {
		if fields := strings.Fields(line); len(fields) >= 3 {
			lines[strings.Join(fields[:3], " ")] = line
		}
	}
	for day, want := range map[string][]string{
		"Mon 14 Sep": {"1.00", "1.00", "14"},
		"Fri 25 Sep": {"1.00", "1.00", "14"},
		"Mon 28 Sep": {"1.00", "1.50", "14", "<- 2026-09-28"},
		"Fri 9 Oct":  {"1.00", "1.50", "14"},
	} {
		line, ok := lines[day]
		if !ok {
			t.Fatalf("no row for %s:\n%s", day, out.String())
		}
		if got := strings.Fields(line)[3:]; strings.Join(got, " ") != strings.Join(want, " ") {
			t.Errorf("%s: %v, want %v", day, got, want)
		}
	}
	if _, ok := lines["Fri 11 Sep"]; ok {
		t.Errorf("printed more than 10 days before the date:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "Mon 12 Oct") || !strings.Contains(out.String(), "part of that baseline") {
		t.Errorf("missing the last row or the baseline note:\n%s", out.String())
	}
}
