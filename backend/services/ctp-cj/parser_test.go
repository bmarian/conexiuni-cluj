package ctp_cj

import "testing"

// Real M26 (lv) shape: the Cluj-Napoca column carries no departures, only a
// service window and an interval, spread across two consecutive rows. The
// opposite column has real times on those same rows that must survive.
const m26WeekdayCSV = `route_long_name,Cluj-Napoca - Floresti / Cetate
service_name,Luni - Vineri
service_start,04.05.2026
in_stop_name,Cluj-Napoca
out_stop_name,M-Floresti Cetate Pl.
,04:44
,06:10
05:10-22:40,06:23
10-20min,06:37
,06:50
`

func TestParseTimetableCSV_FrequencyDirection(t *testing.T) {
	parsed, err := ParseTimetableCSV([]byte(m26WeekdayCSV))
	if err != nil {
		t.Fatalf("ParseTimetableCSV: %v", err)
	}

	// (a) the window cell must never become a bogus departure entry.
	for _, e := range parsed.Entries {
		if e.DepartureIn != "" {
			t.Errorf("expected all DepartureIn empty for headway direction, got %q", e.DepartureIn)
		}
	}

	// (b) the out-time sharing a row with the interval cell must survive.
	if !hasOutTime(parsed.Entries, "06:37") {
		t.Errorf("06:37 (out-time on the interval row) was dropped; entries=%+v", parsed.Entries)
	}
	if !hasOutTime(parsed.Entries, "06:23") {
		t.Errorf("06:23 (out-time on the window row) was dropped; entries=%+v", parsed.Entries)
	}

	// (c) the window + interval halves fold into one InFrequency descriptor.
	if parsed.InFrequency == nil {
		t.Fatalf("expected InFrequency, got nil")
	}
	want := Frequency{Start: "05:10", End: "22:40", MinMinutes: 10, MaxMinutes: 20}
	if *parsed.InFrequency != want {
		t.Errorf("InFrequency = %+v, want %+v", *parsed.InFrequency, want)
	}

	// (d) the explicit-time direction stays non-frequency.
	if parsed.OutFrequency != nil {
		t.Errorf("expected OutFrequency nil, got %+v", *parsed.OutFrequency)
	}
}

func TestApplyFreqCell(t *testing.T) {
	cases := []struct {
		name string
		cell string
		want Frequency
	}{
		{"range interval", "10-20min", Frequency{MinMinutes: 10, MaxMinutes: 20}},
		{"single interval", "15min", Frequency{MinMinutes: 15, MaxMinutes: 15}},
		{"window", "05:45-22:40", Frequency{Start: "05:45", End: "22:40"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var f *Frequency
			if !applyFreqCell(&f, c.cell) {
				t.Fatalf("applyFreqCell(%q) = false, want true", c.cell)
			}
			if *f != c.want {
				t.Errorf("applyFreqCell(%q) = %+v, want %+v", c.cell, *f, c.want)
			}
		})
	}

	// Plain time cells and labels must not be mistaken for frequency cells.
	for _, cell := range []string{"", "07:20", "07:20*", "23:30"} {
		var f *Frequency
		if applyFreqCell(&f, cell) {
			t.Errorf("applyFreqCell(%q) = true, want false", cell)
		}
	}
}

// A normal route (explicit times both columns) keeps both frequencies nil.
func TestParseTimetableCSV_NoFrequency(t *testing.T) {
	csv := "route_long_name,A - B\nin_stop_name,A\nout_stop_name,B\n06:00,06:05\n06:30,06:35\n"
	parsed, err := ParseTimetableCSV([]byte(csv))
	if err != nil {
		t.Fatalf("ParseTimetableCSV: %v", err)
	}
	if parsed.InFrequency != nil || parsed.OutFrequency != nil {
		t.Errorf("expected no frequencies, got in=%+v out=%+v", parsed.InFrequency, parsed.OutFrequency)
	}
	if len(parsed.Entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(parsed.Entries))
	}
}

func hasOutTime(entries []TimetableEntry, t string) bool {
	for _, e := range entries {
		if e.DepartureOut == t {
			return true
		}
	}
	return false
}

// Real M39 (s) file from 2026-09-18: two rows mark the departure with a leading asterisk.
const m39SaturdayCSV = `route_long_name,Cluj-Napoca - Chinteni Lac
service_name,Sambata
service_start,18.05.2024
in_stop_name,Gara Mica Sud
out_stop_name,M-Chinteni Lac Pl.
*07:25,07:55**
09:30,08:35
11:30,10:05
*14:30,12:05
17:25,15:40
19:25,17:55
21:30,20:00
,22:05


`

func TestParseTimetableCSV_LeadingAnnotation(t *testing.T) {
	parsed, err := ParseTimetableCSV([]byte(m39SaturdayCSV))
	if err != nil {
		t.Fatalf("ParseTimetableCSV: %v", err)
	}
	want := []TimetableEntry{
		{"07:25*", "07:55**"},
		{"09:30", "08:35"},
		{"11:30", "10:05"},
		{"14:30*", "12:05"},
		{"17:25", "15:40"},
		{"19:25", "17:55"},
		{"21:30", "20:00"},
		{"", "22:05"},
	}
	if len(parsed.Entries) != len(want) {
		t.Fatalf("got %d entries, want %d: %+v", len(parsed.Entries), len(want), parsed.Entries)
	}
	for i, e := range parsed.Entries {
		if e != want[i] {
			t.Errorf("entry %d = %+v, want %+v", i, e, want[i])
		}
	}
}

// Real M23 (lv) row shapes, CRLF included, plus a made-up pair past midnight.
func TestParseTimetableCSV_LeadingAnnotationEitherColumn(t *testing.T) {
	csv := "service_start,14.09.2026\r\n" +
		"05:00,*04:40\r\n" +
		"05:45,05:20\r\n" +
		"*22:30,22:05\r\n" +
		"23:40,23:50\r\n" +
		"*00:15,*00:25\r\n"
	parsed, err := ParseTimetableCSV([]byte(csv))
	if err != nil {
		t.Fatalf("ParseTimetableCSV: %v", err)
	}
	want := []TimetableEntry{
		{"05:00", "04:40*"},
		{"05:45", "05:20"},
		{"22:30*", "22:05"},
		{"23:40", "23:50"},
		{"24:15*", "24:25*"},
	}
	if len(parsed.Entries) != len(want) {
		t.Fatalf("got %d entries, want %d: %+v", len(parsed.Entries), len(want), parsed.Entries)
	}
	for i, e := range parsed.Entries {
		if e != want[i] {
			t.Errorf("entry %d = %+v, want %+v", i, e, want[i])
		}
	}
}

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
