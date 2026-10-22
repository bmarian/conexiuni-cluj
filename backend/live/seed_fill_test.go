package live

import (
	"conexiuni-cluj/calendar"
	"conexiuni-cluj/database"
	"maps"
	"math"
	"slices"
	"testing"
	"time"
)

func (eng *Engine) withEstimatedYear(t *testing.T) *calendar.Calendar {
	t.Helper()
	prev := eng.activeCalendar.Load()
	cal := calendar.New(append(calendar.Seed(), calendar.Estimate(calendar.Seed()[0])), nil)
	eng.setCalendar(cal)
	t.Cleanup(func() {
		eng.activeCalendar.Store(prev)
		eng.calendarGeneration.Add(1)
	})
	return cal
}

func (eng *Engine) keepSeedFills(t *testing.T) {
	t.Helper()
	eng.seedFills.Lock()
	prev := maps.Clone(eng.seedFills.fills)
	eng.seedFills.Unlock()
	t.Cleanup(func() {
		eng.seedFills.Lock()
		eng.seedFills.fills = prev
		eng.seedFills.Unlock()
		eng.calendarGeneration.Add(1)
	})
}

func dates(days []time.Time) []string {
	out := make([]string, len(days))
	for i, d := range days {
		out[i] = d.Format("2006-01-02")
	}
	return out
}

func (eng *Engine) weekdaysOf(cal *calendar.Calendar, from, to string) []time.Time {
	var out []time.Time
	for d := eng.clujAt(from, 0); !d.After(eng.clujAt(to, 0)); d = d.AddDate(0, 0, 1) {
		if cal.ProfileDay(d) == calendar.Weekday {
			out = append(out, d)
		}
	}
	return out
}

func (eng *Engine) archiveDays(t *testing.T, days []time.Time, duration float64) {
	t.Helper()
	for _, d := range days {
		for m := range 6 {
			eng.archiveSegmentSample(t, d.Add(8*time.Hour+time.Duration(m)*time.Minute), duration)
		}
	}
}

func TestRunsSpanShortBreaksButNotLongOnes(t *testing.T) {
	eng := newTestCity(t)
	cal := eng.withEstimatedYear(t)
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
		run := dayRun(cal, tc.dayType, eng.clujAt(tc.today, 0), tc.n)
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
	eng := newTestCity(t)
	cal := eng.withEstimatedYear(t)
	today := eng.clujAt("2027-09-08", 0)
	always := func([]time.Time) bool { return true }
	never := func([]time.Time) bool { return false }

	w := eng.planDayWindow(cal, calendar.Weekday, today, 10, seedLastYear, always)
	want := []string{"2026-09-07", "2026-09-08", "2026-09-09", "2026-09-10", "2026-09-11", "2026-09-14", "2026-09-15"}
	if w.fill != seedLastYear || w.runDays != 3 || !slices.Equal(dates(w.lastYear), want) {
		t.Fatalf("fill %s, %d run days, last year %v", w.fill, w.runDays, dates(w.lastYear))
	}
	wantRanges := []windowRange{
		{eng.clujAt("2027-09-06", 0).Unix(), math.MaxInt64},
		{eng.clujAt("2026-09-07", 0).Unix(), eng.clujAt("2026-09-16", 0).Unix()},
	}
	if !slices.Equal(w.ranges, wantRanges) {
		t.Fatalf("ranges %v, want %v", w.ranges, wantRanges)
	}

	for name, covered := range map[string]func([]time.Time) bool{"no archive": never, "previous chosen": always} {
		fill := seedLastYear
		if name == "previous chosen" {
			fill = seedPrevious
		}
		w := eng.planDayWindow(cal, calendar.Weekday, today, 10, fill, covered)
		if w.fill != seedPrevious || len(w.lastYear) != 0 || !slices.Equal(w.ranges, []windowRange{{eng.clujAt("2027-06-10", 0).Unix(), math.MaxInt64}}) {
			t.Errorf("%s: fill %s, ranges %v; want the last 10 days back to 10 Jun", name, w.fill, w.ranges)
		}
	}

	if w := eng.planDayWindow(cal, calendar.Weekday, eng.clujAt("2027-09-20", 0), 10, seedLastYear, always); w.fill != seedPrevious || len(w.ranges) != 1 {
		t.Errorf("a full run still took last year's days: %+v", w)
	}

	empty := eng.planDayWindow(cal, calendar.Weekday, eng.clujAt("2027-08-20", 0), 10, seedLastYear, always)
	if len(empty.ranges) != 1 || empty.ranges[0].to == math.MaxInt64 || len(empty.lastYear) != 10 {
		t.Errorf("an empty run should learn only from last year: %+v", empty)
	}
}

