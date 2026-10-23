package botosani

import (
	"conexiuni-cluj/calendar"
	"conexiuni-cluj/models"
	"slices"
	"testing"
)

type externalIDs map[string]int

func (m externalIDs) ExternalID(kind, external string) (int, error) {
	key := kind + ":" + external
	if _, ok := m[key]; !ok {
		m[key] = len(m) + 1
	}
	return m[key], nil
}

func columns(d models.DaySchedule) (in, out []string) {
	for _, e := range d.Entries {
		if e.DepartureIn != "" {
			in = append(in, e.DepartureIn)
		}
		if e.DepartureOut != "" {
			out = append(out, e.DepartureOut)
		}
	}
	return in, out
}

func TestTheUnprintedEndIsTimedFromEachBusRound(t *testing.T) {
	line := pageLine{short: "X1", markers: eastward("A", "b", "C"), runs: []run{
		{label: outward, names: []string{"A", "b", "C"}},
		{label: inward, names: []string{"C", "b", "A"}},
		{label: "CURSA SCURTĂ", names: []string{"A", "b", "A"}},
	}}
	bus := []departure{{minute: 600}, {minute: 660}, {minute: 720, variant: true}, {minute: 750}}
	program := &lineProgram{from: map[string]map[string][][]departure{"A": {calendar.Weekday: {bus}}}, variant: "S"}
	snap, err := build([]pageLine{line}, map[string]*lineProgram{"X1": program}, externalIDs{}, streets{})
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Routes) != 2 || len(snap.Trips) != 3 {
		t.Fatalf("routes %+v, trips %+v", snap.Routes, snap.Trips)
	}
	in, out := columns(snap.Timetables["X1"].Weekdays)
	// Equal lengths: C halfway through each round, none after the last.
	if want := []string{"10:00", "11:00", "12:30"}; !slices.Equal(in, want) {
		t.Errorf("from A %v, want %v", in, want)
	}
	if want := []string{"10:30", "11:30"}; !slices.Equal(out, want) {
		t.Errorf("from C %v, want %v", out, want)
	}
	if in, _ := columns(snap.Timetables["X1S"].Weekdays); !slices.Equal(in, []string{"12:00"}) {
		t.Errorf("short route %v", in)
	}
	if got := snap.Timetables["X1"]; got.InStopName != "A" || got.OutStopName != "C" || got.RouteLongName != "A - C" {
		t.Errorf("timetable %+v", got)
	}
}

func TestBuildFromTheOperatorsPageAndProgramme(t *testing.T) {
	lines, _ := fixturePage(t)
	snap, err := build(lines, fixtureProgram(t), externalIDs{}, streets{})
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Routes) != 3 {
		t.Fatalf("routes %+v", snap.Routes)
	}
	for _, c := range []struct {
		route, day string
		in, out    int
	}{
		{"BT01", calendar.Weekday, 97, 92},
		{"BT01", calendar.Saturday, 60, 56},
		{"BT01", calendar.Sunday, 55, 51},
		{"BT02", calendar.Weekday, 79, 83},
		{"BT02", calendar.Saturday, 24, 27},
		{"BT02", calendar.Sunday, 7, 10},
		{"BT02A", calendar.Weekday, 13, 0},
		{"BT02A", calendar.Saturday, 33, 0},
		{"BT02A", calendar.Sunday, 45, 0},
	} {
		in, out := columns(*daySchedules(snap.Timetables[c.route])[c.day])
		if len(in) != c.in || len(out) != c.out {
			t.Errorf("%s %s: %d and %d departures, want %d and %d", c.route, c.day, len(in), len(out), c.in, c.out)
		}
	}
	if got := snap.Timetables["BT02A"]; got.RouteLongName != "George Enescu - Util" {
		t.Errorf("short route %+v", got)
	}
}
