package handlers

import (
	"bytes"
	"conexiuni-cluj/calendar"
	"conexiuni-cluj/database"
	"conexiuni-cluj/models"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
)

func (city *City) withSeedCalendar(t *testing.T) {
	t.Helper()
	prev := city.activeCalendar.Load()
	city.setCalendar(calendar.New(calendar.Seed(), nil))
	t.Cleanup(func() {
		city.activeCalendar.Store(prev)
		city.calendarGeneration.Add(1)
	})
}

func (city *City) clujAt(date string, hour int) time.Time {
	day, err := time.ParseInLocation("2006-01-02", date, city.Location)
	if err != nil {
		panic(err)
	}
	return day.Add(time.Duration(hour) * time.Hour)
}

func (city *City) insertSample(t *testing.T, observedAt time.Time, dayType string) {
	t.Helper()
	if _, err := city.DB.Exec(`INSERT INTO segment_travel_time_samples
		(route_id, direction_id, from_stop_id, to_stop_id, day_type, bucket_start_min, duration_sec, observed_at)
		VALUES (7, 0, 1, 2, ?, 480, 60, ?)`, dayType, observedAt.Unix()); err != nil {
		t.Fatalf("insert sample: %v", err)
	}
}

func (city *City) sampleDays(t *testing.T) map[string]string {
	t.Helper()
	rows, err := city.DB.Query(`SELECT observed_at, day_type FROM segment_travel_time_samples`)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer func() { _ = rows.Close() }()
	out := map[string]string{}
	for rows.Next() {
		var at int64
		var day string
		if err := rows.Scan(&at, &day); err != nil {
			t.Fatalf("scan: %v", err)
		}
		out[time.Unix(at, 0).In(city.Location).Format("2006-01-02")] = day
	}
	return out
}

func TestDayWindowsCountDaysOfTheirOwnType(t *testing.T) {
	city := newTestCity(t)
	city.withSeedCalendar(t)
	now := city.clujAt("2026-10-27", 12)
	cases := map[string]string{
		calendar.Weekday:         "2026-10-12",
		calendar.WeekdayVacation: "2026-09-02",
		calendar.Saturday:        "2026-10-17",
		calendar.Sunday:          "2026-10-18",
	}
	for day, want := range cases {
		since, _ := city.currentDayWindow(day, now).since()
		got := time.Unix(since, 0).In(city.Location).Format("2006-01-02")
		if got != want {
			t.Errorf("%s window starts %s, want %s", day, got, want)
		}
	}
	if got, want := city.currentDayWindow("bogus", now).earliest(), now.Add(-legacyDayWindow).Unix(); got != want {
		t.Errorf("unknown day type window %d, want 14 days back %d", got, want)
	}
}

func TestPruneKeepsTheLastDaysOfEachType(t *testing.T) {
	city := newTestCity(t)
	city.withSeedCalendar(t)
	now := city.clujAt("2026-10-27", 12)
	city.insertSample(t, city.clujAt("2026-08-28", 8), calendar.WeekdayVacation)
	city.insertSample(t, city.clujAt("2026-09-02", 8), calendar.WeekdayVacation)
	city.insertSample(t, city.clujAt("2026-09-08", 8), calendar.Weekday)
	city.insertSample(t, city.clujAt("2026-10-13", 8), calendar.Weekday)
	city.insertSample(t, city.clujAt("2026-09-01", 8), "bogus")
	city.insertSample(t, city.clujAt("2026-10-01", 8), "bogus")

	if _, err := city.pruneByDayWindow("segment_travel_time_samples", "observed_at", segmentSampleRetention, now); err != nil {
		t.Fatalf("prune: %v", err)
	}
	got := city.sampleDays(t)
	for _, kept := range []string{"2026-09-02", "2026-10-13", "2026-10-01"} {
		if _, ok := got[kept]; !ok {
			t.Errorf("%s was pruned", kept)
		}
	}
	for _, gone := range []string{"2026-08-28", "2026-09-08", "2026-09-01"} {
		if _, ok := got[gone]; ok {
			t.Errorf("%s was kept", gone)
		}
	}
}

