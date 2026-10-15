package handlers

import (
	"conexiuni-cluj/calendar"
	"conexiuni-cluj/database"
	"maps"
	"math"
	"slices"
	"testing"
	"time"
)

func withEstimatedYear(t *testing.T) *calendar.Calendar {
	t.Helper()
	prev := activeCalendar.Load()
	cal := calendar.New(append(calendar.Seed(), calendar.Estimate(calendar.Seed()[0])), nil)
	setCalendar(cal)
	t.Cleanup(func() {
		activeCalendar.Store(prev)
		calendarGeneration.Add(1)
	})
	return cal
}

func keepSeedFills(t *testing.T) {
	t.Helper()
	seedFills.Lock()
	prev := maps.Clone(seedFills.fills)
	seedFills.Unlock()
	t.Cleanup(func() {
		seedFills.Lock()
		seedFills.fills = prev
		seedFills.Unlock()
		calendarGeneration.Add(1)
	})
}

func dates(days []time.Time) []string {
	out := make([]string, len(days))
	for i, d := range days {
		out[i] = d.Format("2006-01-02")
	}
	return out
}

func weekdaysOf(cal *calendar.Calendar, from, to string) []time.Time {
	var out []time.Time
	for d := clujAt(from, 0); !d.After(clujAt(to, 0)); d = d.AddDate(0, 0, 1) {
		if cal.ProfileDay(d) == calendar.Weekday {
			out = append(out, d)
		}
	}
	return out
}

func archiveDays(t *testing.T, days []time.Time, duration float64) {
	t.Helper()
	for _, d := range days {
		for m := range 6 {
			archiveSegmentSample(t, d.Add(8*time.Hour+time.Duration(m)*time.Minute), duration)
		}
	}
}

func TestRunsSpanShortBreaksButNotLongOnes(t *testing.T) {
	cal := withEstimatedYear(t)
	cases := []struct {
		name, dayType, today string
		n, wantLen           int
		wantOldest           string
	}{
		{"autumn break", calendar.Weekday, "2026-11-03", 100, 37, "2026-09-07"},
		{"summer", calendar.Weekday, "2027-09-08", 100, 3, "2027-09-06"},
		{"winter", calendar.Weekday, "2027-01-12", 100, 2, "2027-01-11"},
		{"in the summer holiday", calendar.Weekday, "2027-07-15", 100, 0, ""},
		{"saturdays", calendar.Saturday, "2027-07-17", 2, 2, "2027-07-10"},
	}
	for _, tc := range cases {
		run := dayRun(cal, tc.dayType, clujAt(tc.today, 0), tc.n)
		if len(run) != tc.wantLen {
			t.Errorf("%s: run of %d days %v, want %d", tc.name, len(run), dates(run), tc.wantLen)
			continue
		}
		if len(run) > 0 && run[len(run)-1].Format("2006-01-02") != tc.wantOldest {
			t.Errorf("%s: run starts %s, want %s", tc.name, run[len(run)-1].Format("2006-01-02"), tc.wantOldest)
		}
	}
}

