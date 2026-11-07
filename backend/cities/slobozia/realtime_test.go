package slobozia

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func TestReadsTheLiveMap(t *testing.T) {
	data, err := os.ReadFile("testdata/bus-data.json")
	if err != nil {
		t.Fatal(err)
	}
	// One display as line 2's buses show it.
	data = []byte(strings.Replace(string(data), `"11":" 4B"`, `"11":" 2B"`, 1))
	now := time.Date(2026, 11, 7, 17, 0, 0, 0, time.UTC)
	buses, err := parseBuses(data, map[string]int{"1": 1, "2": 2, "3A": 3, "4B": 4}, now)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]int{"IL35SPT": 1, "IL36SPT": 3, "IL38SPT": 2}
	if len(buses) != len(want) {
		t.Fatalf("%d buses, want %d (off, NO_LINE and 0 are out of service): %+v", len(buses), len(want), buses)
	}
	for _, b := range buses {
		if len(b.Lines) != 1 || b.Lines[0].RouteID != want[b.ID] {
			t.Errorf("%s on %+v, want route %d", b.ID, b.Lines, want[b.ID])
		}
		if !b.At.Equal(now.Add(-1100*time.Millisecond)) || b.Lat == 0 || b.Lon == 0 {
			t.Errorf("%s: %+v", b.ID, b)
		}
	}
	stale := []byte(strings.Replace(string(data), `"stale":false`, `"stale":true`, 1))
	if _, err := parseBuses(stale, map[string]int{"1": 1}, now); !errors.Is(err, errStale) {
		t.Fatalf("stale positions read: %v", err)
	}
}
