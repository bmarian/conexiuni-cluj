package main

import (
	"bytes"
	"conexiuni-cluj/cities"
	"conexiuni-cluj/config"
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/models"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

var updateFixture = flag.Bool("update", false, "rewrite testdata/parity from this run")

// Two small made-up cities through the whole server, compared with testdata/parity.
func TestAPIFixture(t *testing.T) {
	golden, err := filepath.Abs(filepath.Join("testdata", "parity"))
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "out")
	if keep := os.Getenv("FIXTURE_OUT"); keep != "" {
		out = keep
	}
	work := parityWorkdir(t)
	tranzyLike := &fixtureStatic{base: 1, lat: 46.0, lon: 24.0}
	gtfsLike := &fixtureStatic{base: 7, lat: 45.0, lon: 25.0, withTimes: true}
	parityOSM(t, work, map[string][]osmStop{"fixture": osmStops(tranzyLike.snapshot(false)), "fixturegtfs": osmStops(gtfsLike.snapshot(false))})

	cfg := config.Load()
	bucharest, err := time.LoadLocation("Europe/Bucharest")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 20, 3, 0, 0, 0, bucharest)
	recordParity(t, out, now, now.Add(3*time.Hour+40*time.Minute), cfg, func() ([]*cities.City, error) {
		first, err := openStore(cfg, "fixture")
		if err != nil {
			return nil, err
		}
		second, err := openStore(cfg, "fixturegtfs")
		if err != nil {
			return nil, err
		}
		opts, err := cityOptions(cfg)
		if err != nil {
			return nil, err
		}
		opts.Refresh = true
		a := cities.New("fixture", "Fixture", first, tranzyLike, tranzyLike, fixtureRealtime{}, nil, opts)
		a.Operator, a.OperatorURL, a.News = "Fixture Transit", "https://fixture.example/", fixtureNews{at: now.Add(-3 * time.Hour)}
		b := cities.New("fixturegtfs", "Fixture GTFS", second, gtfsLike, nil, nil, nil, opts)
		b.Operator, b.OperatorURL = "Fixture Rail", "https://rail.fixture.example/"
		return []*cities.City{a, b}, nil
	})

	if *updateFixture {
		if err := os.RemoveAll(golden); err != nil {
			t.Fatal(err)
		}
		copyDir(t, out, golden)
		return
	}
	compareDirs(t, golden, out)
}

func compareDirs(t *testing.T, want, got string) {
	t.Helper()
	names := func(dir string) []string {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, e := range entries {
			out = append(out, e.Name())
		}
		return out
	}
	wantNames, gotNames := names(want), names(got)
	for _, name := range wantNames {
		if !slices.Contains(gotNames, name) {
			t.Errorf("missing %s", name)
		}
	}
	for _, name := range gotNames {
		if !slices.Contains(wantNames, name) {
			t.Errorf("new %s", name)
			continue
		}
		a, _ := os.ReadFile(filepath.Join(want, name))
		b, _ := os.ReadFile(filepath.Join(got, name))
		if !bytes.Equal(a, b) {
			t.Errorf("%s differs:\n%s", name, firstDifference(string(a), string(b)))
		}
	}
	if t.Failed() {
		t.Log("go test -run TestAPIFixture -update . rewrites testdata/parity once the change is meant")
	}
}

func firstDifference(a, b string) string {
	al, bl := strings.Split(a, "\n"), strings.Split(b, "\n")
	for i := range max(len(al), len(bl)) {
		var x, y string
		if i < len(al) {
			x = al[i]
		}
		if i < len(bl) {
			y = bl[i]
		}
		if x != y {
			return fmt.Sprintf("line %d\n- %s\n+ %s", i+1, x, y)
		}
	}
	return ""
}