func TestWindowFillsTheFirstSchoolDaysFromLastYear(t *testing.T) {
	cal := withEstimatedYear(t)
	today := clujAt("2027-09-08", 0)
	always := func([]time.Time) bool { return true }
	never := func([]time.Time) bool { return false }

	w := planDayWindow(cal, calendar.Weekday, today, 10, seedLastYear, always)
	want := []string{"2026-09-07", "2026-09-08", "2026-09-09", "2026-09-10", "2026-09-11", "2026-09-14", "2026-09-15"}
	if w.fill != seedLastYear || w.runDays != 3 || !slices.Equal(dates(w.lastYear), want) {
		t.Fatalf("fill %s, %d run days, last year %v", w.fill, w.runDays, dates(w.lastYear))
	}
	wantRanges := []windowRange{
		{clujAt("2027-09-06", 0).Unix(), math.MaxInt64},
		{clujAt("2026-09-07", 0).Unix(), clujAt("2026-09-16", 0).Unix()},
	}
	if !slices.Equal(w.ranges, wantRanges) {
		t.Fatalf("ranges %v, want %v", w.ranges, wantRanges)
	}

	for name, covered := range map[string]func([]time.Time) bool{"no archive": never, "previous chosen": always} {
		fill := seedLastYear
		if name == "previous chosen" {
			fill = seedPrevious
		}
		w := planDayWindow(cal, calendar.Weekday, today, 10, fill, covered)
		if w.fill != seedPrevious || len(w.lastYear) != 0 || !slices.Equal(w.ranges, []windowRange{{clujAt("2027-06-10", 0).Unix(), math.MaxInt64}}) {
			t.Errorf("%s: fill %s, ranges %v; want the last 10 days back to 10 Jun", name, w.fill, w.ranges)
		}
	}

	if w := planDayWindow(cal, calendar.Weekday, clujAt("2027-09-20", 0), 10, seedLastYear, always); w.fill != seedPrevious || len(w.ranges) != 1 {
		t.Errorf("a full run still took last year's days: %+v", w)
	}

	empty := planDayWindow(cal, calendar.Weekday, clujAt("2027-08-20", 0), 10, seedLastYear, always)
	if len(empty.ranges) != 1 || empty.ranges[0].to == math.MaxInt64 || len(empty.lastYear) != 10 {
		t.Errorf("an empty run should learn only from last year: %+v", empty)
	}
}

func TestLastYearNeedsHalfItsDaysInTheArchive(t *testing.T) {
	withTestArchive(t)
	cal := withEstimatedYear(t)
	keepSeedFills(t)
	setSeedFill(calendar.Weekday, seedLastYear)
	now := clujAt("2027-09-08", 12)
	lastYear := weekdaysOf(cal, "2026-09-07", "2026-09-15")

	archiveDays(t, lastYear[:3], 60)
	if w := currentDayWindow(calendar.Weekday, now); w.fill != seedPrevious {
		t.Fatalf("3 of 7 days archived, still filled from last year")
	}
	archiveDays(t, lastYear[3:4], 60)
	calendarGeneration.Add(1)
	if w := currentDayWindow(calendar.Weekday, now); w.fill != seedLastYear || len(w.lastYear) != 7 {
		t.Fatalf("4 of 7 days archived: fill %s with %d days", w.fill, len(w.lastYear))
	}
}

func TestWindowContainsAndSQLAgree(t *testing.T) {
	withTestDB(t)
	cal := withEstimatedYear(t)
	w := planDayWindow(cal, calendar.Weekday, clujAt("2027-09-08", 0), 10, seedLastYear, func([]time.Time) bool { return true })
	run, lastFrom, lastTo := clujAt("2027-09-06", 0), clujAt("2026-09-07", 0), clujAt("2026-09-16", 0)
	cases := map[time.Time]bool{
		lastFrom.Add(-time.Second): false,
		lastFrom:                   true,
		lastTo.Add(-time.Second):   true,
		lastTo:                     false,
		run.Add(-time.Second):      false,
		run:                        true,
		run.AddDate(0, 0, 2):       true,
	}
	inside := 0
	for at, want := range cases {
		if got := w.contains(at.Unix()); got != want {
			t.Errorf("contains(%s) = %t, want %t", at, got, want)
		}
		if want {
			inside++
		}
		insertSample(t, at, calendar.Weekday)
	}
	where, args := w.sql("observed_at")
	var n int
	if err := database.DB.QueryRow(`SELECT COUNT(*) FROM segment_travel_time_samples WHERE `+where, args...).Scan(&n); err != nil {
		t.Fatalf("query: %v", err)
	}
	if n != inside {
		t.Fatalf("the SQL fragment matched %d rows, contains matched %d", n, inside)
	}
}

func TestPruneKeepsLastYearsDays(t *testing.T) {
	withTestDB(t)
	withTestArchive(t)
	cal := withEstimatedYear(t)
	keepSeedFills(t)
	archiveDays(t, weekdaysOf(cal, "2026-09-07", "2026-09-15"), 60)
	setSeedFill(calendar.Weekday, seedLastYear)
	for _, date := range []string{"2026-09-08", "2026-09-21", "2027-06-15", "2027-09-07"} {
		insertSample(t, clujAt(date, 8), calendar.Weekday)
	}

	if _, err := pruneByDayWindow("segment_travel_time_samples", "observed_at", segmentSampleRetention, clujAt("2027-09-08", 12)); err != nil {
		t.Fatalf("prune: %v", err)
	}
	got := sampleDays(t)
	for _, kept := range []string{"2026-09-08", "2027-09-07"} {
		if _, ok := got[kept]; !ok {
			t.Errorf("%s was pruned", kept)
		}
	}
	for _, gone := range []string{"2026-09-21", "2027-06-15"} {
		if _, ok := got[gone]; ok {
			t.Errorf("%s was kept", gone)
		}
	}
}

