package targumures

import (
	"fmt"
	"math"
	"os"
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
	along := (lat - 46.5) * metersPerDegree
	if direction == 1 {
		along = 10_000 - along
	}
	return along, math.Abs(lon-lineLon) * metersPerDegree * math.Cos(lat*math.Pi/180), true
}

func TestReadsTheLiveMap(t *testing.T) {
	data, err := os.ReadFile("testdata/journeys.json")
	if err != nil {
		t.Fatal(err)
	}
	buses, err := parseJourneys(data, map[string]int{"11": 1, "30B": 2, "7": 3})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]int{"MS135": 1, "MS101": 2, "MS320": 3}
	if len(buses) != len(want) {
		t.Fatalf("%d buses, want %d (lines 0 and empty are out of service): %+v", len(buses), len(want), buses)
	}
	for _, b := range buses {
		if len(b.Lines) != 1 || b.Lines[0].RouteID != want[b.ID] {
			t.Errorf("%s on %+v, want route %d", b.ID, b.Lines, want[b.ID])
		}
		if b.At.IsZero() || b.Lat == 0 || b.Lon == 0 {
			t.Errorf("%s: %+v", b.ID, b)
		}
	}
}

func TestABusTakesTheDirectionItMovesAlongAndTheMapsTime(t *testing.T) {
	s := newRealtime("", ids{}, tracing{routes: map[string]int{"11": 1}, lon: map[int]float64{1: 24.55}})
	at := time.Date(2026, 10, 24, 13, 33, 32, 0, time.UTC)
	var trips []string
	for _, lat := range []float64{46.56, 46.562, 46.564} {
		at = at.Add(10 * time.Second)
		data := fmt.Sprintf(`{"journeys":[{"vehicle_id":"MS135","route":"11","lat":%v,"lon":24.55,"speed":null,"direction":null,"timestamp":%q}]}`, lat, at.Format(time.RFC3339Nano))
		buses, err := parseJourneys([]byte(data), s.trace.RouteIDs())
		if err != nil {
			t.Fatal(err)
		}
		vehicles, err := s.follower.Vehicles(buses, at.Add(time.Minute))
		if err != nil {
			t.Fatal(err)
		}
		trip := ""
		for _, v := range vehicles {
			trip = v.TripID
			if v.Timestamp != at.Format(time.RFC3339) {
				t.Errorf("timestamp %s, want the map's %s", v.Timestamp, at.Format(time.RFC3339))
			}
		}
		trips = append(trips, trip)
	}
	if trips[0] != "" || trips[1] != "1_0" || trips[2] != "1_0" {
		t.Fatalf("trips %v, want unplaced until it moves, then north on 1_0", trips)
	}
}
