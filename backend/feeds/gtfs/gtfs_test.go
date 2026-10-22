package gtfs

import (
	"archive/zip"
	"bytes"
	"conexiuni-cluj/calendar"
	"conexiuni-cluj/models"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// One line A-B-C-D: three full trips, a short run, a run from B, an on-time and a late run via X; two trips back.
func testFeed(t *testing.T) *feed {
	return testFeedWith(t, nil)
}

func testFeedWith(t *testing.T, replace map[string]string) *feed {
	t.Helper()
	files := map[string]string{
		"routes.txt": "route_id,route_short_name,route_long_name,route_type,route_color\nL1,7,A - D,3,FF0000\n",
		"stops.txt": "stop_id,stop_name,stop_lat,stop_lon\n" +
			"A,Alpha,45.0,24.0\nB,Beta,45.01,24.0\nC,Gamma,45.02,24.0\nD,Delta,45.03,24.0\nX,Extra,45.015,24.0\n",
		"calendar.txt": "service_id,monday,tuesday,wednesday,thursday,friday,saturday,sunday,start_date,end_date\n" +
			"WK,1,1,1,1,1,0,0,20260101,20271231\nSA,0,0,0,0,0,1,0,20260101,20271231\n",
		"trips.txt": "route_id,service_id,trip_id,direction_id,shape_id,trip_headsign\n" +
			"L1,WK,t1,0,s0,Delta\nL1,WK,t2,0,s0,Delta\nL1,SA,t3,0,s0,Delta\nL1,WK,short,0,s0,Gamma\nL1,WK,late,0,s0,Delta\nL1,WK,extra,0,s0,Delta\nL1,WK,detour,0,s0,Delta\n" +
			"L1,WK,b1,1,s1,Alpha\nL1,WK,b2,1,s1,Alpha\n",
		"stop_times.txt": "trip_id,arrival_time,departure_time,stop_id,stop_sequence\n" +
			"t1,06:00:00,06:00:00,A,1\nt1,06:05:00,06:05:00,B,2\nt1,06:12:00,06:12:00,C,3\nt1,06:20:00,06:20:00,D,4\n" +
			"t2,07:00:00,07:00:00,A,1\nt2,07:06:00,07:06:00,B,2\nt2,07:13:00,07:13:00,C,3\nt2,07:21:00,07:21:00,D,4\n" +
			"t3,09:00:00,09:00:00,A,1\nt3,09:05:00,09:05:00,B,2\nt3,09:12:00,09:12:00,C,3\nt3,09:20:00,09:20:00,D,4\n" +
			"short,08:00:00,08:00:00,A,1\nshort,08:05:00,08:05:00,B,2\nshort,08:12:00,08:12:00,C,3\n" +
			"late,22:30:00,22:30:00,B,1\nlate,22:37:00,22:37:00,C,2\nlate,22:45:00,22:45:00,D,3\n" +
			"extra,10:00:00,10:00:00,A,1\nextra,10:05:00,10:05:00,B,2\nextra,10:08:00,10:08:00,X,3\nextra,10:12:00,10:12:00,C,4\nextra,10:20:00,10:20:00,D,5\n" +
			"detour,11:00:00,11:00:00,A,1\ndetour,11:05:00,11:05:00,B,2\ndetour,11:12:00,11:12:00,X,3\ndetour,11:18:00,11:18:00,C,4\ndetour,11:26:00,11:26:00,D,5\n" +
			"b1,06:30:00,06:30:00,D,1\nb1,06:38:00,06:38:00,C,2\nb1,06:45:00,06:45:00,B,3\nb1,06:50:00,06:50:00,A,4\n" +
			"b2,07:30:00,07:30:00,D,1\nb2,07:38:00,07:38:00,C,2\nb2,07:45:00,07:45:00,B,3\nb2,07:50:00,07:50:00,A,4\n",
	}
	for name, content := range replace {
		files[name] = content
	}
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, content := range files {
		f, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = f.Write([]byte(content))
	}
	_ = w.Close()
	data, err := parse(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// Numbers stay themselves, everything else is counted up from a thousand.
type ids map[string]int

func (m ids) ExternalID(kind, external string) (int, error) {
	if n, err := strconv.Atoi(external); err == nil {
		return n, nil
	}
	key := kind + ":" + external
	if id, ok := m[key]; ok {
		return id, nil
	}
	m[key] = 1000 + len(m)
	return m[key], nil
}

func TestSnapshotKeepsTheUsualPatternOnly(t *testing.T) {
	snap, err := testFeed(t).snapshot(ids{}, time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Routes) != 1 || snap.Routes[0].RouteShortName != "7" || snap.Routes[0].RouteColor != "#FF0000" {
		t.Fatalf("routes %+v", snap.Routes)
	}
	route := snap.Routes[0].RouteID
	if len(snap.Trips) != 2 || snap.Trips[0].TripID != strconv.Itoa(route)+"_0" || snap.Trips[1].TripID != strconv.Itoa(route)+"_1" {
		t.Fatalf("trips %+v", snap.Trips)
	}
	var out []int
	for _, ts := range snap.TripStops {
		if ts.TripID == snap.Trips[0].TripID {
			out = append(out, ts.StopID)
		}
	}
	if len(out) != 4 || len(snap.Stops) != 4 {
		// The extra stop is on no pattern, so it is not a stop of the network.
		t.Fatalf("pattern 0 has %d stops, snapshot %d", len(out), len(snap.Stops))
	}
	offsets := snap.Offsets[snap.Trips[0].TripID]
	// Medians of 5/6, 7/7, 8/8 minutes over the two weekday trips and the Saturday one.
	if len(offsets) != 4 || offsets[0] != 0 || offsets[1] != 300 || offsets[2] != 420 || offsets[3] != 480 {
		t.Fatalf("offsets %v", offsets)
	}
	if len(snap.Shapes) != 8 || snap.Shapes[0].ShapeID != snap.Trips[0].TripID {
		t.Fatalf("shapes %+v", snap.Shapes[:1])
	}
	tt := snap.Timetables["7"]
	if tt == nil || tt.InStopName != "Alpha" || tt.OutStopName != "Delta" {
		t.Fatalf("timetable %+v", tt)
	}
	// The full runs and the on-time run via X; not the short, the from-B or the late one.
	var in, back []string
	for _, e := range tt.Weekdays.Entries {
		if e.DepartureIn != "" {
			in = append(in, e.DepartureIn)
		}
		if e.DepartureOut != "" {
			back = append(back, e.DepartureOut)
		}
	}
	if len(in) != 3 || in[0] != "06:00" || in[2] != "10:00" || len(back) != 2 || back[0] != "06:30" {
		t.Fatalf("weekdays in %v out %v", in, back)
	}
	if n := len(tt.Saturday.Entries); n != 1 || tt.Saturday.Entries[0].DepartureIn != "09:00" {
		t.Fatalf("saturday %+v", tt.Saturday.Entries)
	}
}

func TestRunsAreTimedByTheHourTheyLeave(t *testing.T) {
	snap, err := testFeed(t).snapshot(ids{}, time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	hourly := snap.HourlyOffsets[snap.Trips[0].TripID]
	weekday := hourly[calendar.Weekday]
	// t1 leaves at 6 and rides 5 minutes to B, t2 at 7 and rides 6; no run leaves at 5 or 8, so they take the nearest hour's.
	if len(weekday) != 24 || weekday[6][1] != 300 || weekday[7][1] != 360 || weekday[8][1] != 360 || weekday[5][1] != 300 || weekday[7][3] != 480 {
		t.Fatalf("weekday hours %v", weekday)
	}
	if saturday := hourly[calendar.Saturday]; len(saturday) != 24 || saturday[20][1] != 300 {
		t.Fatalf("saturday hours %v", saturday)
	}
	if _, ok := hourly[calendar.Sunday]; ok {
		t.Fatal("a kind of day with no runs got times")
	}
}

func TestRidesAddUpToTimesTheRunsKeep(t *testing.T) {
	run := func(start int, at ...int) []stopTime {
		out := []stopTime{{stopID: "A", arrival: start, departure: start}}
		for i, m := range at {
			out = append(out, stopTime{stopID: string(rune('B' + i)), arrival: m, departure: m})
		}
		return out
	}
	stops := tripStops{
		"a":       run(420, 421, 422, 424),
		"b":       run(430, 432, 433, 434),
		"c":       run(440, 441, 443, 444),
		"untimed": run(450, -1, 453, 454),
	}
	// Medians per segment (1, 1, 1) would reach D in 3 minutes; every run takes 4.
	if got := rideMedians([]string{"a", "b", "c"}, 4, stops); !slices.Equal(got, []float64{0, 60, 120, 60}) {
		t.Fatalf("rides %v", got)
	}
	if got := rideMedians([]string{"untimed"}, 4, stops); !slices.Equal(got, []float64{0, -1, -1, 60}) {
		t.Fatalf("a stop no run times gave %v", got)
	}
}

func TestRepresentativeDaysStartWhereTheFeedDoes(t *testing.T) {
	f := testFeedWith(t, map[string]string{
		"calendar.txt": "service_id,monday,tuesday,wednesday,thursday,friday,saturday,sunday,start_date,end_date\n" +
			"WK,1,1,1,1,1,0,0,20261021,20270121\nWE,0,0,0,0,0,1,1,20261021,20270121\n",
	})
	days := f.representativeDays(time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC)) // a Sunday
	if days[calendar.Weekday].Day() != 21 || days[calendar.Saturday].Day() != 24 || days[calendar.Sunday].Day() != 25 {
		t.Fatalf("days %v", days)
	}
}

// Sibiu's feed drops its school runs on 30 November and 1 December 2026.
func TestAHolidayDoesNotStandForItsKindOfDay(t *testing.T) {
	f := testFeedWith(t, map[string]string{
		"calendar.txt": "service_id,monday,tuesday,wednesday,thursday,friday,saturday,sunday,start_date,end_date\n" +
			"WK,1,1,1,1,1,0,0,20230601,20270630\nSA,0,0,0,0,0,1,0,20230601,20270630\nE2026,1,1,1,1,1,0,0,20260826,20270618\n",
		"calendar_dates.txt": "service_id,date,exception_type\nE2026,20261130,2\nE2026,20261201,2\n",
	})
	now := time.Date(2026, 11, 30, 9, 0, 0, 0, time.UTC)
	day := f.representativeDays(now)[calendar.Weekday]
	if day.Format("20060102") != "20261202" || !f.servicesOn(day)["E2026"] {
		t.Fatalf("weekdays stand for %s, running %v", day.Format("20060102"), f.servicesOn(day))
	}
	// After the school year the plain weekday is all there is.
	if day := f.representativeDays(time.Date(2027, 6, 21, 0, 0, 0, 0, time.UTC))[calendar.Weekday]; f.servicesOn(day)["E2026"] {
		t.Fatalf("summer weekdays run the school extras on %s", day.Format("20060102"))
	}
}

func TestAnUnnamedRouteIsCalledAfterItsEnds(t *testing.T) {
	pats := testFeed(t).patterns
	names := map[string]string{"A": "Alpha", "D": "Delta"}
	if got := routeName(pats, "L1", names); got != "Alpha - Delta" {
		t.Fatalf("named %q", got)
	}
	// One direction only: its first stop and where it is headed.
	for id, p := range pats {
		if p.dir == 1 {
			delete(pats, id)
		}
	}
	if got := routeName(pats, "L1", names); got != "Alpha - Delta" {
		t.Fatalf("one-way route named %q", got)
	}
	if got := routeName(pats, "L9", names); got != "" {
		t.Fatalf("unknown route named %q", got)
	}
}

func TestALineWithoutDirectionsTakesThemFromWhereItsRunsHead(t *testing.T) {
	named := testFeed(t)
	trips := named.files["trips"]
	lines := []string{"route_id,service_id,trip_id,shape_id,trip_headsign"}
	for _, row := range trips.rows {
		lines = append(lines, strings.Join([]string{trips.get(row, "route_id"), trips.get(row, "service_id"), trips.get(row, "trip_id"), trips.get(row, "shape_id"), trips.get(row, "trip_headsign")}, ","))
	}
	unnamed := testFeedWith(t, map[string]string{"trips.txt": strings.Join(lines, "\n") + "\n"})
	if named.inferred != 0 || unnamed.inferred != 1 {
		t.Fatalf("inferred %d and %d lines", named.inferred, unnamed.inferred)
	}
	if !reflect.DeepEqual(unnamed.trips, named.trips) || !reflect.DeepEqual(unnamed.patterns, named.patterns) || unnamed.dropped != named.dropped {
		t.Fatalf("patterns %+v, want %+v", unnamed.patterns, named.patterns)
	}
}

func TestHeadwayTripsBecomeTimedRuns(t *testing.T) {
	f := testFeedWith(t, map[string]string{
		"trips.txt": "route_id,service_id,trip_id,direction_id,shape_id,trip_headsign\nL1,WK,every,0,s0,Delta\n",
		"stop_times.txt": "trip_id,arrival_time,departure_time,stop_id,stop_sequence\n" +
			"every,05:00:00,05:00:00,A,1\nevery,05:05:00,05:05:00,B,2\nevery,05:12:00,05:12:00,C,3\nevery,05:20:00,05:20:00,D,4\n",
		"frequencies.txt": "trip_id,start_time,end_time,headway_secs\nevery,06:00:00,06:30:00,600\nevery,06:30:00,07:10:00,1200\n",
	})
	snap, err := f.snapshot(ids{}, time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	tt := snap.Timetables["7"]
	var in []string
	for _, e := range tt.Weekdays.Entries {
		in = append(in, e.DepartureIn)
	}
	// Each window's runs, its end excluded; the template's own 05:00 is only a shape for them.
	if want := []string{"06:00", "06:10", "06:20", "06:30", "06:50"}; !reflect.DeepEqual(in, want) {
		t.Fatalf("weekdays %v, want %v", in, want)
	}
	if fr := tt.Weekdays.InFrequency; fr == nil || *fr != (models.Frequency{Start: "06:00", End: "07:10", MinMinutes: 10, MaxMinutes: 20}) {
		t.Fatalf("frequency %+v", fr)
	}
	if offsets := snap.Offsets[snap.Trips[0].TripID]; !reflect.DeepEqual(offsets, []float64{0, 300, 420, 480}) {
		t.Fatalf("offsets %v", offsets)
	}
}
