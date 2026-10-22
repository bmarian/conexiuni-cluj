package traces

import (
	"conexiuni-cluj/database"
	"conexiuni-cluj/router"
	"math"
	"slices"
	"strings"
	"testing"
	"time"
)

type memStore struct {
	lines []string
	fixes []database.LineFix
}

func (m *memStore) TracedLines() ([]string, error)      { return m.lines, nil }
func (m *memStore) SetTracedLines(lines []string) error { m.lines = lines; return nil }
func (m *memStore) AddLineFixes(fixes []database.LineFix) error {
	m.fixes = append(m.fixes, fixes...)
	return nil
}
func (m *memStore) LineFixes(line string, since time.Time) ([]database.LineFix, error) {
	var out []database.LineFix
	for _, f := range m.fixes {
		if f.Line == line && !f.First.Before(since) {
			out = append(out, f)
		}
	}
	slices.SortStableFunc(out, func(a, b database.LineFix) int {
		if c := strings.Compare(a.Vehicle, b.Vehicle); c != 0 {
			return c
		}
		return a.First.Compare(b.First)
	})
	return out, nil
}
func (m *memStore) PruneLineFixes(line string, before time.Time) error {
	m.fixes = slices.DeleteFunc(m.fixes, func(f database.LineFix) bool { return f.Line == line && f.First.Before(before) })
	return nil
}

var t0 = time.Unix(1_800_000_000, 0)

func TestARecorderKeepsOneRowPerPlace(t *testing.T) {
	store := &memStore{lines: []string{"M104"}}
	r := NewRecorder(store, 30)
	if !r.Traced("M104") || r.Traced("5") {
		t.Fatal("traced lines not read from the store")
	}
	at := t0
	poll := func(seen ...Sighting) {
		if err := r.Record(seen, at); err != nil {
			t.Fatal(err)
		}
		at = at.Add(30 * time.Second)
	}
	// Three polls standing (jitter under 30 m), a move, then gone.
	poll(Sighting{"M104", "SV1", 47.65, 26.25})
	poll(Sighting{"M104", "SV1", 47.65005, 26.25})
	poll(Sighting{"M104", "SV1", 47.65, 26.25005})
	poll(Sighting{"M104", "SV1", 47.652, 26.25})
	poll()
	if len(store.fixes) != 2 || store.fixes[0].Last.Sub(store.fixes[0].First) != time.Minute || store.fixes[1].Lat != 47.652 {
		t.Fatalf("fixes %+v", store.fixes)
	}
}

func TestTracingDropsWhatIsOlderThanTheRoute(t *testing.T) {
	store := &memStore{fixes: []database.LineFix{
		{Line: "M6", Vehicle: "a", First: t0, Last: t0},
		{Line: "M6", Vehicle: "a", First: t0.Add(time.Hour), Last: t0.Add(time.Hour)},
		{Line: "M9", Vehicle: "b", First: t0, Last: t0},
	}}
	r := NewRecorder(store, 30)
	lines, err := r.Trace(map[string]time.Time{"M6": t0.Add(time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	if lines["M6"].Buses() != 1 || len(lines["M6"].buses[0]) != 1 || !r.Traced("M6") || r.Traced("M9") {
		t.Fatalf("lines %+v, store %+v", lines, store)
	}
}

func p(lat, lon float64) router.Point { return router.Point{Lat: lat, Lon: lon} }

// A bus's fixes a minute apart; stands give how many minutes it stood at each.
func bus(vehicle string, points []router.Point, stands map[int]int) []database.LineFix {
	at := t0
	var out []database.LineFix
	for i, pt := range points {
		last := at.Add(time.Duration(stands[i]) * time.Minute)
		out = append(out, database.LineFix{Line: "L", Vehicle: vehicle, Lat: pt.Lat, Lon: pt.Lon, First: at, Last: last})
		at = last.Add(time.Minute)
	}
	return out
}

func straight(a, b router.Point) []router.Point { return []router.Point{a, b} }

func TestAPathIsTheMedianPassInTheOrderDriven(t *testing.T) {
	a, b := p(47.640, 26.25), p(47.658, 26.25)
	// Three buses: one a little east, one further east, one far east; the median is the further-east one.
	var fixes []database.LineFix
	for i, lon := range []float64{26.255, 26.26, 26.30} {
		fixes = append(fixes, bus(string(rune('a'+i)), []router.Point{a, p(47.649, lon), b}, nil)...)
	}
	l := newLine(fixes)
	path := l.Path(a, b, straight)
	if len(path) != 3 || path[1].Lon != 26.26 {
		t.Fatalf("path %v", path)
	}
	if l.Path(b, a, straight) != nil {
		t.Fatal("no bus drove the other way")
	}
	if d := DistanceToPath(p(47.649, 26.26), path); d > 1 {
		t.Fatalf("distance to own point %.1f m", d)
	}
}

func TestAnEndIsWhereBusesStandLongest(t *testing.T) {
	spital, metro := p(47.649, 26.25), p(47.645, 26.25)
	// Past Metro and Spital, two minutes at a crossing, ten at the end, back past Spital.
	l := newLine(bus("a", []router.Point{metro, spital, p(47.652, 26.25), p(47.655, 26.25), p(47.652, 26.25), spital}, map[int]int{2: 2, 3: 10}))
	end, ok := l.End(&metro, spital, []router.Point{metro, spital}, 1000, false)
	if !ok || math.Abs(end.Lat-47.655) > 1e-9 {
		t.Fatalf("end %v %v", end, ok)
	}
	if _, ok := l.End(&metro, spital, []router.Point{metro, spital}, 200, false); ok {
		t.Fatal("a stand beyond reach was taken")
	}
	// The way into the line: where it stood before reaching Metro, coming from the south.
	l = newLine(bus("a", []router.Point{p(47.630, 26.25), p(47.640, 26.25), metro, spital}, map[int]int{0: 8}))
	start, ok := l.End(&spital, metro, []router.Point{metro, spital}, 2000, true)
	if !ok || math.Abs(start.Lat-47.630) > 1e-9 {
		t.Fatalf("start %v %v", start, ok)
	}
}