func TestRelabelFollowsTheCalendar(t *testing.T) {
	city := newTestCity(t)
	city.withSeedCalendar(t)
	city.insertSample(t, city.clujAt("2026-09-03", 8), calendar.Weekday)
	city.insertSample(t, city.clujAt("2026-09-08", 8), calendar.Weekday)
	city.insertSample(t, city.clujAt("2026-12-01", 8), calendar.Weekday)
	if _, err := city.DB.Exec(`INSERT INTO turnaround_samples (route_id, direction_id, day_type, duration_sec, arrived_at, departed_at)
		VALUES (7, 0, 'weekday', 60, ?, ?)`, city.clujAt("2026-09-03", 8).Unix(), city.clujAt("2026-09-03", 8).Unix()+60); err != nil {
		t.Fatalf("insert stand: %v", err)
	}

	if _, err := city.relabelSamples(city.clujAt("2026-12-02", 12)); err != nil {
		t.Fatalf("relabel: %v", err)
	}
	want := map[string]string{"2026-09-03": calendar.WeekdayVacation, "2026-09-08": calendar.Weekday, "2026-12-01": calendar.Sunday}
	got := city.sampleDays(t)
	for date, day := range want {
		if got[date] != day {
			t.Errorf("%s labelled %q, want %q", date, got[date], day)
		}
	}
	var stand string
	if err := city.DB.QueryRow(`SELECT day_type FROM turnaround_samples`).Scan(&stand); err != nil || stand != calendar.WeekdayVacation {
		t.Errorf("stand labelled %q (%v), want weekday_vacation", stand, err)
	}
}

func TestRestoreBringsBackDaysTheMainDatabaseLost(t *testing.T) {
	city := newTestCity(t)
	city.withSeedCalendar(t)
	city.insertSample(t, city.clujAt("2026-10-26", 8), calendar.WeekdayVacation)
	for _, date := range []string{"2026-08-20", "2026-09-03", "2026-09-08"} {
		if _, err := city.Archive.Exec(`INSERT INTO segment_travel_time_samples
			(route_id, direction_id, from_stop_id, to_stop_id, day_type, bucket_start_min, duration_sec, observed_at)
			VALUES (7, 0, 1, 2, 'weekday', 480, 60, ?)`, city.clujAt(date, 8).Unix()); err != nil {
			t.Fatalf("archive sample: %v", err)
		}
	}

	restored, err := city.restoreWindowsFromArchive(city.clujAt("2026-10-27", 12))
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	got := city.sampleDays(t)
	if restored != 1 || got["2026-09-03"] != calendar.WeekdayVacation {
		t.Fatalf("restored %d, days %v; want only 3 Sep back, as a school-holiday weekday", restored, got)
	}
}

func TestSiblingProfilesBeatOtherDayTypes(t *testing.T) {
	pair := stopPair{FromStopID: 1, ToStopID: 2}
	rows := []segmentProfileRow{
		{pair: pair, dayType: calendar.Saturday, bucket: 480, sampleCount: 40, medianSec: 50},
		{pair: pair, dayType: calendar.Weekday, bucket: 480, sampleCount: 40, medianSec: 90},
	}
	got := selectSegmentProfiles(rows, calendar.WeekdayVacation, 480)[pair]
	if got.DurationSec != 90 {
		t.Fatalf("school-holiday weekday took %v s, want the term weekday's 90", got.DurationSec)
	}
	if want := 0.75 * 40.0 / 50.0; math.Abs(got.Confidence-want) > 1e-9 {
		t.Fatalf("confidence %v, want %v", got.Confidence, want)
	}
	rows = append(rows, segmentProfileRow{pair: pair, dayType: calendar.WeekdayVacation, bucket: -1, sampleCount: 6, medianSec: 70})
	if got := selectSegmentProfiles(rows, calendar.WeekdayVacation, 480)[pair]; got.DurationSec != 70 {
		t.Fatalf("own whole-day profile lost to the sibling: %v", got.DurationSec)
	}
}

