package sitegtfs

import (
	"conexiuni-cluj/calendar"
	"conexiuni-cluj/feeds/gtfs"
	"conexiuni-cluj/models"
	"conexiuni-cluj/router"
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

type school bool

func (s school) SchoolDay(time.Time) bool { return bool(s) }

func route(id string, stations ...string) Route {
	r := Route{ID: id}
	for i, name := range stations {
		r.Stops = append(r.Stops, Stop{ID: name, Name: name, At: router.Point{Lat: 46.07 + float64(i)/100, Lon: 23.57}, Placed: true})
	}
	return r
}

func timetable(t *testing.T, lines []Line, term school) *models.Timetable {
	t.Helper()
	data, err := Zip(lines, map[string][]router.Point{}, term, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	snap, err := gtfs.Built("test", func() ([]byte, error) { return data, nil }, 0, ids{}).Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Timetables) != 1 {
		t.Fatalf("%d timetables", len(snap.Timetables))
	}
	for _, tt := range snap.Timetables {
		return tt
	}
	return nil
}

func column(s models.DaySchedule, in bool) []string {
	var times []string
	for _, e := range s.Entries {
		v := e.DepartureOut
		if in {
			v = e.DepartureIn
		}
		if v != "" {
			times = append(times, v)
		}
	}
	return times
}

func TestZip(t *testing.T) {
	out := route("110_M1", "Gara (Penny)", "Ampoi 3", "MICESTI-Labului")
	back := route("110R_M2", "MICESTI-Labului", "Ampoi 3", "Gara CFR")
	out.Minutes, back.Minutes = []int{0, 4, 9}, []int{0, 5, 9}
	out.Departures = []Departure{
		{Day: calendar.Weekday, Minute: 7 * 60}, {Day: calendar.Weekday, Minute: 7*60 + 30, Only: Term},
		{Day: calendar.Weekday, Minute: 8 * 60, Only: Holidays}, {Day: calendar.Weekday, Minute: 9 * 60, Only: Only(time.Tuesday)},
		{Day: calendar.Saturday, Minute: 10 * 60},
	}
	back.Departures = []Departure{{Day: calendar.Weekday, Minute: 12 * 60}}
	lines := []Line{{Name: "110", Ways: [2]*Route{&out, &back}}}
	inTerm := timetable(t, lines, true)
	if inTerm.RouteShortName != "110" || inTerm.InStopName != "Gara (Penny)" || inTerm.OutStopName != "MICESTI-Labului" {
		t.Fatalf("line %+v", inTerm)
	}
	if got := column(inTerm.Weekdays, true); !slices.Equal(got, []string{"07:00", "07:30"}) {
		t.Errorf("term weekdays %v", got)
	}
	if got := column(inTerm.Weekdays, false); !slices.Equal(got, []string{"12:00"}) {
		t.Errorf("back %v", got)
	}
	if got := column(inTerm.Saturday, true); !slices.Equal(got, []string{"10:00"}) {
		t.Errorf("saturday %v", got)
	}
	if got := column(timetable(t, lines, false).Weekdays, true); !slices.Equal(got, []string{"07:00", "08:00"}) {
		t.Errorf("holiday weekdays %v", got)
	}
}

func TestFirstStopOnly(t *testing.T) {
	r := route("1", "Gara", "Centru", "Spital")
	r.Departures = []Departure{{Day: calendar.Weekday, Minute: 6 * 60}, {Day: calendar.Weekday, Minute: 6*60 + 20}}
	tt := timetable(t, []Line{{Name: "1", Ways: [2]*Route{&r, nil}}}, true)
	if got := column(tt.Weekdays, true); !slices.Equal(got, []string{"06:00", "06:20"}) {
		t.Errorf("weekdays %v", got)
	}
}

func TestFootnotes(t *testing.T) {
	for text, want := range map[string]Limit{
		"Circulă doar în perioada școlii.":                                  Term,
		"Circulă doar în perioada vacanțelor școlare.":                      Holidays,
		"Circulă doar în zilele de marți.":                                  Only(time.Tuesday),
		"La orele marcate cu 00:00 nu circulă în timpul vacanțelor școlare": Term,
		"Nu circulă în perioada școlii.":                                    Holidays,
		"Nu se efectuează în perioada vacanțelor.":                          Term,
	} {
		if got, ok := LimitOf(text); !ok || got != want {
			t.Errorf("%q: %q", text, got)
		}
	}
	for _, text := range []string{"Circulă la cerere.", "Nu circulă marțea."} {
		if _, ok := LimitOf(text); ok {
			t.Errorf("%q has a meaning", text)
		}
	}
}
