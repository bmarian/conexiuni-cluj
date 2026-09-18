package handlers

import (
	"conexiuni-cluj/database"
	"conexiuni-cluj/models"
	"reflect"
	"testing"
)

func minutesOf(t *testing.T, times ...string) []int {
	t.Helper()
	out := make([]int, len(times))
	for i, s := range times {
		m, ok := parseDepartureMinutes(s)
		if !ok {
			t.Fatalf("bad time %q", s)
		}
		out[i] = m
	}
	return out
}

func TestAlignDeparturesShiftsWholeTimetable(t *testing.T) {
	before := minutesOf(t, "06:00", "06:10", "06:20", "06:30")
	after := minutesOf(t, "06:05", "06:15", "06:25", "06:35")

	shifts, removed, added := alignDepartures(before, after)
	if len(shifts) != 4 || len(removed) != 0 || len(added) != 0 {
		t.Fatalf("want 4 shifts only, got shifts=%v removed=%v added=%v", shifts, removed, added)
	}
	for _, s := range shifts {
		if s[1]-s[0] != 5 {
			t.Fatalf("want +5 shifts, got %v", shifts)
		}
	}
}

func TestAlignDeparturesPrefersExactMatchesOverShifts(t *testing.T) {
	before := minutesOf(t, "07:00", "07:05", "07:10", "07:15")
	after := minutesOf(t, "07:00", "07:10", "07:15")

	shifts, removed, added := alignDepartures(before, after)
	if len(shifts) != 0 || len(added) != 0 || !reflect.DeepEqual(removed, minutesOf(t, "07:05")) {
		t.Fatalf("want only 07:05 removed, got shifts=%v removed=%v added=%v", shifts, removed, added)
	}
}

func TestAlignDeparturesLargeMoveIsRemoveAndAdd(t *testing.T) {
	before := minutesOf(t, "08:00", "09:00")
	after := minutesOf(t, "08:40", "09:00")

	shifts, removed, added := alignDepartures(before, after)
	if len(shifts) != 0 || !reflect.DeepEqual(removed, minutesOf(t, "08:00")) || !reflect.DeepEqual(added, minutesOf(t, "08:40")) {
		t.Fatalf("got shifts=%v removed=%v added=%v", shifts, removed, added)
	}
}

func dayOf(pairs ...[2]string) models.DaySchedule {
	d := models.DaySchedule{ServiceStart: "07.09.2026"}
	for _, p := range pairs {
		d.Entries = append(d.Entries, models.TimetableEntry{DepartureIn: p[0], DepartureOut: p[1]})
	}
	return d
}

func TestDiffTimetablesReportsEachKindPerDirection(t *testing.T) {
	previous := &models.Timetable{
		InStopName: "A", OutStopName: "B",
		Weekdays: dayOf([2]string{"06:00", "06:30"}, [2]string{"07:00", "07:30"}, [2]string{"08:00", "08:30"}),
	}
	current := &models.Timetable{
		InStopName: "A", OutStopName: "B",
		Weekdays: dayOf([2]string{"06:03", "06:30"}, [2]string{"07:00", "07:28"}, [2]string{"08:00", ""}, [2]string{"09:00", ""}),
	}

	items := diffTimetables(previous, current)
	got := map[string]RouteChangeItem{}
	for _, it := range items {
		got[it.Kind+"/"+it.Direction] = it
	}

	if it := got["later/0"]; !reflect.DeepEqual(it.Shifts, []TimeShift{{"06:00", "06:03"}}) || it.Toward != "B" {
		t.Fatalf("later/0 = %+v", it)
	}
	if it := got["trips_added/0"]; !reflect.DeepEqual(it.Times, []string{"09:00"}) {
		t.Fatalf("trips_added/0 = %+v", it)
	}
	if it := got["earlier/1"]; !reflect.DeepEqual(it.Shifts, []TimeShift{{"07:30", "07:28"}}) || it.Toward != "A" {
		t.Fatalf("earlier/1 = %+v", it)
	}
	if it := got["trips_removed/1"]; !reflect.DeepEqual(it.Times, []string{"08:30"}) {
		t.Fatalf("trips_removed/1 = %+v", it)
	}
	if len(items) != 4 {
		t.Fatalf("want 4 items, got %+v", items)
	}
}

func TestDiffTimetablesIgnoresSwappedColumnsAndMissingDays(t *testing.T) {
	previous := &models.Timetable{
		InStopName: "A", OutStopName: "B",
		Weekdays: dayOf([2]string{"06:00", "06:30"}),
		Saturday: dayOf([2]string{"10:00", "10:30"}),
	}
	current := &models.Timetable{
		InStopName: "B", OutStopName: "A",
		Weekdays: dayOf([2]string{"06:30", "06:00"}),
	}

	if items := diffTimetables(previous, current); len(items) != 0 {
		t.Fatalf("want no changes, got %+v", items)
	}
}