func TestScheduleDelayFallsBackToTheSiblingDay(t *testing.T) {
	city := newTestCity(t)
	city.installDelayRoute(t)
	city.withSeedCalendar(t)
	for _, day := range []int{10, 11, 14, 15, 16, 17, 18, 21} {
		city.recordRun(t, weekdayAt(day, 8, 3))
	}
	if _, err := city.recomputeScheduleDelays(delayZone, weekdayAt(23, 12, 0)); err != nil {
		t.Fatalf("recompute: %v", err)
	}
	city.resetProfileSnapshots()
	if delay, ok := city.snapshotScheduleDelay(delayRouteID, 0, calendar.WeekdayVacation, 8*60); !ok || delay != 180 {
		t.Fatalf("school-holiday delay = %v (%t), want the term's 180", delay, ok)
	}
}

func TestScheduleDelayForgetsRunsBeforeATimetableChange(t *testing.T) {
	for name, tc := range map[string]struct {
		changed   func(v *database.TimetableVersion)
		wantDelay bool
	}{
		"weekday departures moved": {func(v *database.TimetableVersion) { v.Weekdays = "moved" }, false},
		"only saturday changed":    {func(v *database.TimetableVersion) { v.Saturday = "moved" }, true},
	} {
		t.Run(name, func(t *testing.T) {
			city := newTestCity(t)
			city.withoutArchive(func() { city.installDelayRoute(t) })
			city.withSeedCalendar(t)
			before := database.TimetableVersion{RouteShortName: "T1", Weekdays: "w", Saturday: "s", Sunday: "u"}
			after := before
			tc.changed(&after)
			if _, err := city.ArchiveTimetables([]database.TimetableVersion{before}, weekdayAt(1, 0, 0)); err != nil {
				t.Fatalf("archive: %v", err)
			}
			if _, err := city.ArchiveTimetables([]database.TimetableVersion{after}, weekdayAt(17, 23, 0)); err != nil {
				t.Fatalf("archive: %v", err)
			}
			for _, day := range []int{10, 11, 14, 15, 16, 17, 18, 21} {
				city.recordRun(t, weekdayAt(day, 8, 3))
			}
			if _, err := city.recomputeScheduleDelays(delayZone, weekdayAt(23, 12, 0)); err != nil {
				t.Fatalf("recompute: %v", err)
			}
			if _, ok := city.loadScheduleDelay(delayRouteID, 0, calendar.Weekday, 8*60); ok != tc.wantDelay {
				t.Fatalf("learned a delay: %t, want %t", ok, tc.wantDelay)
			}
		})
	}
}

