package live

import (
	"testing"

	"conexiuni-cluj/predictions"
)

func TestOnDrawnLine(t *testing.T) {
	var points []predictions.Point
	for i := range 11 {
		points = append(points, predictions.Point{Lat: 46.70 + float64(i)*0.001, Lon: 23.6})
	}
	stops := []predictions.Stop{
		{ID: 1, Lat: points[0].Lat, Lon: points[0].Lon},
		{ID: 2, Lat: points[2].Lat, Lon: points[2].Lon},
		{ID: 3, Lat: points[8].Lat, Lon: points[8].Lon},
		{ID: 4, Lat: points[10].Lat, Lon: points[10].Lon},
	}
	trip := predictions.NewTrip("t", predictions.NewShapeIndex(points), stops, nil)
	ends := func(id int) bool { return id == 1 || id == 4 }
	cases := []struct {
		about    string
		shapeIdx int
		hidden   func(int) bool
		want     bool
	}{
		{"at the hidden start", 0, ends, false},
		{"at the first visible stop", 2, ends, true},
		{"between visible stops", 5, ends, true},
		{"at the last visible stop", 8, ends, true},
		{"at the hidden end", 10, ends, false},
		{"nothing hidden", 0, func(int) bool { return false }, true},
		{"a hidden stop in the middle", 5, func(id int) bool { return id == 2 }, true},
	}
	for _, c := range cases {
		if got := onDrawnLine(trip, c.shapeIdx, c.hidden); got != c.want {
			t.Errorf("%s: got %v", c.about, got)
		}
	}
}
