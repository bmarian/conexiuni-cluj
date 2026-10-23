package destinations

import (
	"conexiuni-cluj/models"
	"conexiuni-cluj/router"
	"reflect"
	"sort"
	"testing"
)

func hours(at int, offsets []int) [][]int {
	out := make([][]int, 24)
	out[at] = offsets
	return out
}

// Two districts on line 5, a village one line further, and the city itself, farther from every stop than its districts.
func testIndex() *Index {
	bundle := &models.NetworkBundle{
		Stops: []models.Stop{
			{StopID: 1, StopName: "Unu", StopLat: 0, StopLon: 0.001},
			{StopID: 2, StopName: "Doi", StopLat: 0, StopLon: 0.003},
			{StopID: 3, StopName: "Trei", StopLat: 0, StopLon: 0.019},
			{StopID: 4, StopName: "Patru", StopLat: 0, StopLon: 0.021},
			{StopID: 5, StopName: "Cinci", StopLat: 0.05, StopLon: 0.001},
		},
		Routes: []models.NetworkRoute{
			{
				RouteID: 10, RouteShortName: "5", RouteLongName: "Alfa - Beta",
				Timetable: &models.Timetable{RouteLongName: "Alfa - Beta", Weekdays: models.DaySchedule{Entries: []models.TimetableEntry{
					{DepartureIn: "08:00", DepartureOut: "08:05"}, {DepartureIn: "08:10"}, {DepartureIn: "08:20"},
				}}},
				Trips:   []models.NetworkTrip{{TripID: "10_0", StopIDs: []int{1, 2, 3, 4}}, {TripID: "10_1", StopIDs: []int{4, 3, 2, 1}}},
				Offsets: map[string][][]int{"weekday": hours(8, []int{60, 120, 300, 120, 0, 120, 300, 120})},
			},
			{
				RouteID: 11, RouteShortName: "M1", RouteLongName: "Beta - Vale",
				Timetable: &models.Timetable{RouteLongName: "Beta - Vale", Saturday: models.DaySchedule{Entries: []models.TimetableEntry{
					{DepartureIn: "09:00", DepartureOut: "09:30"},
				}}},
				Trips:   []models.NetworkTrip{{TripID: "11_0", StopIDs: []int{4, 5}}, {TripID: "11_1", StopIDs: []int{5, 4}}},
				Offsets: map[string][][]int{"saturday": hours(9, []int{0, 600, 0, 600})},
			},
		},
	}
	areas := []router.Area{
		{Name: "Așa", Lat: 0, Lon: 0, Place: "suburb"},
		{Name: "Bună", Lat: 0, Lon: 0.02, Place: "neighbourhood"},
		{Name: "Orașul", Lat: 0, Lon: 0.01, Place: "city"},
		{Name: "Vale", Lat: 0.05, Lon: 0, Place: "village"},
	}
	return build(bundle, areas)
}

func TestDestinations(t *testing.T) {
	x := testIndex()
	var slugs []string
	for _, d := range x.Destinations() {
		slugs = append(slugs, d.Slug)
	}
	if want := []string{"asa", "buna", "vale"}; !reflect.DeepEqual(slugs, want) {
		t.Fatalf("slugs = %v, want %v", slugs, want)
	}
	if got := x.AreaOf(1); got == nil || got.Slug != "asa" {
		t.Fatalf("stop 1 is in %v, want asa", got)
	}
	if got := x.Destination("buna").RouteIDs; !reflect.DeepEqual(got, []int{10, 11}) {
		t.Fatalf("lines at buna = %v", got)
	}
}

