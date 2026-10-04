package handlers

import (
	"conexiuni-cluj/models"
	"testing"
)

func TestNormalizeDayScheduleMovesLeadingAnnotation(t *testing.T) {
	d := models.DaySchedule{Entries: []models.TimetableEntry{
		{DepartureIn: "*07:25", DepartureOut: "07:55**"},
		{DepartureIn: "23:40", DepartureOut: "23:50"},
		{DepartureIn: "*00:15", DepartureOut: "00:25"},
	}}
	normalizeDaySchedule(&d)

	want := []models.TimetableEntry{
		{DepartureIn: "07:25*", DepartureOut: "07:55**"},
		{DepartureIn: "23:40", DepartureOut: "23:50"},
		{DepartureIn: "24:15*", DepartureOut: "24:25"},
	}
	for i, e := range d.Entries {
		if e != want[i] {
			t.Errorf("entry %d = %+v, want %+v", i, e, want[i])
		}
	}
}

// Real line 25 (d) rows, where CTP typed 21:08 for 22:08. The cached copy used to
// read that as past midnight and push the rest of the evening into the next day.
func TestNormalizeDayScheduleTypoIsNotMidnight(t *testing.T) {
	d := models.DaySchedule{Entries: []models.TimetableEntry{
		{DepartureIn: "21:44", DepartureOut: "21:50"},
		{DepartureIn: "21:56", DepartureOut: "22:02"},
		{DepartureIn: "21:08", DepartureOut: "22:14"},
		{DepartureIn: "22:20", DepartureOut: "22:25"},
		{DepartureIn: "23:40", DepartureOut: "23:50"},
		{DepartureIn: "00:15", DepartureOut: "00:25"},
	}}
	normalizeDaySchedule(&d)

	want := []models.TimetableEntry{
		{DepartureIn: "21:44", DepartureOut: "21:50"},
		{DepartureIn: "21:56", DepartureOut: "22:02"},
		{DepartureIn: "21:08", DepartureOut: "22:14"},
		{DepartureIn: "22:20", DepartureOut: "22:25"},
		{DepartureIn: "23:40", DepartureOut: "23:50"},
		{DepartureIn: "24:15", DepartureOut: "24:25"},
	}
	for i, e := range d.Entries {
		if e != want[i] {
			t.Errorf("entry %d = %+v, want %+v", i, e, want[i])
		}
	}
}

func TestParseDepartureMinutesIgnoresAnnotations(t *testing.T) {
	for in, want := range map[string]int{"*07:25": 445, "07:25*": 445, "24:15*": 1455} {
		if got, ok := parseDepartureMinutes(in); !ok || got != want {
			t.Errorf("parseDepartureMinutes(%q) = %d, %v; want %d", in, got, ok, want)
		}
	}
}
