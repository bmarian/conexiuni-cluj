package botosani

import (
	"os"
	"strings"
	"testing"
)

func fixtureRows(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile("testdata/rows.txt")
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(string(data), "\n")
}

func TestParseWindowsReadsEveryBlock(t *testing.T) {
	windows := parseWindows(fixtureRows(t))
	count := map[string]int{}
	for _, w := range windows {
		count[routeOf(w)+"/"+w.day]++
	}
	want := map[string]int{
		"101/weekdays": 5, "101/saturday": 2, "101/sunday": 2,
		"102/weekdays": 2, "102S/weekdays": 3,
		"102/saturday": 2, "102S/saturday": 2,
		"102/sunday": 1, "102S/sunday": 2,
	}
	for key, n := range want {
		if count[key] != n {
			t.Errorf("%s: %d windows, want %d", key, count[key], n)
		}
	}
	if len(windows) != 21 {
		t.Errorf("%d windows in all, want 21", len(windows))
	}
}

func TestDeparturesFollowTheHeadways(t *testing.T) {
	var weekdays []window
	for _, w := range parseWindows(fixtureRows(t)) {
		if w.line == "101" && w.day == "weekdays" {
			weekdays = append(weekdays, w)
		}
	}
	d := departures(weekdays)
	if d[0] != "05:00" || d[1] != "05:10" || d[len(d)-1] != "23:00" {
		t.Fatalf("departures start %s, %s and end %s", d[0], d[1], d[len(d)-1])
	}
	// 05-08 every 10 (18), 08-12 every 12 (20), 12-17 every 10 (30), 17-20 every 12 (15), 20-23 every 15 (12) and the last bus.
	if len(d) != 18+20+30+15+12+1 {
		t.Fatalf("%d departures", len(d))
	}
	f := frequency(weekdays)
	if f.Start != "05:00" || f.End != "23:00" || f.MinMinutes != 10 || f.MaxMinutes != 15 {
		t.Fatalf("frequency %+v", f)
	}
}
