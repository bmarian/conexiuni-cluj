package suceava

import (
	"math"
	"os"
	"slices"
	"testing"
	"time"
)

type ids map[string]int

func (m ids) ExternalID(kind, external string) (int, error) {
	key := kind + ":" + external
	if _, ok := m[key]; !ok {
		m[key] = len(m) + 1
	}
	return m[key], nil
}

// Each line a straight road along its meridian: north on direction 0, south on 1.
type tracing struct {
	routes map[string]int
	lon    map[int]float64
}

func (t tracing) RouteIDs() map[string]int { return t.routes }

func (t tracing) Along(routeID, direction int, lat, lon float64) (float64, float64, bool) {
	lineLon, ok := t.lon[routeID]
	if !ok {
		return 0, 0, false
	}
	const metersPerDegree = 111_195.0
	along := (lat - 47.6) * metersPerDegree
	if direction == 1 {
		along = 10_000 - along
	}
	return along, math.Abs(lon-lineLon) * metersPerDegree * math.Cos(lat*math.Pi/180), true
}

var testRoutes = map[string]int{"M104": 1, "M10": 2, "M100": 3, "5": 4}

func newTestRealtime() *realtime {
	return newRealtime("", ids{}, tracing{routes: testRoutes, lon: map[int]float64{1: 26.25, 2: 26.30, 3: 26.35, 4: 26.25}})
}

// The trip of each step's one bus, "" while it is not placed.
func followBus(t *testing.T, s *realtime, plate, label string, lon float64, lats ...float64) []string {
	t.Helper()
	at := time.Unix(1_800_000_000, 0)
	var out []string
	for _, lat := range lats {
		at = at.Add(20 * time.Second)
		vehicles, err := s.vehicles([]sighting{{plate: plate, lat: lat, lon: lon, label: label}}, at)
		if err != nil {
			t.Fatal(err)
		}
		trip := ""
		for _, v := range vehicles {
			trip = v.TripID
		}
		out = append(out, trip)
	}
	return out
}

func TestReadsTheLiveMap(t *testing.T) {
	data, err := os.ReadFile("testdata/buses.json")
	if err != nil {
		t.Fatal(err)
	}
	buses, err := parseBuses(data)
	if err != nil {
		t.Fatal(err)
	}
	// Buses with contact on and a position; the rest are parked.
	if len(buses) != 63 || buses[0].plate != "SV18PSV" || buses[0].label != "M117" || buses[0].lat != 47.63762 {
		t.Fatalf("%d buses, first %+v", len(buses), buses[0])
	}
}

func TestABusTakesTheDirectionItMovesAlong(t *testing.T) {
	// Both directions fit a bus that has not moved; then it heads north, stands, and turns back south.
	got := followBus(t, newTestRealtime(), "SV1", "M104", 26.25, 47.65, 47.652, 47.6521, 47.649)
	if want := []string{"", "1_0", "1_0", "1_1"}; !slices.Equal(got, want) {
		t.Fatalf("trips %v, want %v", got, want)
	}
}

func TestABusCreepingAFewMetresAPollHasMoved(t *testing.T) {
	// 0.0001° is 11 m, under GPS jitter, but three of them are not.
	got := followBus(t, newTestRealtime(), "SV4", "M104", 26.25, 47.65, 47.6501, 47.6502, 47.6503)
	if want := []string{"", "", "", "1_0"}; !slices.Equal(got, want) {
		t.Fatalf("trips %v, want %v", got, want)
	}
}

func TestABusStandingAtATerminusLeavesOnTheDirectionStartingThere(t *testing.T) {
	// 47.6 is where direction 0 starts; direction 1 ends 10 km on, which the test road's direction 1 measures from.
	if got := followBus(t, newTestRealtime(), "SV5", "M104", 26.25, 47.6001); got[0] != "1_0" {
		t.Fatalf("trips %v", got)
	}
}

func TestACutLabelFallsBackToTheLineItBegins(t *testing.T) {
	if got := lineCandidates("  5", testRoutes); len(got) != 1 || got[0].RouteID != 4 {
		t.Fatalf("padded label %v", got)
	}
	// M100 and M104 both begin with it.
	if got := lineCandidates("M10", testRoutes); len(got) != 3 || got[0].RouteID != 2 || got[0].Cut || got[1].RouteID != 3 || !got[1].Cut || got[2].RouteID != 1 || !got[2].Cut {
		t.Fatalf("label filling the display %v", got)
	}
	if got := followBus(t, newTestRealtime(), "SV2", "M10", 26.35, 47.65, 47.652); got[1] != "3_0" {
		t.Fatalf("bus showing M10 on the road of M100 is on %v", got)
	}
	if got := followBus(t, newTestRealtime(), "SV3", "M10", 26.30, 47.65, 47.652); got[1] != "2_0" {
		t.Fatalf("bus showing M10 on its own road is on %v", got)
	}
}
