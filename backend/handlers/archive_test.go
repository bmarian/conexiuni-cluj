package handlers

import (
	"conexiuni-cluj/database"
	"path/filepath"
	"testing"
	"time"
)

func withTestArchive(t *testing.T) {
	t.Helper()
	if err := database.ConnectArchive(filepath.Join(t.TempDir(), "archive.db")); err != nil {
		t.Fatalf("connect archive: %v", err)
	}
	t.Cleanup(func() {
		_ = database.Archive.Close()
		database.Archive = nil
	})
}

func archivedVersions(t *testing.T, table string) int {
	t.Helper()
	var n int
	if err := database.Archive.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

func TestStoredTimetablesAndStopsReachTheArchiveOnce(t *testing.T) {
	stops := installDelayRoute(t)
	withTestArchive(t)
	if _, err := snapshotArchiveVersions(time.Now()); err != nil {
		t.Fatalf("first snapshot: %v", err)
	}
	if archivedVersions(t, "timetable_versions") != 1 || archivedVersions(t, "stop_times_versions") != 1 {
		t.Fatalf("timetables %d, trips %d; want 1 each", archivedVersions(t, "timetable_versions"), archivedVersions(t, "stop_times_versions"))
	}

	if err := storeStopTimesInDB(stops); err != nil {
		t.Fatalf("store again: %v", err)
	}
	if n, err := snapshotArchiveVersions(time.Now()); err != nil || n != 0 {
		t.Fatalf("snapshot after an unchanged store added %d versions (%v)", n, err)
	}

	stops[2].OffsetArrivalTime = 180
	if err := storeStopTimesInDB(stops); err != nil {
		t.Fatalf("store changed: %v", err)
	}
	if n := archivedVersions(t, "stop_times_versions"); n != 2 {
		t.Fatalf("%d trip versions after a stop moved, want 2", n)
	}
	if n, err := snapshotArchiveVersions(time.Now()); err != nil || n != 0 {
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