func TestObservedRunsTellWhichTimetableRan(t *testing.T) {
	city := newTestCity(t)
	city.installDelayRoute(t)
	city.withSeedCalendar(t)
	var weekdays, saturday []models.TimetableEntry
	for m := 6 * 60; m <= 20*60; m += 20 {
		weekdays = append(weekdays, models.TimetableEntry{DepartureIn: fmt.Sprintf("%02d:%02d", m/60, m%60)})
	}
	for m := 6*60 + 10; m <= 20*60; m += 40 {
		saturday = append(saturday, models.TimetableEntry{DepartureIn: fmt.Sprintf("%02d:%02d", m/60, m%60)})
	}
	if err := city.storeTimetableInDB(&models.Timetable{RouteShortName: "T1", InStopName: "A", OutStopName: "B",
		Weekdays: models.DaySchedule{Entries: weekdays}, Saturday: models.DaySchedule{Entries: saturday}}); err != nil {
		t.Fatalf("timetable: %v", err)
	}
	day := city.clujAt("2026-09-23", 0)
	for _, e := range saturday {
		departed := day.Add(time.Duration(minutesOf(t, e.DepartureIn)[0]+1) * time.Minute)
		if _, err := city.DB.Exec(`INSERT INTO segment_travel_time_samples
			(route_id, direction_id, from_stop_id, to_stop_id, day_type, bucket_start_min, duration_sec, observed_at)
			VALUES (?, 0, 100, 101, 'weekday', 480, 120, ?)`, delayRouteID, departed.Unix()+120); err != nil {
			t.Fatalf("run: %v", err)
		}
	}

	fits, observed, err := city.observedScheduleFits(day)
	if err != nil {
		t.Fatalf("fits: %v", err)
	}
	if observed != len(saturday) || fits[0].Day != calendar.Saturday || fits[0].Fit != 1 {
		t.Fatalf("fits %+v over %d runs; want saturday at 1.0 over %d", fits, observed, len(saturday))
	}
	for _, f := range fits[1:] {
		if f.Fit != 0 {
			t.Errorf("%s fit %v, want 0", f.Day, f.Fit)
		}
	}
}

func TestAdminSavesASchoolYear(t *testing.T) {
	city := newTestCity(t)
	city.withSeedCalendar(t)
	relearned := 0
	prevRelearn := relearnAfterCalendarChange
	relearnAfterCalendarChange = func(*City, time.Time) { relearned++ }
	t.Cleanup(func() { relearnAfterCalendarChange = prevRelearn })

	app := fiber.New()
	RegisterAdminRoutes(app.Group("/api"), []*City{city}, "secret", false)
	put := func(y calendar.SchoolYear) *http.Response {
		body, _ := json.Marshal(y)
		req := httptest.NewRequest("PUT", "/api/admin/calendar", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{Name: adminSessionCookie, Value: newAdminSession("secret", time.Now())})
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		return resp
	}

	bad := calendar.Estimate(calendar.Seed()[0])
	bad.Start = "2027-04-01"
	if resp := put(bad); resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid year answered %d", resp.StatusCode)
	}
	next := calendar.Estimate(calendar.Seed()[0])
	resp := put(next)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("save answered %d", resp.StatusCode)
	}
	var view adminCalendarView
	if err := json.NewDecoder(resp.Body).Decode(&view); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !city.currentCalendar().HasYear(2027) || relearned != 1 {
		t.Fatalf("2027 known: %t, relearned %d times", city.currentCalendar().HasYear(2027), relearned)
	}
	statuses := map[int]string{}
	for _, y := range view.Years {
		statuses[y.Year] = y.Status
	}
	if statuses[2027] != "saved" || statuses[2028] != "estimate" {
		t.Fatalf("years %v, want 2027 saved and a 2028 estimate", statuses)
	}

	if err := city.LoadCalendar(); err != nil || !city.currentCalendar().HasYear(2027) {
		t.Fatalf("the saved year did not survive a reload: %v", err)
	}
}

func TestCalendarProblemAsksForTheNextYearFromAugust(t *testing.T) {
	city := newTestCity(t)
	city.withSeedCalendar(t)
	if p := city.calendarProblem(city.clujAt("2027-07-31", 12)); p != "" {
		t.Errorf("July 2027 asked: %q", p)
	}
	if p := city.calendarProblem(city.clujAt("2027-08-01", 12)); p != "Enter the 2027–2028 school year in the calendar" {
		t.Errorf("August 2027: %q", p)
	}
}