func TestRestoreBringsBackLastYearsDaysOnce(t *testing.T) {
	withTestDB(t)
	withTestArchive(t)
	cal := withEstimatedYear(t)
	keepSeedFills(t)
	archiveDays(t, weekdaysOf(cal, "2026-09-07", "2026-09-16"), 60)
	archiveDays(t, []time.Time{clujAt("2026-09-12", 0)}, 60)
	setSeedFill(calendar.Weekday, seedLastYear)
	for m := range 6 {
		insertSample(t, clujAt("2026-09-10", 8).Add(time.Duration(m)*time.Minute), calendar.Weekday)
	}
	insertSample(t, clujAt("2027-09-07", 8), calendar.Weekday)
	now := clujAt("2027-09-08", 12)

	restored, err := restoreWindowsFromArchive(now)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	if restored != 6*6 {
		t.Fatalf("restored %d rows, want the 6 missing days of 6", restored)
	}
	if again, err := restoreWindowsFromArchive(now); err != nil || again != 0 {
		t.Fatalf("a second restore brought back %d (%v)", again, err)
	}
	perDay := map[string]int{}
	rows, err := database.DB.Query(`SELECT observed_at, day_type FROM segment_travel_time_samples`)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var at int64
		var day string
		if err := rows.Scan(&at, &day); err != nil {
			t.Fatalf("scan: %v", err)
		}
		perDay[time.Unix(at, 0).In(database.StatsLocation()).Format("2006-01-02")+" "+day]++
	}
	for _, date := range []string{"2026-09-07", "2026-09-08", "2026-09-09", "2026-09-10", "2026-09-11", "2026-09-14", "2026-09-15"} {
		if n := perDay[date+" weekday"]; n != 6 {
			t.Errorf("%s has %d weekday rows, want 6", date, n)
		}
	}
	for _, date := range []string{"2026-09-12 saturday", "2026-09-16 weekday"} {
		if n := perDay[date]; n != 0 {
			t.Errorf("%s restored %d rows outside the window", date, n)
		}
	}
}

func TestScheduleDelayDropsLastYearAfterATimetableChange(t *testing.T) {
	for name, tc := range map[string]struct {
		changed   func(v *database.TimetableVersion)
		wantDelay bool
	}{
		"weekday departures moved": {func(v *database.TimetableVersion) { v.Weekdays = "moved" }, false},
		"only saturday changed":    {func(v *database.TimetableVersion) { v.Saturday = "moved" }, true},
	} {
		t.Run(name, func(t *testing.T) {
			installDelayRoute(t)
			withTestArchive(t)
			cal := withEstimatedYear(t)
			keepSeedFills(t)
			lastYear := weekdaysOf(cal, "2026-09-07", "2026-09-15")
			archiveDays(t, lastYear, 60)
			setSeedFill(calendar.Weekday, seedLastYear)
			before := database.TimetableVersion{RouteShortName: "T1", Weekdays: "w", Saturday: "s", Sunday: "u"}
			after := before
			tc.changed(&after)
			if _, err := database.ArchiveTimetables([]database.TimetableVersion{before}, clujAt("2026-08-01", 0)); err != nil {
				t.Fatalf("archive: %v", err)
			}
			if _, err := database.ArchiveTimetables([]database.TimetableVersion{after}, clujAt("2027-08-01", 0)); err != nil {
				t.Fatalf("archive: %v", err)
			}
			for _, d := range append(lastYear, clujAt("2027-09-06", 0), clujAt("2027-09-07", 0)) {
				recordRun(t, d.Add(8*time.Hour+3*time.Minute))
			}
			if _, err := recomputeScheduleDelays(database.StatsLocation(), clujAt("2027-09-08", 12)); err != nil {
				t.Fatalf("recompute: %v", err)
			}
			if _, ok := loadScheduleDelay(delayRouteID, 0, calendar.Weekday, 8*60); ok != tc.wantDelay {
				t.Fatalf("learned a delay: %t, want %t", ok, tc.wantDelay)
			}
		})
	}
}