func TestRidesByOrigin(t *testing.T) {
	x := testIndex()
	cases := []struct {
		to, from string
		want     Ride
	}{
		// From the last stop of Așa to the first of Bună, timed from the 08:00 run on.
		{"buna", "asa", Ride{RouteID: 10, Direction: 0, Board: 2, Alight: 3, Minutes: [2]int{5, 5}, EveryMin: 10, First: "08:03", Last: "08:23", DayType: "weekday"}},
		// A single run: no gap.
		{"asa", "buna", Ride{RouteID: 10, Direction: 1, Board: 3, Alight: 2, Minutes: [2]int{5, 5}, First: "08:07", Last: "08:07", DayType: "weekday"}},
		// No weekday service: the first kind of day it runs on.
		{"vale", "buna", Ride{RouteID: 11, Direction: 0, Board: 4, Alight: 5, Minutes: [2]int{10, 10}, First: "09:00", Last: "09:00", DayType: "saturday"}},
	}
	for _, c := range cases {
		var origin *Origin
		for i, o := range x.Destination(c.to).From {
			if o.Slug == c.from {
				origin = &x.Destination(c.to).From[i]
			}
		}
		if origin == nil || len(origin.Rides) != 1 {
			t.Fatalf("%s: origins = %+v, want one ride from %s", c.to, x.Destination(c.to).From, c.from)
		}
		if got := origin.Rides[0]; got != c.want {
			t.Errorf("%s from %s = %+v, want %+v", c.to, c.from, got, c.want)
		}
	}
	// The origin with more stops first.
	if from := x.Destination("buna").From; len(from) != 2 || from[0].Slug != "asa" || from[1].Slug != "vale" {
		t.Errorf("origins of buna = %+v", from)
	}
}

func TestStationLines(t *testing.T) {
	x := testIndex()
	want := []Departing{{Toward: "Beta", Lines: []Line{{10, 0}}}, {Toward: "Alfa", Lines: []Line{{10, 1}}}}
	if got := x.StationLines(3); !reflect.DeepEqual(got, want) {
		t.Errorf("lines at 3 = %+v, want %+v", got, want)
	}
	// A line ending here that also leaves from here is listed once, by where it leaves for.
	want = []Departing{{Toward: "Beta", Lines: []Line{{11, 1}}}}
	if got := x.StationLines(5); !reflect.DeepEqual(got, want) {
		t.Errorf("lines at 5 = %+v, want %+v", got, want)
	}
	var stops []int
	for _, s := range x.RouteStops(10, 1) {
		stops = append(stops, s.StopID)
	}
	if !reflect.DeepEqual(stops, []int{4, 3, 2, 1}) {
		t.Errorf("route 10 back = %v", stops)
	}
}

// Where OSM names no district, the stops in town are in the town itself.
func TestTownWithoutDistricts(t *testing.T) {
	bundle := &models.NetworkBundle{
		Stops:  []models.Stop{{StopID: 1, StopLat: 0, StopLon: 0.001}, {StopID: 2, StopLat: 0, StopLon: 0.004}, {StopID: 3, StopLat: 0, StopLon: 0.03}},
		Routes: []models.NetworkRoute{{RouteID: 1, RouteShortName: "1", Trips: []models.NetworkTrip{{TripID: "1_0", StopIDs: []int{1, 2, 3}}}}},
	}
	areas := []router.Area{{Name: "Orașul", Lat: 0, Lon: 0, Place: "city"}, {Name: "Satul", Lat: 0, Lon: 0.03, Place: "village"}}
	x := build(bundle, areas)
	if a, b, c := x.AreaOf(1), x.AreaOf(2), x.AreaOf(3); a == nil || a.Slug != "orasul" || b != a || c == nil || c.Slug != "satul" {
		t.Errorf("areas = %v %v %v, want orasul orasul satul", a, b, c)
	}
}

func TestAreasWait(t *testing.T) {
	x := build(&models.NetworkBundle{Stops: []models.Stop{{StopID: 1}}}, nil)
	if len(x.Destinations()) != 0 || x.AreaOf(1) != nil {
		t.Error("destinations before the place index is read")
	}
}

func TestLineLess(t *testing.T) {
	names := []string{"M11", "24B", "5", "TE10", "TE3", "24", "5B"}
	sort.Slice(names, func(i, j int) bool { return lineLess(names[i], names[j]) })
	if want := []string{"5", "5B", "24", "24B", "M11", "TE3", "TE10"}; !reflect.DeepEqual(names, want) {
		t.Errorf("sorted = %v, want %v", names, want)
	}
}
