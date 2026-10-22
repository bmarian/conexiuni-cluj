package suceava

import (
	"conexiuni-cluj/database"
	"conexiuni-cluj/feeds/traces"
	"conexiuni-cluj/router"
	"math"
	"testing"
	"time"
)

type memTraces struct{ fixes []database.LineFix }

func (m *memTraces) TracedLines() ([]string, error)         { return nil, nil }
func (m *memTraces) SetTracedLines([]string) error          { return nil }
func (m *memTraces) PruneLineFixes(string, time.Time) error { return nil }
func (m *memTraces) AddLineFixes(fixes []database.LineFix) error {
	m.fixes = append(m.fixes, fixes...)
	return nil
}
func (m *memTraces) LineFixes(line string, _ time.Time) ([]database.LineFix, error) {
	return m.fixes, nil
}

// One bus of line M1 through the points, a minute apart; stands give how many minutes it stood at each.
func recordedM1(t *testing.T, points []router.Point, stands map[int]int) map[string]*traces.Line {
	t.Helper()
	at := time.Unix(1_800_000_000, 0)
	store := &memTraces{}
	for i, p := range points {
		last := at.Add(time.Duration(stands[i]) * time.Minute)
		store.fixes = append(store.fixes, database.LineFix{Line: "M1", Vehicle: "SV1", Lat: p.Lat, Lon: p.Lon, First: at, Last: last})
		at = last.Add(time.Minute)
	}
	lines, err := traces.NewRecorder(store, 30).Trace(map[string]time.Time{"M1": {}})
	if err != nil {
		t.Fatal(err)
	}
	return lines
}

func pt(lat, lon float64) router.Point { return router.Point{Lat: lat, Lon: lon} }

func TestAnEndNothingNamesIsWhereBusesStand(t *testing.T) {
	p := newPlacer(testSnapshot(), nil, nil)
	// Up past Obcini and Spital, ten minutes at the end, back past Spital.
	recorded := recordedM1(t, []router.Point{pt(47.640, 26.25), pt(47.645, 26.25), pt(47.649, 26.25), pt(47.652, 26.25), pt(47.655, 26.25), pt(47.652, 26.25), pt(47.649, 26.25)}, map[int]int{4: 10})
	placed, counts := p.place([]pageLine{{short: "M1", directions: [][]string{{"OBCINI", "SPITAL", "CAPAT"}, {"CAPAT", "SPITAL", "OBCINI"}}}}, recorded)
	if placed[0] == nil {
		t.Fatalf("line left out, counts %v", counts)
	}
	for d, dir := range placed[0].dirs {
		end := dir[len(dir)-1]
		if d == 1 {
			end = dir[0]
		}
		if math.Abs(end.at.Lat-47.655) > 1e-9 {
			t.Fatalf("direction %d end %+v", d, end)
		}
	}
	if counts[fromTraces] != 1 {
		t.Fatalf("counts %v", counts)
	}
}

func TestAStationNoBusPassesIsSpacedAlongTheirWay(t *testing.T) {
	p := newPlacer(testSnapshot(), nil, nil)
	obcini, metro := p.stops[0], p.stops[2]
	dir := []station{
		{name: "OBCINI", stop: &obcini, at: pt(obcini.StopLat, obcini.StopLon), from: fromZip},
		{name: "DEPARTE", at: pt(47.645, 26.30), from: fromName},
		{name: "METRO", stop: &metro, at: pt(metro.StopLat, metro.StopLon), from: fromZip},
	}
	// The bus goes east of the straight way and back.
	recorded := recordedM1(t, []router.Point{pt(47.640, 26.25), pt(47.646, 26.26), pt(47.652, 26.26), pt(47.658, 26.25)}, nil)["M1"]
	p.checkAgainstTraces(dir, recorded)
	if dir[1].from != disproved {
		t.Fatalf("station 4 km off the bus's way kept: %+v", dir[1])
	}
	p.interpolate(dir, p.route(recorded))
	// Halfway along the way the bus drove, on its eastern leg, not on the straight line between the ends.
	if dir[1].from != spaced || math.Abs(dir[1].at.Lon-26.26) > 1e-3 {
		t.Fatalf("spaced at %+v", dir[1].at)
	}
}

func TestOnlyWholeDisplaysAreTraced(t *testing.T) {
	traced := func(line string) bool { return line == "M104" || line == "M6" }
	for label, want := range map[string]string{"M104": "M104", "M10": "", "  6": "M6", "M6.": "M6", "5": ""} {
		if got := tracedLine(label, traced); got != want {
			t.Errorf("%q: %q, want %q", label, got, want)
		}
	}
}
