package sources

import (
	"conexiuni-cluj/models"
	"strconv"
	"testing"
)

type testPlace struct {
	name  string
	town  string
	route int
	// One street per platform; two platforms are a stop's two sides.
	streets []string
}

// Each place sits on its own stretch of road, between two stops of other names about 400 m away.
func placesSnapshot(places []testPlace) (*Snapshot, Locate) {
	snap := &Snapshot{}
	where := map[Point][2]string{}
	add := func(name string, lat, lon float64) int {
		id := len(snap.Stops) + 1
		snap.Stops = append(snap.Stops, models.Stop{StopID: id, StopName: name, StopLat: lat, StopLon: lon})
		return id
	}
	call := func(trip string, route, direction int, stops ...int) {
		snap.Trips = append(snap.Trips, models.Trip{TripID: trip, RouteID: route, DirectionID: models.DirectionType(direction)})
		for i, s := range stops {
			snap.TripStops = append(snap.TripStops, models.TripStop{TripID: trip, StopID: s, StopSequence: i})
		}
	}
	routes := map[int]bool{}
	for k, p := range places {
		if !routes[p.route] {
			routes[p.route] = true
			snap.Routes = append(snap.Routes, models.Route{RouteID: p.route, RouteType: models.Bus})
		}
		lon := 24 + float64(k)*0.1
		west, east := add("West "+strconv.Itoa(k), 45, lon-0.005), add("East "+strconv.Itoa(k), 45, lon+0.005)
		for side, street := range p.streets {
			lat := 45 + float64(side)*0.0004
			stop := add(p.name, lat, lon)
			where[Point{lat, lon}] = [2]string{p.town, street}
			if side == 0 {
				call(strconv.Itoa(k)+"_0", p.route, 0, west, stop, east)
			} else {
				call(strconv.Itoa(k)+"_1", p.route, 1, east, stop, west)
			}
		}
	}
	locate := func(points []Point) ([]string, []string, error) {
		towns, streets := make([]string, len(points)), make([]string, len(points))
		for i, p := range points {
			towns[i], streets[i] = where[p][0], where[p][1]
		}
		return towns, streets, nil
	}
	return snap, locate
}

func setMode(snap *Snapshot, route int, mode models.RouteType) {
	for i := range snap.Routes {
		if snap.Routes[i].RouteID == route {
			snap.Routes[i].RouteType = mode
		}
	}
}

func namesOf(snap *Snapshot) map[string]int {
	out := map[string]int{}
	for _, s := range snap.Stops {
		out[s.StopName]++
	}
	return out
}

func expectNames(t *testing.T, snap *Snapshot, want map[string]int) {
	t.Helper()
	got := namesOf(snap)
	for name, count := range want {
		if got[name] != count {
			t.Errorf("%q: %d stops, want %d (all: %v)", name, got[name], count, got)
		}
	}
}

func TestAStationsPlatformsShareItsLowestStopID(t *testing.T) {
	snap, locate := placesSnapshot([]testPlace{{"Piata Romana", "București", 1, []string{"Bulevardul Magheru", "Strada Mendeleev"}}})
	if _, err := Stations(snap, locate); err != nil {
		t.Fatal(err)
	}
	stations := map[string][]int{}
	for _, s := range snap.Stops {
		stations[s.StopName] = append(stations[s.StopName], s.StationID)
	}
	if got := stations["Piata Romana"]; len(got) != 2 || got[0] != 3 || got[1] != 3 {
		t.Fatalf("platforms in stations %v", got)
	}
	if got := stations["West 0"]; len(got) != 1 || got[0] != 1 {
		t.Fatalf("a stop of its own name is its own station: %v", got)
	}
}

