package push

import (
	"conexiuni-cluj/calendar"
	"conexiuni-cluj/database"
	"conexiuni-cluj/transit"
	"conexiuni-cluj/updates"
	"path/filepath"
	"testing"
	"time"
)

// A city's push and updates, wired as the city wires them, with no live table.
type testCity struct {
	*Service
	updates *updates.Service
}

type noLive struct{ calendar *calendar.Calendar }

func (noLive) ArrivalAt(string, int, time.Time) (time.Time, bool) { return time.Time{}, false }
func (l noLive) ScheduleDay(t time.Time) string                   { return l.calendar.ScheduleDay(t) }

func newTestCity(t *testing.T) *testCity {
	t.Helper()
	dir := t.TempDir()
	store, err := database.Open(filepath.Join(dir, "test.db"), filepath.Join(dir, "archive.db"), 5)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(store.Close)
	loc, _ := time.LoadLocation("Europe/Bucharest")
	city := &testCity{Service: New("test", loc, store, noLive{calendar.New(calendar.Seed(), nil)}), updates: updates.New(store, transit.New(store), loc)}
	city.updates.PushPending = Enabled
	city.updates.Notify = city.Wake
	return city
}
