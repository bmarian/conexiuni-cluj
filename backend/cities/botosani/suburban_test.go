package botosani

import (
	"conexiuni-cluj/calendar"
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/models"
	"conexiuni-cluj/router"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

func fixtureMap(t *testing.T) aditMap {
	t.Helper()
	page, err := os.ReadFile("testdata/metrodreamin.html")
	if err != nil {
		t.Fatal(err)
	}
	maps, err := parseMap(page)
	if err != nil {
		t.Fatal(err)
	}
	return maps
}

func fixtureVillages(t *testing.T) []router.Area {
	t.Helper()
	data, err := os.ReadFile("testdata/villages.json")
	if err != nil {
		t.Fatal(err)
	}
	var out []router.Area
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestParseSuburbanReadsEachLineWithBothEnds(t *testing.T) {
	lines := fixturePage(t).suburban
	byShort := map[string]suburbanTimes{}
	var shorts []string
	for _, l := range lines {
		shorts = append(shorts, l.short)
		byShort[l.short] = l
	}
	// BT07 comes from the notes under MT07.
	want := []string{"MT01", "MT02", "MT03", "MT04", "MT05", "MT06", "MT07", "BT07", "MT08", "MT08B", "MT09", "MT10", "MT11", "MT12"}
	if !slices.Equal(shorts, want) {
		t.Fatalf("lines %v", shorts)
	}
	mt01 := byShort["MT01"]
	if mt01.title[0] != "Botoșani" || mt01.title[3] != "Bohoghina" || !slices.Equal(mt01.minutes, []int{60}) {
		t.Fatalf("MT01 %+v", mt01)
	}
	town, far := mt01.ends[0], mt01.ends[1]
	if town.place != "Botoșani" || town.hub != "Luceafărul" || far.place != "Bohoghina" || far.hub != "" {
		t.Fatalf("MT01 ends %+v", mt01.ends)
	}
	if len(town.days[calendar.Weekday]) != 12 || len(town.days[calendar.Saturday]) != 5 || len(far.days[calendar.Sunday]) != 6 {
		t.Errorf("MT01 departures %+v", mt01.ends)
	}
	if got := town.days[calendar.Weekday][10]; got != (timed{minute: 20*60 + 20, term: true}) {
		t.Errorf("MT01 *20:20 read as %+v", got)
	}
	// The *20:15** run from Botoșani is BT07's.
	if mt07 := byShort["MT07"]; len(mt07.ends[0].days[calendar.Weekday]) != 9 || mt07.ends[0].hub != "Bariera Cătămărești" {
		t.Errorf("MT07 %+v", mt07.ends[0])
	}
	if mt09 := byShort["MT09"]; !slices.Equal(mt09.minutes, []int{40, 25}) {
		t.Errorf("MT09 trip minutes %v", mt09.minutes)
	}
	bt07 := byShort["BT07"]
	route := []string{"Botoșani", "Zăicești", "Buzeni", "Bălușeni", "Bălușenii Noi"}
	if bt07.follows != "MT07" || !slices.Equal(bt07.title, route) || bt07.ends[0].hub != "Bariera Cătămărești" {
		t.Errorf("BT07 %+v", bt07)
	}
	// 20:15 is noted under MT07 and MT08** alike.
	if got, want := bt07.ends[0].days[calendar.Weekday], []timed{{20*60 + 15, true}, {20*60 + 15, true}}; !slices.Equal(got, want) {
		t.Errorf("BT07 from Botoșani %v", got)
	}
	if got := bt07.ends[1].days[calendar.Weekday]; !slices.Equal(got, []timed{{21*60 + 15, true}}) {
		t.Errorf("BT07 from Bălușenii Noi %v", got)
	}
	if mt08b := byShort["MT08B"]; mt08b.follows != "MT08" || len(mt08b.ends[0].days[calendar.Sunday]) != 3 || len(mt08b.ends[0].days[calendar.Weekday]) != 3 {
		t.Errorf("MT08B %+v", mt08b)
	}
}

func TestADITMapKeepsTheLongestPieceOfEachLine(t *testing.T) {
	page, err := os.ReadFile("testdata/harta-interactiva.html")
	if err != nil {
		t.Fatal(err)
	}
	if link, err := mapLink(page); err != nil || link != "https://metrodreamin.com/view/RU5QUjNiT0g2S2haeXFYbUhieDVYa251YzIwM3ww" {
		t.Fatalf("link %q, %v", link, err)
	}
	maps := fixtureMap(t)
	for short, stops := range map[string]int{"MT01": 27, "MT04": 10, "MT08": 20, "MT09": 9, "MT12": 14} {
		if len(maps.lines[short].stops) != stops {
			t.Errorf("%s: %d stops, want %d", short, len(maps.lines[short].stops), stops)
		}
	}
	// BT01 and the MT lines, three of them in two pieces.
	if len(maps.pieces) != 16 {
		t.Errorf("%d pieces", len(maps.pieces))
	}
	for _, l := range maps.pieces {
		for k, m := range l.stops {
			if strings.Contains(m.name, "  ") || l.points[l.at[k]] != m.at {
				t.Errorf("stop %+v at point %d", m, l.at[k])
			}
		}
	}
}

func suburbanSnapshot(t *testing.T, term bool) (map[string][]string, *feeds.Snapshot) {
	t.Helper()
	parsed := fixturePage(t)
	snap, err := build(sources{
		lines: parsed.lines, programs: fixtureProgram(t), suburban: parsed.suburban, adit: fixtureMap(t), villages: fixtureVillages(t),
		term: map[string]bool{calendar.Weekday: term, calendar.Saturday: term, calendar.Sunday: term},
	}, externalIDs{}, streets{})
	if err != nil {
		t.Fatal(err)
	}
	names := map[int]string{}
	for _, s := range snap.Stops {
		names[s.StopID] = s.StopName
	}
	ways := map[string][]string{}
	for _, ts := range snap.TripStops {
		ways[ts.TripID] = append(ways[ts.TripID], names[ts.StopID])
	}
	byShort := map[string][]string{}
	for _, trip := range snap.Trips {
		for _, r := range snap.Routes {
			if r.RouteID == trip.RouteID {
				byShort[fmt.Sprintf("%s/%d", r.RouteShortName, trip.DirectionID)] = ways[trip.TripID]
			}
		}
	}
	return byShort, snap
}

func TestSuburbanLinesStopAtTheTownStationsOfTheBTLines(t *testing.T) {
	ways, snap := suburbanSnapshot(t, true)
	// ADIT's "Strada Pacea" and "Bulevardul George Enescu" are George Enescu's two sides.
	if got := ways["MT03/0"][:5]; !slices.Equal(got, []string{"George Enescu", "Piața Mică", "Prieteniei", "Primăverii", "Botosani - Bariera Sucevei"}) {
		t.Errorf("MT03 from town %q", got)
	}
	// ADIT's own stop between Șuruburi and Bariera stays apart.
	if got := ways["MT01/0"][10:14]; !slices.Equal(got, []string{"Șuruburi", "Eltrans", "Bariera Cătămărăști", "Catamarasti-Deal - Tridex"}) {
		t.Errorf("MT01 from town %q", got)
	}
	if got := ways["MT11/0"]; got[len(got)-1] != "Roșiori" {
		t.Errorf("MT11's unnamed terminus %q", got[len(got)-1])
	}
	for _, s := range snap.Stops {
		if s.StopName == unnamedStation {
			t.Errorf("unnamed stop %+v", s)
		}
	}
	for _, s := range snap.Stops {
		if slices.Contains([]string{"Luceafarul", "Strada Pacea", "Bazar/Panda", "Spital Recuperare", "Strada Primăverii"}, s.StopName) {
			t.Errorf("ADIT's own %q kept beside the town station", s.StopName)
		}
	}
}

func TestSuburbanTimesFromBothEndsAndTheTripTime(t *testing.T) {
	_, snap := suburbanSnapshot(t, true)
	mt01 := snap.Timetables["MT01"]
	if mt01.RouteLongName != "Botoșani - Bohoghina" || mt01.InStopName != "Luceafărul" {
		t.Errorf("MT01 %+v", mt01)
	}
	in, out := columns(mt01.Weekdays)
	if len(in) != 12 || len(out) != 14 || in[0] != "05:50" || out[0] != "05:05" {
		t.Errorf("MT01 weekdays %v / %v", in, out)
	}
	for _, r := range snap.Routes {
		if r.RouteShortName != "MT09" {
			continue
		}
		for d, want := range []float64{40, 25} {
			sum := 0.0
			for _, o := range snap.Offsets[models.PatternID(r.RouteID, d)] {
				sum += o
			}
			if math.Abs(sum-want*60) > 1 {
				t.Errorf("MT09 way %d takes %.0f s, want %.0f min", d, sum, want)
			}
		}
	}
	_, holidays := suburbanSnapshot(t, false)
	if in, out := columns(holidays.Timetables["MT01"].Weekdays); len(in) != 11 || len(out) != 13 {
		t.Errorf("MT01 in the school holidays: %d and %d departures", len(in), len(out))
	}
}

type schoolDays func(time.Time) bool

func (f schoolDays) SchoolDay(t time.Time) bool { return f(t) }

func TestTermRunsFollowMostOfTheSampledWeeks(t *testing.T) {
	start := time.Date(2026, 12, 14, 0, 0, 0, 0, time.UTC)
	// The winter break from 20 December, past the middle of four weeks.
	term := termRuns(schoolDays(func(d time.Time) bool { return d.Before(time.Date(2026, 12, 20, 0, 0, 0, 0, time.UTC)) }), start)
	if term[calendar.Weekday] {
		t.Error("starred runs kept through a mostly holiday month")
	}
	term = termRuns(schoolDays(func(time.Time) bool { return true }), start)
	if !term[calendar.Weekday] || !term[calendar.Saturday] {
		t.Errorf("term %v", term)
	}
}

func TestRoutesTheMapHasNoLineForFollowTheirPlaces(t *testing.T) {
	ways, snap := suburbanSnapshot(t, true)
	// MT08 to Bălușeni, then ADIT's branch to Bălușenii Noi.
	if got := ways["MT08B/0"][16:]; !slices.Equal(got, []string{"Cosuleni Ram.", "Baluseni - La Buiucu", "Bălușeni", "Bălușenii Noi"}) {
		t.Errorf("MT08B %q", got)
	}
	// MT07 through Zăicești to Buzeni, across to MT08 for Bălușeni, then the same piece.
	want := []string{
		"Zaicesti Ram.", "Zaicesti - Scoala Primara Nr.1", "Zaicesti - Biserica Adomnirea Maicii Domnului", "Buzeni - Ramificatie Cosuleni",
		"Buzeni - Intersectie", "Cosuleni Ram.", "Baluseni - La Buiucu", "Bălușeni", "Bălușenii Noi",
	}
	if got := ways["BT07/0"][13:]; !slices.Equal(got, want) {
		t.Errorf("BT07 %q", got)
	}
	if got := ways["BT07/1"]; got[0] != "Bălușenii Noi" || got[len(got)-1] != "Bariera Cătămărăști" {
		t.Errorf("BT07 back %q", got)
	}
	if in, out := columns(snap.Timetables["BT07"].Weekdays); !slices.Equal(in, []string{"20:15"}) || !slices.Equal(out, []string{"21:15"}) {
		t.Errorf("BT07 weekdays %v / %v", in, out)
	}
	if in, _ := columns(snap.Timetables["BT07"].Saturday); len(in) != 0 {
		t.Errorf("BT07 on Saturdays %v", in)
	}
	if got := snap.Timetables["MT08B"]; got.RouteLongName != "Botoșani - Bălușenii Noi" {
		t.Errorf("MT08B %+v", got)
	}
}

func TestTerminiTakeThePagesNameForTheVillageTheyAreIn(t *testing.T) {
	ways, _ := suburbanSnapshot(t, true)
	for short, want := range map[string]string{
		"MT01": "Bohoghina", // ADIT: "Bucecea", the town Bohoghina belongs to
		"MT05": "Băiceni",   // "Strada Pădurii"
		"MT10": "Victoria",  // "Centura Botoșani (etapa 2)"
		"MT11": "Roșiori",   // unnamed
		"MT07": "Buzeni - Berlina COM",
		"MT04": "Manastirea Doamnei - Inter. Str. Nucului", // the map stops short of Agafton
	} {
		if got := ways[short+"/0"]; got[len(got)-1] != want {
			t.Errorf("%s ends at %q, want %q", short, got[len(got)-1], want)
		}
	}
}

func TestAWayOnBothSidesOfOneStationOnlyIsNoWay(t *testing.T) {
	sides := eastward("a")
	line := mapLine{points: []router.Point{sides[0].at, sides[1].at}, stops: sides, at: []int{0, 1}}
	stations := []station{{markers: sides, centre: centreOf(sides)}}
	if _, ok := suburbanWay(line, map[string]int{"a-south": 0, "a-north": 0}, stations, streets{}, 0); ok {
		t.Error("a way with one stop")
	}
}
