package gtfs

import (
	"archive/zip"
	"bytes"
	"strconv"
	"testing"
	"time"
)

// One line A-B-C-D: three full trips, a short run, a run from B, an on-time and a late run via X; two trips back.
func testFeed(t *testing.T) *feed {
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

func TestRepresentativeDaysStartWhereTheFeedDoes(t *testing.T) {
	now := time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC) // a Sunday
	start := time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC)
	days := representativeDays(now, start, start.AddDate(0, 3, 0))
	if days["weekdays"].Day() != 21 || days["saturday"].Day() != 24 || days["sunday"].Day() != 25 {
		t.Fatalf("days %v", days)
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
