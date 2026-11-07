package pitesti

import (
	"conexiuni-cluj/calendar"
	"conexiuni-cluj/feeds/sitegtfs"
	"conexiuni-cluj/router"
	"slices"
	"testing"
)

// Stops about a kilometre apart; the last is off the map unless placed.
func straight(n int, placed bool) []sitegtfs.Stop {
	out := make([]sitegtfs.Stop, n)
	for i := range out {
		out[i] = sitegtfs.Stop{ID: string(rune('a' + i)), At: router.Point{Lat: 44.85 + float64(i)/100, Lon: 24.87}, Placed: placed || i < n-1}
	}
	return out
}

func TestReach(t *testing.T) {
	if got := reach(straight(3, true), []int{600, 610}); !slices.Equal(got, []int{0, 10, 20}) {
		t.Errorf("untimed end %v", got)
	}
	if got := reach(straight(3, false), []int{600, 610}); !slices.Equal(got, []int{0, 10, 10}) {
		t.Errorf("unplaced end %v", got)
	}
	if got := reach(straight(3, true), []int{1430, 1442, 1450}); !slices.Equal(got, []int{0, 12, 20}) {
		t.Errorf("timed %v", got)
	}
}

func TestRoutes(t *testing.T) {
	out := way{stops: straight(2, true), runs: map[string][][]int{
		calendar.Weekday:  {{600, 605}, {660, 666}},
		calendar.Saturday: {{700, 705}},
	}}
	back := way{stops: straight(2, true), runs: map[string][][]int{calendar.Sunday: {{900, 904}}}}
	routes := out.routes("20-0")
	if len(routes) != 2 || routes[0].ID != "20-0-0" || !slices.Equal(routes[0].Minutes, []int{0, 5}) || !slices.Equal(routes[1].Minutes, []int{0, 6}) {
		t.Fatalf("routes %+v", routes)
	}
	want := []sitegtfs.Departure{{Day: calendar.Weekday, Minute: 600}, {Day: calendar.Saturday, Minute: 700}}
	if !slices.Equal(routes[0].Departures, want) {
		t.Errorf("departures %+v", routes[0].Departures)
	}
	lines := pair("2B", [2][]*sitegtfs.Route{routes, back.routes("20-1")})
	if len(lines) != 2 || lines[0].Name != "2B" || lines[1].Name != "2B" || lines[0].Ways[1] == nil || lines[1].Ways[1] != nil || lines[1].Ways[0] != routes[1] {
		t.Errorf("lines %+v", lines)
	}
}
