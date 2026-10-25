package slugs

import (
	"conexiuni-cluj/database"
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/models"
	"path/filepath"
	"strconv"
	"testing"
)

type stop struct {
	id, station int
	name        string
	lat, lon    float64
	routes      []int
}

func snapshot(routes []models.Route, stops []stop) *feeds.Snapshot {
	snap := &feeds.Snapshot{Routes: routes}
	for _, s := range stops {
		snap.Stops = append(snap.Stops, models.Stop{StopID: s.id, StationID: s.station, StopName: s.name, StopLat: s.lat, StopLon: s.lon})
		for _, r := range s.routes {
			trip := strconv.Itoa(r) + "_0"
			snap.Trips = append(snap.Trips, models.Trip{TripID: trip, RouteID: r})
			snap.TripStops = append(snap.TripStops, models.TripStop{TripID: trip, StopID: s.id})
		}
	}
	return snap
}

func newService(t *testing.T) *Service {
	t.Helper()
	dir := t.TempDir()
	store, err := database.Open(filepath.Join(dir, "test.db"), filepath.Join(dir, "archive.db"), 5)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)
	return New(store)
}

func want(t *testing.T, x *Index, kind Kind, expected map[int]string) {
	t.Helper()
	for id, slug := range expected {
		if got := x.Of(kind, id); got != slug {
			t.Errorf("%s %d = %q, want %q", kind, id, got, slug)
		}
	}
}

var (
	bus1   = models.Route{RouteID: 1, RouteShortName: "24B", RouteType: models.Bus}
	bus2   = models.Route{RouteID: 2, RouteShortName: "28", RouteType: models.Bus}
	bus3   = models.Route{RouteID: 3, RouteShortName: "28.", RouteType: models.Bus}
	tram   = models.Route{RouteID: 4, RouteShortName: "9", RouteType: models.Tram}
	bus9   = models.Route{RouteID: 5, RouteShortName: "9.", RouteType: models.Bus}
	trolly = models.Route{RouteID: 6, RouteShortName: "M26", RouteType: models.Trolleybus}
	lines  = []models.Route{bus1, bus2, bus3, tram, bus9, trolly}
)

func TestNamesakesAreToldApart(t *testing.T) {
	s := newService(t)
	snap := snapshot(lines, []stop{
		{id: 10, station: 10, name: "Piața Unirii", lat: 46.77, lon: 23.59, routes: []int{1}},
		{id: 11, station: 10, name: "Piața Unirii", lat: 46.7702, lon: 23.5902, routes: []int{2}},
		{id: 20, station: 20, name: "Calea Florești", lat: 46.76, lon: 23.55, routes: []int{4, 2}},
		{id: 21, station: 21, name: "Calea Florești", lat: 46.761, lon: 23.551, routes: []int{6}},
		{id: 22, station: 22, name: "Calea Florești", lat: 46.75, lon: 23.54, routes: []int{6}},
		{id: 30, station: 30, name: "Gara", lat: 46.78, lon: 23.58, routes: []int{1, 2}},
		{id: 31, station: 31, name: "Gara", lat: 46.79, lon: 23.50, routes: []int{1}},
		{id: 40, station: 40, name: "Spitalul Recuperare - Sud", lat: 46.75, lon: 23.6, routes: []int{1}},
	})
	towns := func(points []feeds.Point) ([]string, []string, error) {
		out := make([]string, len(points))
		for i, p := range points {
			out[i] = "Cluj-Napoca"
			if p.Lon < 23.52 {
				out[i] = "Florești"
			}
		}
		return out, make([]string, len(points)), nil
	}
	if err := s.Refresh(snap, towns); err != nil {
		t.Fatal(err)
	}
	want(t, s.Index(), Station, map[int]string{
		10: "piata-unirii",
		20: "calea-floresti",
		21: "calea-floresti-troleibuz-nord",
		22: "calea-floresti-troleibuz-sud",
		30: "gara",
		31: "gara-floresti",
		40: "spitalul-recuperare-sud",
	})
	want(t, s.Index(), Line, map[int]string{1: "24b", 2: "28", 3: "28-2", 4: "9", 5: "9-autobuz", 6: "m26"})
}

func TestAddressesStay(t *testing.T) {
	s := newService(t)
	stops := []stop{
		{id: 10, station: 10, name: "Lipovei", lat: 47.05, lon: 21.92, routes: []int{1}},
		{id: 20, station: 20, name: "Observatorului", lat: 47.06, lon: 21.93, routes: []int{1}},
	}
	if err := s.Refresh(snapshot(lines, stops), nil); err != nil {
		t.Fatal(err)
	}
	// A busier namesake that comes later does not take the plain name.
	stops = append(stops, stop{id: 30, station: 30, name: "Lipovei", lat: 47.07, lon: 21.94, routes: []int{4, 2, 6}})
	stops[1].name = "Observatorului Nord"
	if err := s.Refresh(snapshot(lines, stops), nil); err != nil {
		t.Fatal(err)
	}
	x := s.Index()
	want(t, x, Station, map[int]string{10: "lipovei", 30: "lipovei-tramvai", 20: "observatorului-nord"})
	if id, ok := x.Find(Station, "observatorului"); !ok || id != 20 {
		t.Errorf("the old address points at %d, %v", id, ok)
	}

	// A station whose id is now another station's platform keeps its address.
	stops[0].station = 9
	stops = append(stops, stop{id: 9, station: 9, name: "Lipovei", lat: 47.05, lon: 21.92, routes: []int{1}})
	if err := s.Refresh(snapshot(lines, stops), nil); err != nil {
		t.Fatal(err)
	}
	want(t, s.Index(), Station, map[int]string{9: "lipovei", 30: "lipovei-tramvai"})
}