// Two lines crossing at a shared stop, ids starting at base*100; changed, the second line no longer calls at its second stop.
func fixtureSnapshot(base int, lat, lon float64, changed bool) *feeds.Snapshot {
	snap := &feeds.Snapshot{}
	shared := base*100 + 3
	colors := map[int][2]string{1: {"#C62F24", "#16D057"}, 7: {"#D84B3B", "#1C38D9"}}[base]
	lines := []struct {
		id         int
		name       string
		color      string
		kind       models.RouteType
		stops      []int
		dLat, dLon float64
	}{
		{base, fmt.Sprint(base), colors[0], 3, []int{base*100 + 1, base*100 + 2, shared, base*100 + 4, base*100 + 5}, 0, 0.006},
		{base + 1, fmt.Sprintf("%dB", base+1), colors[1], 0, []int{base*100 + 11, base*100 + 12, shared, base*100 + 14, base*100 + 15}, 0.004, 0},
	}
	seen := map[int]bool{}
	for _, l := range lines {
		snap.Routes = append(snap.Routes, models.Route{RouteID: l.id, AgencyID: 1, RouteShortName: l.name,
			RouteLongName: fmt.Sprintf("Capăt %d - Capăt %d", l.stops[0], l.stops[len(l.stops)-1]), RouteType: l.kind,
			RouteColor: l.color})
		points := make([][2]float64, len(l.stops))
		for i, id := range l.stops {
			offset := float64(i - 2)
			points[i] = [2]float64{lat + offset*l.dLat, lon + offset*l.dLon}
			if changed && id == base*100+12 {
				continue
			}
			if !seen[id] {
				seen[id] = true
				snap.Stops = append(snap.Stops, models.Stop{StopID: id, StopName: fmt.Sprintf("Stația %d", id), StopLat: points[i][0], StopLon: points[i][1]})
			}
		}
		for dir := range 2 {
			tripID := fmt.Sprintf("%d_%d", l.id, dir)
			stops := slices.Clone(l.stops)
			pts := slices.Clone(points)
			if dir == 1 {
				slices.Reverse(stops)
				slices.Reverse(pts)
			}
			snap.Trips = append(snap.Trips, models.Trip{TripID: tripID, RouteID: l.id, DirectionID: models.DirectionType(dir),
				TripHeadsign: fmt.Sprintf("Capăt %d", stops[len(stops)-1]), ShapeID: tripID})
			seq := 0
			for _, id := range stops {
				if changed && id == base*100+12 {
					continue
				}
				seq++
				snap.TripStops = append(snap.TripStops, models.TripStop{TripID: tripID, StopID: id, StopSequence: seq})
			}
			seq = 0
			for i := range pts {
				for step := range 4 {
					if i == len(pts)-1 && step > 0 {
						break
					}
					f := float64(step) / 4
					p := pts[i]
					if i+1 < len(pts) {
						p = [2]float64{p[0] + f*(pts[i+1][0]-p[0]), p[1] + f*(pts[i+1][1]-p[1])}
					}
					seq++
					snap.Shapes = append(snap.Shapes, models.Shape{ShapeID: tripID, ShapePtLat: p[0], ShapePtLon: p[1], ShapePtSequence: seq})
				}
			}
		}
	}
	return snap
}

// Changed, the first line runs every 15 minutes instead of 12.
func fixtureTimetables(snap *feeds.Snapshot, changed bool) map[string]*models.Timetable {
	out := map[string]*models.Timetable{}
	for i, route := range snap.Routes {
		every := []int{12, 20}[i%2]
		if changed && i == 0 {
			every = 15
		}
		day := func(from, to, step int) models.DaySchedule {
			var d models.DaySchedule
			for m := from; m <= to; m += step {
				d.Entries = append(d.Entries, models.TimetableEntry{
					DepartureIn:  fmt.Sprintf("%02d:%02d", m/60, m%60),
					DepartureOut: fmt.Sprintf("%02d:%02d", (m+5)/60, (m+5)%60),
				})
			}
			return d
		}
		var first, last string
		for _, ts := range snap.TripStops {
			if ts.TripID == fmt.Sprintf("%d_0", route.RouteID) && ts.StopSequence == 1 {
				first = fmt.Sprintf("Stația %d", ts.StopID)
			}
			if ts.TripID == fmt.Sprintf("%d_1", route.RouteID) && ts.StopSequence == 1 {
				last = fmt.Sprintf("Stația %d", ts.StopID)
			}
		}
		t := &models.Timetable{RouteShortName: route.RouteShortName, RouteLongName: route.RouteLongName, InStopName: first, OutStopName: last,
			Weekdays: day(5*60, 23*60, every), Saturday: day(6*60, 23*60, 2*every), Sunday: day(7*60, 22*60, 30)}
		if i%2 == 1 {
			t.Sunday = models.DaySchedule{InFrequency: &models.Frequency{Start: "07:00", End: "22:00", MinMinutes: 25, MaxMinutes: 35},
				OutFrequency: &models.Frequency{Start: "07:10", End: "22:10", MinMinutes: 25, MaxMinutes: 35}}
		}
		out[route.RouteShortName] = t
	}
	return out
}

