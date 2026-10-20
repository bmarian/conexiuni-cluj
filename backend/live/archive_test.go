package live

import (
	"testing"
	"time"
)

func (eng *Engine) archivedVersions(t *testing.T, table string) int {
	t.Helper()
	var n int
	if err := eng.Archive.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

func TestStoredTimetablesAndStopsReachTheArchiveOnce(t *testing.T) {
	eng := newTestCity(t)
	stops := eng.installDelayRoute(t)
	if _, err := eng.ArchiveVersions(time.Now()); err != nil {
		t.Fatalf("first snapshot: %v", err)
	}
	if eng.archivedVersions(t, "timetable_versions") != 1 || eng.archivedVersions(t, "stop_times_versions") != 1 {
		t.Fatalf("timetables %d, trips %d; want 1 each", eng.archivedVersions(t, "timetable_versions"), eng.archivedVersions(t, "stop_times_versions"))
	}

	if err := eng.StoreStopTimes(stops); err != nil {
		t.Fatalf("store again: %v", err)
	}
	if n, err := eng.ArchiveVersions(time.Now()); err != nil || n != 0 {
		t.Fatalf("snapshot after an unchanged store added %d versions (%v)", n, err)
	}

	stops[2].OffsetArrivalTime = 180
	if err := eng.StoreStopTimes(stops); err != nil {
		t.Fatalf("store changed: %v", err)
	}
	if n := eng.archivedVersions(t, "stop_times_versions"); n != 2 {
		t.Fatalf("%d trip versions after a stop moved, want 2", n)
	}
	if n, err := eng.ArchiveVersions(time.Now()); err != nil || n != 0 {
		t.Fatalf("snapshot repeated the hooked version: %d (%v)", n, err)
	}
}

func TestNextArchiveRun(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Bucharest")
	if err != nil {
		t.Skip("no tz data")
	}
	cases := []struct{ now, want string }{
		{"2026-10-15 02:00", "2026-10-15 03:30"},
		{"2026-10-15 03:30", "2026-10-16 03:30"},
		{"2026-10-15 22:00", "2026-10-16 03:30"},
	}
	for _, c := range cases {
		now, _ := time.ParseInLocation("2006-01-02 15:04", c.now, loc)
		if got := nextArchiveRun(now, loc).Format("2006-01-02 15:04"); got != c.want {
			t.Errorf("nextArchiveRun(%s) = %s, want %s", c.now, got, c.want)
		}
	}
}
