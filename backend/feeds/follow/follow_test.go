package follow

import (
	"math"
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
	lon map[int]float64
}

func (t tracing) RouteIDs() map[string]int { return nil }

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

func TestABusTakesTheDirectionItMovesAlongAndTheFeedsTime(t *testing.T) {
	f := New(ids{}, tracing{lon: map[int]float64{1: 24.55}})
	at := time.Date(2026, 10, 24, 13, 33, 32, 0, time.UTC)
	var trips []string
	for _, lat := range []float64{46.56, 46.562, 46.564} {
		at = at.Add(10 * time.Second)
		vehicles, err := f.Vehicles([]Bus{{ID: "MS135", Label: "MS135", Lat: lat, Lon: 24.55, At: at, Lines: []Line{{RouteID: 1}}}}, at.Add(time.Minute))
		if err != nil {
			t.Fatal(err)
		}
		trip := ""
		for _, v := range vehicles {
			trip = v.TripID
			if v.Timestamp != at.Format(time.RFC3339) {
				t.Errorf("timestamp %s, want the feed's %s", v.Timestamp, at.Format(time.RFC3339))
			}
		}
		trips = append(trips, trip)
	}
	if trips[0] != "" || trips[1] != "1_0" || trips[2] != "1_0" {
		t.Fatalf("trips %v, want unplaced until it moves, then north on 1_0", trips)
	}
}