func TestLastYearNeedsHalfItsDaysInTheArchive(t *testing.T) {
	eng := newTestCity(t)
	cal := eng.withEstimatedYear(t)
	eng.keepSeedFills(t)
	eng.setSeedFill(calendar.Weekday, seedLastYear)
	now := eng.clujAt("2027-09-08", 12)
	lastYear := eng.weekdaysOf(cal, "2026-09-07", "2026-09-15")

	eng.archiveDays(t, lastYear[:3], 60)
	if w := eng.currentDayWindow(calendar.Weekday, now); w.fill != seedPrevious {
		t.Fatalf("3 of 7 days archived, still filled from last year")
	}
	eng.archiveDays(t, lastYear[3:4], 60)
	eng.calendarGeneration.Add(1)
	if w := eng.currentDayWindow(calendar.Weekday, now); w.fill != seedLastYear || len(w.lastYear) != 7 {
		t.Fatalf("4 of 7 days archived: fill %s with %d days", w.fill, len(w.lastYear))
	}
}

func TestWindowContainsAndSQLAgree(t *testing.T) {
	eng := newTestCity(t)
	cal := eng.withEstimatedYear(t)
	w := eng.planDayWindow(cal, calendar.Weekday, eng.clujAt("2027-09-08", 0), 10, seedLastYear, func([]time.Time) bool { return true })
	run, lastFrom, lastTo := eng.clujAt("2027-09-06", 0), eng.clujAt("2026-09-07", 0), eng.clujAt("2026-09-16", 0)
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
		eng.insertSample(t, at, calendar.Weekday)
	}
	where, args := w.sql("observed_at")
	var n int
	if err := eng.DB.QueryRow(`SELECT COUNT(*) FROM segment_travel_time_samples WHERE `+where, args...).Scan(&n); err != nil {
		t.Fatalf("query: %v", err)
	}
	if n != inside {
		t.Fatalf("the SQL fragment matched %d rows, contains matched %d", n, inside)
	}
}

func TestPruneKeepsLastYearsDays(t *testing.T) {
	eng := newTestCity(t)
	cal := eng.withEstimatedYear(t)
	eng.keepSeedFills(t)
	eng.archiveDays(t, eng.weekdaysOf(cal, "2026-09-07", "2026-09-15"), 60)
	eng.setSeedFill(calendar.Weekday, seedLastYear)
	for _, date := range []string{"2026-09-08", "2026-09-21", "2027-06-15", "2027-09-07"} {
		eng.insertSample(t, eng.clujAt(date, 8), calendar.Weekday)
	}

	if _, err := eng.pruneByDayWindow("segment_travel_time_samples", "observed_at", SampleRetention, eng.clujAt("2027-09-08", 12)); err != nil {
		t.Fatalf("prune: %v", err)
	}
	got := eng.sampleDays(t)
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
	eng := newTestCity(t)
	cal := eng.withEstimatedYear(t)
	eng.keepSeedFills(t)
	eng.archiveDays(t, eng.weekdaysOf(cal, "2026-09-07", "2026-09-16"), 60)
	eng.archiveDays(t, []time.Time{eng.clujAt("2026-09-12", 0)}, 60)
	eng.setSeedFill(calendar.Weekday, seedLastYear)
	for m := range 6 {
		eng.insertSample(t, eng.clujAt("2026-09-10", 8).Add(time.Duration(m)*time.Minute), calendar.Weekday)
	}
	eng.insertSample(t, eng.clujAt("2027-09-07", 8), calendar.Weekday)
	now := eng.clujAt("2027-09-08", 12)

	restored, err := eng.restoreWindowsFromArchive(now)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	if restored != 6*6 {
		t.Fatalf("restored %d rows, want the 6 missing days of 6", restored)
	}
	if again, err := eng.restoreWindowsFromArchive(now); err != nil || again != 0 {
		t.Fatalf("a second restore brought back %d (%v)", again, err)
	}
	perDay := map[string]int{}
	rows, err := eng.DB.Query(`SELECT observed_at, day_type FROM segment_travel_time_samples`)
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
		perDay[time.Unix(at, 0).In(eng.Location).Format("2006-01-02")+" "+day]++
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
			eng := newTestCity(t)
			eng.withoutArchive(func() { eng.installDelayRoute(t) })
			cal := eng.withEstimatedYear(t)
			eng.keepSeedFills(t)
			lastYear := eng.weekdaysOf(cal, "2026-09-07", "2026-09-15")
			eng.archiveDays(t, lastYear, 60)
			eng.setSeedFill(calendar.Weekday, seedLastYear)
			before := database.TimetableVersion{RouteShortName: "T1", Weekdays: "w", Saturday: "s", Sunday: "u"}
			after := before
			tc.changed(&after)
			if _, err := eng.ArchiveTimetables([]database.TimetableVersion{before}, eng.clujAt("2026-08-01", 0)); err != nil {
				t.Fatalf("archive: %v", err)
			}
			if _, err := eng.ArchiveTimetables([]database.TimetableVersion{after}, eng.clujAt("2027-08-01", 0)); err != nil {
				t.Fatalf("archive: %v", err)
			}
			for _, d := range append(lastYear, eng.clujAt("2027-09-06", 0), eng.clujAt("2027-09-07", 0)) {
				eng.recordRun(t, d.Add(8*time.Hour+3*time.Minute))
			}
			if _, err := eng.recomputeScheduleDelays(eng.Location, eng.clujAt("2027-09-08", 12)); err != nil {
				t.Fatalf("recompute: %v", err)
			}
			if _, ok := eng.loadScheduleDelay(delayRouteID, 0, calendar.Weekday, 8*60); ok != tc.wantDelay {
				t.Fatalf("learned a delay: %t, want %t", ok, tc.wantDelay)
			}
		})
	}
}

