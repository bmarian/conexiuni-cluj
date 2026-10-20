package feeds

import (
	"conexiuni-cluj/models"
	"testing"
)

func TestCanonicalTime(t *testing.T) {
	cases := map[string]string{
		"*07:25":   "07:25*",
		" *22:30 ": "22:30*",
		"07:55**":  "07:55**",
		"07:20":    "07:20",
		"**07:20":  "07:20**",
		"":         "",
	}
	for in, want := range cases {
		if got := CanonicalTime(in); got != want {
			t.Errorf("CanonicalTime(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeEntriesMovesLeadingAnnotation(t *testing.T) {
	d := models.DaySchedule{Entries: []models.TimetableEntry{
		{DepartureIn: "*07:25", DepartureOut: "07:55**"},
		{DepartureIn: "23:40", DepartureOut: "23:50"},
		{DepartureIn: "*00:15", DepartureOut: "00:25"},
	}}
	NormalizeEntries(d.Entries)

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

// Real line 25 (d) rows, where CTP typed 21:08 for 22:08.
func TestNormalizeEntriesTypoIsNotMidnight(t *testing.T) {
	d := models.DaySchedule{Entries: []models.TimetableEntry{
		{DepartureIn: "21:44", DepartureOut: "21:50"},
		{DepartureIn: "21:56", DepartureOut: "22:02"},
		{DepartureIn: "21:08", DepartureOut: "22:14"},
		{DepartureIn: "22:20", DepartureOut: "22:25"},
		{DepartureIn: "23:40", DepartureOut: "23:50"},
		{DepartureIn: "00:15", DepartureOut: "00:25"},
	}}
	NormalizeEntries(d.Entries)

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
