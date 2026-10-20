package live

import (
	"conexiuni-cluj/database"
	"conexiuni-cluj/transit"
	"path/filepath"
	"testing"
	"time"
)

func newTestCity(t *testing.T) *Engine {
	t.Helper()
	dir := t.TempDir()
	store, err := database.Open(filepath.Join(dir, "test.db"), filepath.Join(dir, "archive.db"), 5)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	loc, err := time.LoadLocation("Europe/Bucharest")
	if err != nil {
		t.Fatal(err)
	}
	eng := New("test", loc, store, transit.New(store), &transit.Availability{}, nil, nil, Options{})
	eng.resetProfileSnapshots()
	t.Cleanup(eng.Close)
	return eng
}

func (eng *Engine) withoutArchive(fn func()) {
	archive := eng.Archive
	eng.Archive = nil
	defer func() { eng.Archive = archive }()
	fn()
}

func minutesOf(t *testing.T, times ...string) []int {
	t.Helper()
	out := make([]int, len(times))
	for i, s := range times {
		m, ok := transit.ParseDeparture(s)
		if !ok {
			t.Fatalf("bad time %q", s)
		}
		out[i] = m
	}
	return out
}

// Same bucket preference as segment profiles: this hour, the nearest one, then all day.
func (eng *Engine) loadScheduleDelay(routeID, directionID int, dayType string, bucket int) (float64, bool) {
	rows, err := eng.loadScheduleDelayRows(routeID, directionID, dayType)
	if err != nil {
		eng.log("schedule delay: route=%d direction=%d: %v", routeID, directionID, err)
		return 0, false
	}
	return selectScheduleDelay(rows, bucket)
}

func (eng *Engine) resetTurnaroundCache() {
	eng.turnaroundCache.Lock()
	eng.turnaroundCache.stats = nil
	eng.turnaroundCache.routes = nil
	eng.turnaroundCache.Unlock()
}