func TestDiffTimetablesReportsNewDayOfService(t *testing.T) {
	previous := &models.Timetable{
		InStopName: "A", OutStopName: "B",
		Saturday: models.DaySchedule{ServiceName: "Sambata", ServiceStart: "23.08.2025"},
	}
	current := &models.Timetable{
		InStopName: "A", OutStopName: "B",
		Saturday: dayOf([2]string{"10:00", ""}),
	}

	items := diffTimetables(previous, current)
	want := []RouteChangeItem{{
		Kind: changeTripsAdded, Direction: "0", Toward: "B", Days: []string{"saturday"},
		Since: "07.09.2026", All: true, Times: []string{"10:00"},
	}}
	if !reflect.DeepEqual(items, want) {
		t.Fatalf("got %+v", items)
	}
}

func TestDiffTimetablesSkipsHeadwayDirections(t *testing.T) {
	previous := &models.Timetable{InStopName: "A", OutStopName: "B", Weekdays: dayOf([2]string{"06:00", "06:30"})}
	current := &models.Timetable{InStopName: "A", OutStopName: "B", Weekdays: dayOf([2]string{"06:00", ""})}
	current.Weekdays.OutFrequency = &models.Frequency{Start: "05:00", End: "23:00", MinMinutes: 10, MaxMinutes: 15}

	if items := diffTimetables(previous, current); len(items) != 0 {
		t.Fatalf("want no changes, got %+v", items)
	}
}

func TestDiffTimetablesMergesIdenticalWeekendChanges(t *testing.T) {
	previous := &models.Timetable{
		InStopName: "A", OutStopName: "B",
		Saturday: dayOf([2]string{"10:00", "10:30"}),
		Sunday:   dayOf([2]string{"10:00", "10:30"}),
	}
	current := &models.Timetable{
		InStopName: "A", OutStopName: "B",
		Saturday: dayOf([2]string{"10:00", "10:30"}, [2]string{"11:00", ""}),
		Sunday:   dayOf([2]string{"10:00", "10:30"}, [2]string{"11:00", ""}),
	}

	items := diffTimetables(previous, current)
	if len(items) != 1 || !reflect.DeepEqual(items[0].Days, []string{"saturday", "sunday"}) {
		t.Fatalf("want one merged item, got %+v", items)
	}
}

func stopsFor(tripID string, stops ...int) []models.StopTime {
	out := make([]models.StopTime, len(stops))
	for i, id := range stops {
		out[i] = models.StopTime{TripID: tripID, StopID: id, StopSequence: i, StopHeadsign: "S" + itoa(int32(id))}
	}
	return out
}

func TestDiffRouteStops(t *testing.T) {
	previous := append(stopsFor("25_0", 1, 2, 3, 4, 5), stopsFor("25_1", 5, 4, 3, 2, 1)...)
	current := append(stopsFor("25_0", 1, 2, 9, 4, 5), stopsFor("25_1", 5, 3, 4, 2, 1)...)

	items := diffRouteStops(previous, current)
	want := []RouteChangeItem{
		{Kind: changeStopsRemoved, Direction: "0", Toward: "S5", Stops: []string{"S3"}},
		{Kind: changeStopsAdded, Direction: "0", Toward: "S5", Stops: []string{"S9"}},
	}
	if !reflect.DeepEqual(items, want) {
		t.Fatalf("got %+v", items)
	}
}

func TestDiffRouteStopsSkipsRewrittenRoutes(t *testing.T) {
	previous := stopsFor("25_0", 1, 2, 3, 4, 5, 6)
	current := stopsFor("25_0", 1, 7, 8, 9, 10, 6)

	if items := diffRouteStops(previous, current); len(items) != 0 {
		t.Fatalf("want nothing for a mostly different stop list, got %+v", items)
	}
}

func TestStoreStopTimesDropsStaleTail(t *testing.T) {
	withTestDB(t)

	if err := storeStopTimesTrackingChanges("25")(stopsFor("25_0", 1, 2, 3, 4)); err != nil {
		t.Fatal(err)
	}
	if err := storeStopTimesTrackingChanges("25")(stopsFor("25_0", 1, 2, 4)); err != nil {
		t.Fatal(err)
	}

	stored, err := getStopTimesForTrips([]string{"25_0"})
	if err != nil {
		t.Fatal(err)
	}
	if len(stored) != 3 {
		t.Fatalf("want 3 stops after shrinking the trip, got %+v", stored)
	}

	var count int
	var changes string
	if err := database.DB.QueryRow(`SELECT COUNT(*), MAX(changes) FROM route_changes WHERE route_short_name = '25'`).Scan(&count, &changes); err != nil {
		t.Fatal(err)
	}
	if count != 1 || changes != `[{"kind":"stops_removed","direction":"0","toward":"S4","stops":["S3"]}]` {
		t.Fatalf("got %d rows: %s", count, changes)
	}

	if err := storeStopTimesTrackingChanges("25")(stopsFor("25_0", 1, 2, 3, 4)); err != nil {
		t.Fatal(err)
	}
	if err := storeStopTimesTrackingChanges("25")(stopsFor("25_0", 1, 2, 4)); err != nil {
		t.Fatal(err)
	}
	if err := database.DB.QueryRow(`SELECT COUNT(*) FROM route_changes WHERE route_short_name = '25'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("flapping data should not repeat the same news, got %d rows", count)
	}
}