func (eng *Engine) installSlowSeptembers(t *testing.T, cal *calendar.Calendar) {
	t.Helper()
	eng.archiveDays(t, eng.weekdaysOf(cal, "2026-09-07", "2026-10-16"), 200)
	eng.archiveDays(t, eng.weekdaysOf(cal, "2027-05-17", "2027-06-18"), 60)
	eng.archiveDays(t, eng.weekdaysOf(cal, "2027-09-06", "2027-09-16"), 200)
}

func TestSeedSwitchesToLastYearWhenItPredictsBetter(t *testing.T) {
	eng := newTestCity(t)
	cal := eng.withEstimatedYear(t)
	eng.keepSeedFills(t)
	eng.installSlowSeptembers(t, cal)

	eng.tuneSeedFill(eng.clujAt("2027-09-17", 4))
	saved, ok, err := eng.savedSeedTuning(calendar.Weekday)
	if err != nil || !ok {
		t.Fatalf("nothing saved (%v)", err)
	}
	if saved.Days != 9 || saved.Fill != seedLastYear || !saved.Changed || saved.LastYear >= saved.Previous {
		t.Fatalf("saved %+v; want last year chosen over 9 days", saved)
	}
	if eng.seedFill(calendar.Weekday) != seedLastYear {
		t.Fatalf("the weekday fill is %s", eng.seedFill(calendar.Weekday))
	}
	if w := eng.currentDayWindow(calendar.Weekday, eng.clujAt("2027-09-14", 12)); w.fill != seedLastYear || len(w.lastYear) != 3 {
		t.Fatalf("today's window: fill %s with %d days from last year", w.fill, len(w.lastYear))
	}

	eng.seedFills.Lock()
	delete(eng.seedFills.fills, calendar.Weekday)
	eng.seedFills.Unlock()
	eng.LoadWindowTuning()
	if eng.seedFill(calendar.Weekday) != seedLastYear {
		t.Fatal("a restart forgot the chosen fill")
	}
}

func TestSeedStaysWithTooFewDays(t *testing.T) {
	eng := newTestCity(t)
	cal := eng.withEstimatedYear(t)
	eng.keepSeedFills(t)
	eng.installSlowSeptembers(t, cal)

	eng.tuneSeedFill(eng.clujAt("2027-09-10", 4))
	saved, ok, _ := eng.savedSeedTuning(calendar.Weekday)
	if !ok || saved.Days != 4 || saved.Fill != seedPrevious || saved.Changed {
		t.Fatalf("saved %+v (%t); want 4 days and no change", saved, ok)
	}
	if eng.seedFill(calendar.Weekday) != seedPrevious {
		t.Fatalf("switched to %s", eng.seedFill(calendar.Weekday))
	}

	eng.tuneSeedFill(eng.clujAt("2027-09-19", 4))
	if _, ok, _ := eng.savedSeedTuning(calendar.Saturday); ok {
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