func TestTheSameNameAndModeInOtherTownsGetTheirTown(t *testing.T) {
	snap, locate := placesSnapshot([]testPlace{
		{"1 Decembrie 1918", "București", 1, []string{"Bulevardul Theodor Pallady", "Bulevardul Theodor Pallady"}},
		// Two platforms naming different streets are still one place, told apart by its town.
		{"1 Decembrie 1918", "Dărăști-Ilfov", 2, []string{"Strada 1 Decembrie 1918", "Strada Argeș"}},
		{"1 Decembrie 1918", "Tânganu", 3, []string{"Strada Gării"}},
		{"Gara", "București", 4, []string{"Calea Griviței"}},
		{"Gara", "Buftea", 5, []string{"Strada Gării"}},
	})
	// The metro is the only metro of its name; the buses share theirs.
	setMode(snap, 1, models.Subway)
	n, err := Stations(snap, locate)
	if err != nil {
		t.Fatal(err)
	}
	expectNames(t, snap, map[string]int{
		"1 Decembrie 1918": 2, "1 Decembrie 1918 (Dărăști-Ilfov)": 2, "1 Decembrie 1918 (Tânganu)": 1,
		"Gara (București)": 1, "Gara (Buftea)": 1,
	})
	if n != 5 {
		t.Errorf("renamed %d, want 5", n)
	}
}

func TestADifferentModeTellsApartStationsOfOneTown(t *testing.T) {
	snap, locate := placesSnapshot([]testPlace{
		{"1 Mai", "București", 1, []string{"Calea Griviței", "Calea Griviței"}},
		{"1 Mai", "București", 2, []string{"Bulevardul 1 Mai", "Bulevardul 1 Mai"}},
	})
	setMode(snap, 1, models.Subway)
	setMode(snap, 2, models.Trolleybus)
	if n, err := Stations(snap, locate); err != nil || n != 0 {
		t.Fatalf("renamed %d, %v", n, err)
	}
	expectNames(t, snap, map[string]int{"1 Mai": 4})
}

func TestTheSameModeInOneTownGetsTheStreet(t *testing.T) {
	snap, locate := placesSnapshot([]testPlace{
		{"Gara", "București", 1, []string{"Calea Griviței"}},
		{"Gara", "București", 2, []string{"Bulevardul Dinicu Golescu"}},
		{"Posta", "Buftea", 3, []string{"Strada Mihai Eminescu"}},
		{"Posta", "Buftea", 3, []string{"Strada Gării"}},
		{"Posta", "Otopeni", 4, []string{"Calea Bucureștilor"}},
	})
	if _, err := Stations(snap, locate); err != nil {
		t.Fatal(err)
	}
	expectNames(t, snap, map[string]int{
		"Gara (Calea Griviței)": 1, "Gara (Bulevardul Dinicu Golescu)": 1,
		"Posta (Strada Mihai Eminescu, Buftea)": 1, "Posta (Strada Gării, Buftea)": 1, "Posta (Otopeni)": 1,
	})
}

func TestATerminusTwinIsTheSameStation(t *testing.T) {
	snap, locate := placesSnapshot([]testPlace{{"Pasarela", "Popești-Leordeni", 1, []string{"Strada Unirii"}}, {"Pasarela", "Voluntari", 2, []string{"Strada Unirii"}}})
	// The bus calls at the arrival side, then at the departure side a metre away, so its next stop is a namesake.
	arrival := snap.Stops[2]
	twin := models.Stop{StopID: 100, StopName: "Pasarela", StopLat: arrival.StopLat, StopLon: arrival.StopLon + 0.00001}
	snap.Stops = append(snap.Stops, twin)
	snap.Trips = append(snap.Trips, models.Trip{TripID: "loop", RouteID: 1})
	snap.TripStops = append(snap.TripStops,
		models.TripStop{TripID: "loop", StopID: arrival.StopID, StopSequence: 0}, models.TripStop{TripID: "loop", StopID: twin.StopID, StopSequence: 1})
	base := locate
	locate = func(points []Point) ([]string, []string, error) {
		towns, streets, err := base(points)
		for i, p := range points {
			if p == (Point{twin.StopLat, twin.StopLon}) {
				towns[i], streets[i] = "Popești-Leordeni", "Strada Unirii"
			}
		}
		return towns, streets, err
	}
	if _, err := Stations(snap, locate); err != nil {
		t.Fatal(err)
	}
	expectNames(t, snap, map[string]int{"Pasarela (Popești-Leordeni)": 2, "Pasarela (Voluntari)": 1})
	if snap.Stops[len(snap.Stops)-1].StationID != arrival.StopID {
		t.Fatalf("twin in station %d, want %d", snap.Stops[len(snap.Stops)-1].StationID, arrival.StopID)
	}
}

