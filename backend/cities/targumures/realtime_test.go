package targumures

import (
	"os"
	"testing"
)

func TestReadsTheLiveMap(t *testing.T) {
	data, err := os.ReadFile("testdata/journeys.json")
	if err != nil {
		t.Fatal(err)
	}
	buses, err := parseJourneys(data, map[string]int{"11": 1, "30B": 2, "7": 3})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]int{"MS135": 1, "MS101": 2, "MS320": 3}
	if len(buses) != len(want) {
		t.Fatalf("%d buses, want %d (lines 0 and empty are out of service): %+v", len(buses), len(want), buses)
	}
	for _, b := range buses {
		if len(b.Lines) != 1 || b.Lines[0].RouteID != want[b.ID] {
			t.Errorf("%s on %+v, want route %d", b.ID, b.Lines, want[b.ID])
		}
		if b.At.IsZero() || b.Lat == 0 || b.Lon == 0 {
			t.Errorf("%s: %+v", b.ID, b)
		}
	}
}