func TestNetworkScheduleDaysListHolidaysOnly(t *testing.T) {
	city := newTestCity(t)
	city.withSeedCalendar(t)
	days, notes := city.networkScheduleDays(city.clujAt("2026-10-15", 12))
	if n := notes["2026-12-01"]; n.Reason != dayNoteHoliday || n.Holiday != "national_day" {
		t.Fatalf("1 Dec note %+v", n)
	}
	if days["2026-11-30"] != calendar.Sunday || days["2026-12-01"] != calendar.Sunday {
		t.Fatalf("30 Nov and 1 Dec: %q %q, want sunday", days["2026-11-30"], days["2026-12-01"])
	}
	if _, ok := days["2026-10-26"]; ok {
		t.Fatal("a school-holiday weekday changed the timetable")
	}
	if _, ok := days["2027-05-03"]; !ok {
		t.Fatal("next year's Easter Monday is missing")
	}
}

func TestRebuildLearnsEachDayTypeFromItsOwnWindow(t *testing.T) {
	city := newTestCity(t)
	city.withSeedCalendar(t)
	for _, date := range []string{"2026-09-02", "2026-09-03", "2026-09-04", "2026-10-26", "2026-10-27"} {
		city.insertSample(t, city.clujAt(date, 8), calendar.WeekdayVacation)
	}
	city.insertSample(t, city.clujAt("2026-09-08", 8), calendar.Weekday)
	for _, date := range []string{"2026-10-19", "2026-10-20", "2026-10-21", "2026-10-22", "2026-10-23"} {
		city.insertSample(t, city.clujAt(date, 8), calendar.Weekday)
	}

	if _, err := city.rebuildSegmentProfiles(city.clujAt("2026-10-27", 12)); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	counts := map[string]int{}
	rows, err := city.DB.Query(`SELECT day_type, sample_count FROM segment_travel_time_profiles WHERE bucket_start_min = 480`)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var day string
		var n int
		if err := rows.Scan(&day, &n); err != nil {
			t.Fatalf("scan: %v", err)
		}
		counts[day] = n
	}
	if counts[calendar.WeekdayVacation] != 5 || counts[calendar.Weekday] != 5 {
		t.Fatalf("profiles from %v samples; want 5 school-holiday days and the 5 recent term days", counts)
	}
}

func (city *City) installSaturdayRuns(t *testing.T, date string) {
	t.Helper()
	city.installDelayRoute(t)
	var weekdays, saturday []models.TimetableEntry
	for m := 6 * 60; m <= 20*60; m += 20 {
		weekdays = append(weekdays, models.TimetableEntry{DepartureIn: fmt.Sprintf("%02d:%02d", m/60, m%60)})
	}
	for m := 6*60 + 2; m <= 20*60; m += 5 {
		saturday = append(saturday, models.TimetableEntry{DepartureIn: fmt.Sprintf("%02d:%02d", m/60, m%60)})
	}
	if err := city.storeTimetableInDB(&models.Timetable{RouteShortName: "T1", InStopName: "A", OutStopName: "B",
		Weekdays: models.DaySchedule{Entries: weekdays}, Saturday: models.DaySchedule{Entries: saturday}}); err != nil {
		t.Fatalf("timetable: %v", err)
	}
	day := city.clujAt(date, 0)
	for _, e := range saturday {
		departed := day.Add(time.Duration(minutesOf(t, e.DepartureIn)[0]) * time.Minute)
		if _, err := city.DB.Exec(`INSERT INTO segment_travel_time_samples
			(route_id, direction_id, from_stop_id, to_stop_id, day_type, bucket_start_min, duration_sec, observed_at)
			VALUES (?, 0, 100, 101, 'weekday', 480, 120, ?)`, delayRouteID, departed.Unix()+120); err != nil {
			t.Fatalf("run: %v", err)
		}
	}
}

func TestNightlyCheckRecordsTheTimetableThatRan(t *testing.T) {
	city := newTestCity(t)
	city.installSaturdayRuns(t, "2026-09-23")
	city.withSeedCalendar(t)
	city.checkObservedSchedule(city.clujAt("2026-09-24", 4))
	o, ok, err := city.ScheduleOverrideOn("2026-09-23")
	if err != nil || !ok || o.Schedule != calendar.Saturday || o.Source != overrideObserved {
		t.Fatalf("override %+v (%t, %v), want saturday seen running", o, ok, err)
	}
	if got := city.scheduleDay(city.clujAt("2026-09-23", 8)); got != calendar.Saturday {
		t.Fatalf("23 Sep now runs %s", got)
	}
}