func osmStops(snap *feeds.Snapshot) []osmStop {
	out := make([]osmStop, len(snap.Stops))
	for i, s := range snap.Stops {
		out[i] = osmStop{id: s.StopID, name: s.StopName, lat: s.StopLat, lon: s.StopLon}
	}
	return out
}

// The first Snapshot is the network as it was, every later one the changed network.
type fixtureStatic struct {
	base      int
	lat, lon  float64
	withTimes bool
	calls     atomic.Int32
}

func (f *fixtureStatic) snapshot(changed bool) *feeds.Snapshot {
	snap := fixtureSnapshot(f.base, f.lat, f.lon, changed)
	if f.withTimes {
		snap.Timetables = fixtureTimetables(snap, changed)
		snap.Offsets = map[string][]float64{}
		for _, trip := range snap.Trips {
			snap.Offsets[trip.TripID] = []float64{0, 90, 120, 75, 100}[:len(tripStopsOf(snap, trip.TripID))]
		}
	}
	return snap
}

func tripStopsOf(snap *feeds.Snapshot, tripID string) []models.TripStop {
	var out []models.TripStop
	for _, ts := range snap.TripStops {
		if ts.TripID == tripID {
			out = append(out, ts)
		}
	}
	return out
}

func (f *fixtureStatic) Snapshot() (*feeds.Snapshot, error) {
	return f.snapshot(f.calls.Add(1) > 1), nil
}

func (f *fixtureStatic) Timetables(snap *feeds.Snapshot) (map[string]*models.Timetable, error) {
	return fixtureTimetables(snap, f.calls.Load() > 1), nil
}

// Off the whole seconds the vehicles are polled at, so the second import never races a poll.
func (f *fixtureStatic) Cadence() time.Duration { return 2*time.Hour + 13370*time.Millisecond }

// Three vehicles out and back along their lines with the clock, a minute's stand at each end.
type fixtureRealtime struct{}

func (fixtureRealtime) Vehicles() ([]models.Vehicle, error) {
	now := time.Now()
	const leg, stand = 6 * time.Minute, time.Minute
	cycle := 2 * (leg + stand)
	var out []models.Vehicle
	for _, bus := range []struct {
		id, route int
		label     string
		kind      int
		// The first stop and the last, of direction 0.
		from, to [2]float64
		lag      time.Duration
	}{
		{11, 1, "B11", 3, [2]float64{46.0, 23.988}, [2]float64{46.0, 24.012}, 0},
		{12, 1, "B12", 3, [2]float64{46.0, 23.988}, [2]float64{46.0, 24.012}, 7 * time.Minute},
		{21, 2, "T21", 0, [2]float64{45.992, 24.0}, [2]float64{46.008, 24.0}, 3 * time.Minute},
	} {
		at := time.Duration(now.Add(bus.lag).UnixNano()) % cycle
		direction, along, speed := 0, 0.0, 20.0
		switch {
		case at < leg:
			along = float64(at) / float64(leg)
		case at < leg+stand:
			along, speed = 1, 0
		case at < 2*leg+stand:
			direction, along = 1, 1-float64(at-leg-stand)/float64(leg)
		default:
			direction, speed = 1, 0
		}
		v := models.Vehicle{ID: bus.id, Label: bus.label, VehicleType: bus.kind, Speed: speed, RouteID: bus.route,
			TripID:    fmt.Sprintf("%d_%d", bus.route, direction),
			Latitude:  bus.from[0] + along*(bus.to[0]-bus.from[0]),
			Longitude: bus.from[1] + along*(bus.to[1]-bus.from[1]),
			Timestamp: now.UTC().Format(time.RFC3339)}
		if bus.id == 11 {
			v.BikeAccessible, v.WheelchairAccessible = "BIKE_ACCESSIBLE", "WHEELCHAIR_ACCESSIBLE"
		}
		out = append(out, v)
	}
	return out, nil
}

func (fixtureRealtime) Budget() feeds.Budget { return feeds.Budget{} }

type fixtureNews struct{ at time.Time }

func (f fixtureNews) News(*time.Location) ([]feeds.NewsItem, error) {
	return []feeds.NewsItem{
		{Title: "Linia 1 circulă deviat", URL: "https://fixture.example/stiri/1", At: f.at},
		{Title: "Program de sărbători", URL: "https://fixture.example/stiri/2", At: f.at.Add(-48 * time.Hour)},
	}, nil
}

func (fixtureNews) Cadence() time.Duration { return time.Hour }