func TestTimetableEndsAndRouteNamesFollow(t *testing.T) {
	snap, locate := placesSnapshot([]testPlace{
		{"Alpha", "Tunari", 7, []string{"Strada Gării"}},
		{"Alpha", "Rudeni", 8, []string{"Strada Panduri"}},
	})
	// Route 7 starts at its Alpha.
	snap.Trips = append(snap.Trips, models.Trip{TripID: "7_1", RouteID: 7, DirectionID: 1})
	snap.TripStops = append(snap.TripStops, models.TripStop{TripID: "7_1", StopID: 3, StopSequence: 0}, models.TripStop{TripID: "7_1", StopID: 1, StopSequence: 1})
	snap.Routes = []models.Route{{RouteID: 7, RouteShortName: "7", RouteLongName: "West 0 - Alpha", RouteType: models.Bus}, {RouteID: 8, RouteType: models.Bus}}
	snap.Timetables = map[string]*models.Timetable{"7": {RouteShortName: "7", InStopName: "West 0", OutStopName: "Alpha"}}
	if _, err := Stations(snap, locate); err != nil {
		t.Fatal(err)
	}
	if got := snap.Routes[0].RouteLongName; got != "West 0 - Alpha (Tunari)" {
		t.Errorf("route name %q", got)
	}
	if tt := snap.Timetables["7"]; tt.InStopName != "West 0" || tt.OutStopName != "Alpha (Tunari)" {
		t.Errorf("timetable ends %q / %q", tt.InStopName, tt.OutStopName)
	}
}

// One-way streets: each direction stops on its own street, further apart than the next stop.
func TestALinesTwoDirectionsAreOneStationWhereTheirStopsOverlap(t *testing.T) {
	snap := &Snapshot{}
	add := func(id int, name string, lon float64) {
		snap.Stops = append(snap.Stops, models.Stop{StopID: id, StopName: name, StopLat: 45, StopLon: lon})
	}
	// 1 Mai 700 m apart, each 500 m from its other stops; Gara in two villages 14 km apart.
	add(1, "West", 24.0)
	add(2, "1 Mai", 24.0064)
	add(3, "East", 24.0128)
	add(4, "1 Mai", 24.0153)
	add(5, "Far", 24.0217)
	add(6, "Gara", 24.2)
	add(7, "Gara", 24.38)
	add(8, "Field", 24.2064)
	add(9, "Hill", 24.3736)
	trip := func(id string, route, dir int, stops ...int) {
		snap.Trips = append(snap.Trips, models.Trip{TripID: id, RouteID: route, DirectionID: models.DirectionType(dir)})
		for i, s := range stops {
			snap.TripStops = append(snap.TripStops, models.TripStop{TripID: id, StopID: s, StopSequence: i})
		}
	}
	trip("1_0", 1, 0, 1, 2, 3)
	trip("1_1", 1, 1, 5, 4, 3)
	trip("2_0", 2, 0, 6, 8, 9)
	trip("2_1", 2, 1, 9, 7, 8)
	if _, err := Stations(snap, func(p []Point) ([]string, []string, error) { return make([]string, len(p)), make([]string, len(p)), nil }); err != nil {
		t.Fatal(err)
	}
	station := map[int]int{}
	for _, s := range snap.Stops {
		station[s.StopID] = s.StationID
	}
	if station[2] != station[4] {
		t.Errorf("1 Mai's two directions are stations %d and %d", station[2], station[4])
	}
	if station[6] == station[7] {
		t.Errorf("the two villages' Gara are one station")
	}
}