func installSlowSeptembers(t *testing.T, cal *calendar.Calendar) {
	t.Helper()
	archiveDays(t, weekdaysOf(cal, "2026-09-07", "2026-10-16"), 200)
	archiveDays(t, weekdaysOf(cal, "2027-05-17", "2027-06-18"), 60)
	archiveDays(t, weekdaysOf(cal, "2027-09-06", "2027-09-16"), 200)
}

func TestSeedSwitchesToLastYearWhenItPredictsBetter(t *testing.T) {
	withTestDB(t)
	withTestArchive(t)
	cal := withEstimatedYear(t)
	keepSeedFills(t)
	installSlowSeptembers(t, cal)

	tuneSeedFill(clujAt("2027-09-17", 4))
	saved, ok, err := savedSeedTuning(calendar.Weekday)
	if err != nil || !ok {
		t.Fatalf("nothing saved (%v)", err)
	}
	if saved.Days != 9 || saved.Fill != seedLastYear || !saved.Changed || saved.LastYear >= saved.Previous {
		t.Fatalf("saved %+v; want last year chosen over 9 days", saved)
	}
	if seedFill(calendar.Weekday) != seedLastYear {
		t.Fatalf("the weekday fill is %s", seedFill(calendar.Weekday))
	}
	if w := currentDayWindow(calendar.Weekday, clujAt("2027-09-14", 12)); w.fill != seedLastYear || len(w.lastYear) != 3 {
		t.Fatalf("today's window: fill %s with %d days from last year", w.fill, len(w.lastYear))
	}

	seedFills.Lock()
	delete(seedFills.fills, calendar.Weekday)
	seedFills.Unlock()
	LoadWindowTuning()
	if seedFill(calendar.Weekday) != seedLastYear {
		t.Fatal("a restart forgot the chosen fill")
	}
}

func TestSeedStaysWithTooFewDays(t *testing.T) {
	withTestDB(t)
	withTestArchive(t)
	cal := withEstimatedYear(t)
	keepSeedFills(t)
	installSlowSeptembers(t, cal)

	tuneSeedFill(clujAt("2027-09-10", 4))
	saved, ok, _ := savedSeedTuning(calendar.Weekday)
	if !ok || saved.Days != 4 || saved.Fill != seedPrevious || saved.Changed {
		t.Fatalf("saved %+v (%t); want 4 days and no change", saved, ok)
	}
	if seedFill(calendar.Weekday) != seedPrevious {
		t.Fatalf("switched to %s", seedFill(calendar.Weekday))
	}

	tuneSeedFill(clujAt("2027-09-19", 4))
	if _, ok, _ := savedSeedTuning(calendar.Saturday); ok {
		t.Fatal("a Saturday deep in its run was evaluated")
	}
}

func TestPickSeedFill(t *testing.T) {
	result := func(previous, lastYear float64, days int) seedTuning {
		return seedTuning{Previous: previous, LastYear: lastYear, Days: days}
	}
	cases := []struct {
		name    string
		result  seedTuning
		current string
		want    string
	}{
		{"clearly better", result(100, 90, tuningMinDays), seedPrevious, seedLastYear},
		{"1% better", result(100, 99, tuningMinDays), seedPrevious, seedPrevious},
		{"too few days", result(100, 50, tuningMinDays-1), seedPrevious, seedPrevious},
		{"back to previous", result(90, 100, tuningMinDays), seedLastYear, seedPrevious},
		{"last year still holds", result(99, 100, tuningMinDays), seedLastYear, seedLastYear},
	}
	for _, tc := range cases {
		if got, changed := pickSeedFill(tc.result, tc.current); got != tc.want || changed != (tc.want != tc.current) {
			t.Errorf("%s: %s (changed %t), want %s", tc.name, got, changed, tc.want)
		}
	}
}