func TestNightlyCheckLeavesTheAdminsChoiceAlone(t *testing.T) {
	city := newTestCity(t)
	city.installSaturdayRuns(t, "2026-09-23")
	city.withSeedCalendar(t)
	if err := city.SaveScheduleOverride(database.ScheduleOverride{Date: "2026-09-23", Schedule: calendar.Weekday, Source: overrideByAdmin}); err != nil {
		t.Fatalf("save: %v", err)
	}
	city.checkObservedSchedule(city.clujAt("2026-09-24", 4))
	if o, _, _ := city.ScheduleOverrideOn("2026-09-23"); o.Source != overrideByAdmin || o.Schedule != calendar.Weekday {
		t.Fatalf("override became %+v", o)
	}
}

func TestAdminSetsTheTimetableForADay(t *testing.T) {
	city := newTestCity(t)
	city.withSeedCalendar(t)
	prevRelearn := relearnAfterCalendarChange
	relearnAfterCalendarChange = func(*City, time.Time) {}
	t.Cleanup(func() { relearnAfterCalendarChange = prevRelearn })

	app := fiber.New()
	RegisterAdminRoutes(app.Group("/api"), []*City{city}, "secret", false)
	cookie := &http.Cookie{Name: adminSessionCookie, Value: newAdminSession("secret", time.Now())}
	set := func(date, schedule string) (int, adminDayResponse) {
		body, _ := json.Marshal(adminDayRequest{Date: date, Schedule: schedule})
		req := httptest.NewRequest("PUT", "/api/admin/calendar/day", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(cookie)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		var out adminDayResponse
		_ = json.NewDecoder(resp.Body).Decode(&out)
		return resp.StatusCode, out
	}

	date := time.Now().In(city.Location).AddDate(0, 0, 3).Format("2006-01-02")
	if code, _ := set(date, "holiday"); code != http.StatusBadRequest {
		t.Fatalf("unknown timetable answered %d", code)
	}
	if code, _ := set("2001-01-01", calendar.Sunday); code != http.StatusBadRequest {
		t.Fatalf("a date years away answered %d", code)
	}
	code, out := set(date, calendar.Saturday)
	if code != http.StatusOK || out.Day.ScheduleDay != calendar.Saturday || out.Day.Override == nil || out.Day.Override.Source != overrideByAdmin {
		t.Fatalf("set saturday: %d %+v", code, out.Day)
	}
	day, _ := time.ParseInLocation("2006-01-02", date, city.Location)
	if city.scheduleDay(day) != calendar.Saturday {
		t.Fatalf("%s does not run the Saturday timetable", date)
	}
	if days, notes := city.networkScheduleDays(time.Now()); day.Weekday() != time.Saturday && (days[date] != calendar.Saturday || notes[date].Reason != dayNoteSet) {
		t.Fatalf("the network bundle says %q (%+v) for %s", days[date], notes[date], date)
	}

	req := httptest.NewRequest("GET", "/api/admin/calendar/day?date="+date, nil)
	req.AddCookie(cookie)
	resp, _ := app.Test(req)
	var got adminCalendarDay
	_ = json.NewDecoder(resp.Body).Decode(&got)
	if got.ScheduleDay != calendar.Saturday || got.Automatic != calendar.New(calendar.Seed(), nil).AutomaticScheduleDay(day) {
		t.Fatalf("day view %+v", got)
	}

	if code, out := set(date, ""); code != http.StatusOK || out.Day.Override != nil || out.Day.ScheduleDay != out.Day.Automatic {
		t.Fatalf("back to automatic: %d %+v", code, out.Day)
	}
}
