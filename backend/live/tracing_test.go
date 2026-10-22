package live

import (
	"conexiuni-cluj/predictions"
	"math"
	"testing"
)

func TestAlongMeasuresAPointOnADirectionsShape(t *testing.T) {
	eng := newTestCity(t)
	shape := predictions.NewShapeIndex([]predictions.Point{{Lat: 47.60, Lon: 26.25}, {Lat: 47.61, Lon: 26.25}, {Lat: 47.62, Lon: 26.25}})
	eng.liveTrips.byTrip = map[string]*liveTrip{"7_0": {trip: predictions.NewTrip("7_0", shape, []predictions.Stop{{ID: 1, Lat: 47.60, Lon: 26.25}, {ID: 2, Lat: 47.62, Lon: 26.25}}, nil), routeID: 7}}
	eng.liveTrips.routeIDs = map[string]int{"M7": 7}

	along, off, ok := eng.Along(7, 0, 47.61, 26.2513)
	if !ok || math.Abs(along-shape.Cumulative[1]) > 1e-9 || math.Abs(off-100) > 5 {
		t.Fatalf("along %.0f off %.0f ok %v", along, off, ok)
	}
	if _, _, ok := eng.Along(7, 1, 47.61, 26.25); ok {
		t.Fatal("a direction without a shape answered")
	}
	if eng.RouteIDs()["M7"] != 7 {
		t.Fatalf("route ids %v", eng.RouteIDs())
	}
}
