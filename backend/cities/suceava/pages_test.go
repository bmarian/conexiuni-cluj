package suceava

import (
	"conexiuni-cluj/calendar"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func fixtureLines(t *testing.T) map[string]pageLine {
	t.Helper()
	data, err := os.ReadFile("testdata/pages.json")
	if err != nil {
		t.Fatal(err)
	}
	var pages []page
	if err := json.Unmarshal(data, &pages); err != nil {
		t.Fatal(err)
	}
	out := map[string]pageLine{}
	for _, l := range parseLines(pages) {
		out[l.short] = l
	}
	return out
}

func TestReadsTheLinePages(t *testing.T) {
	lines := fixtureLines(t)
	if len(lines) != 4 {
		t.Fatalf("%d lines, want M104, M117, M6 and 7 (not the index page)", len(lines))
	}
	m104 := lines["M104"]
	if len(m104.directions) != 2 || len(m104.directions[0]) != 10 || len(m104.directions[1]) != 9 || m104.directions[1][0] != "DEPOZIT MARMURĂ" {
		t.Fatalf("M104 stations %v", m104.directions)
	}
	weekday := m104.departures[calendar.Weekday]
	// "Plecări Obcini Flori" leaves from the outward start, "Plecări Cumpărătura" from the village the way back starts in.
	if len(weekday[0]) != 14 || weekday[0][0] != 5*60+40 || len(weekday[1]) != 14 || weekday[1][0] != 6*60+10 {
		t.Fatalf("M104 weekdays %v", weekday)
	}
	if len(m104.departures[calendar.Saturday]) != 0 || len(m104.departures[calendar.Sunday]) != 0 {
		t.Fatalf("M104 runs at the weekend: %v", m104.departures)
	}
	// The run from Primăria Bosanci starts off the line, so it is a departure of neither direction.
	if m117 := lines["M117"].departures[calendar.Weekday]; len(m117[0]) != 6 || len(m117[1]) != 5 {
		t.Fatalf("M117 weekdays %v", m117)
	}
	if m6 := lines["M6"]; len(m6.directions) != 1 || len(m6.directions[0]) != 26 || len(m6.departures[calendar.Weekday][0]) != 9 {
		t.Fatalf("M6 loop %d directions, departures %v", len(m6.directions), m6.departures)
	}
	if seven := lines["7"]; len(seven.directions) != 2 || len(seven.departures) != 0 {
		t.Fatalf("line 7, timed by events, has departures %v", seven.departures)
	}
}

func TestDayTitles(t *testing.T) {
	for title, want := range map[string][]string{
		"Luni – Vineri":               {calendar.Weekday},
		"Weekend și Sărbători legale": {calendar.Saturday, calendar.Sunday},
		"Sâmbătă și sărbători legale": {calendar.Saturday},
		"Luni – Duminică":             {calendar.Weekday, calendar.Saturday, calendar.Sunday},
	} {
		if got := dayKinds(title); !reflect.DeepEqual(got, want) {
			t.Errorf("%q: %v, want %v", title, got, want)
		}
	}
}
