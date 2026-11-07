package satumare

import (
	"conexiuni-cluj/calendar"
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/models"
	"slices"
	"testing"
)

// A zip-like snapshot: each line one route, its directions starting at the stops named.
func snapshot(starts map[string][2]string, timetables map[string]*models.Timetable) *feeds.Snapshot {
	snap := &feeds.Snapshot{Timetables: timetables}
	for line, ends := range starts {
		routeID := len(snap.Routes) + 1
		snap.Routes = append(snap.Routes, models.Route{RouteID: routeID, RouteShortName: line})
		for dir, name := range ends {
			if name == "" {
				continue
			}
			stopID := len(snap.Stops) + 1
			trip := models.PatternID(routeID, dir)
			snap.Stops = append(snap.Stops, models.Stop{StopID: stopID, StopName: name})
			snap.Trips = append(snap.Trips, models.Trip{TripID: trip, RouteID: routeID, DirectionID: models.DirectionType(dir)})
			snap.TripStops = append(snap.TripStops, models.TripStop{TripID: trip, StopID: stopID})
		}
	}
	return snap
}

func TestLay(t *testing.T) {
	routes := map[string]route{}
	for _, r := range line10(t) {
		routes[r.label] = r
	}
	routes["21"] = route{label: "21", from: [2]string{"Sediul Transurban"}, times: [2]map[string][]string{{calendar.Weekday: {"23:30", "00:10"}}}}
	zip10C := &models.Timetable{RouteShortName: "10C"}
	zip7IIB := &models.Timetable{RouteShortName: "7II B"}
	snap := snapshot(map[string][2]string{
		"10":    {"Auchan", "Ambudului"},
		"10C":   {"Ambudului", "Auchan"},
		"10D":   {"Ambudului", "Spicului Wilburg"},
		"21":    {"Sediul Transurban"},
		"7II B": {"Spicului Wilburg"},
		"7C":    {"Sediul Transurban", "Spicului Wilburg"},
	}, map[string]*models.Timetable{"10C": zip10C, "7II B": zip7IIB})

	out := lay(snap, routes)
	if len(out) != 5 || out["10C"] != zip10C || out["7II B"] != zip7IIB {
		t.Fatalf("timetables %v", out)
	}
	// The zip runs line 10 from Auchan first, so the site's columns swap.
	ten := out["10"]
	if ten.InStopName != "Auchan" || len(ten.Weekdays.Entries) != 14 || len(ten.Saturday.Entries) != 16 ||
		ten.Weekdays.Entries[0] != (models.TimetableEntry{DepartureIn: "05:50", DepartureOut: "05:30"}) {
		t.Fatalf("10: %q, %+v", ten.InStopName, ten.Weekdays.Entries)
	}
	if got := out["10D"].Sunday.Entries; !slices.Equal(got, []models.TimetableEntry{{DepartureIn: "21:30", DepartureOut: "22:30"}}) {
		t.Fatalf("10D Sunday %+v", got)
	}
	if got := out["21"].Weekdays.Entries; !slices.Equal(got, []models.TimetableEntry{{DepartureIn: "23:30"}, {DepartureIn: "24:10"}}) {
		t.Fatalf("21 %+v", got)
	}
}
