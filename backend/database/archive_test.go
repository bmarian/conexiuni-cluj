package database

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func withArchiveTestDBs(t *testing.T) (*Store, string) {
	t.Helper()
	dir := t.TempDir()
	return openTestStore(t, filepath.Join(dir, "main.db"), filepath.Join(dir, "archive.db")), dir
}

func openTestStore(t *testing.T, path, archivePath string) *Store {
	t.Helper()
	s, err := Open(path, archivePath, 5)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(s.Close)
	return s
}

func insertSegmentSample(t *testing.T, s *Store, observedAt int64) {
	t.Helper()
	if _, err := s.DB.Exec(`
		INSERT INTO segment_travel_time_samples
		(route_id, direction_id, from_stop_id, to_stop_id, day_type, bucket_start_min, duration_sec, observed_at)
		VALUES (25, 1, 10, 11, 'weekday', 480, 61.5, ?)`, observedAt); err != nil {
		t.Fatalf("insert sample: %v", err)
	}
}

func archiveRows(t *testing.T, s *Store, table string) int {
	t.Helper()
	var n int
	if err := s.Archive.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

func TestCopyToArchiveCopiesEachRowOnce(t *testing.T) {
	s, _ := withArchiveTestDBs(t)
	base := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC).Unix()
	for day := range int64(3) {
		insertSegmentSample(t, s, base+day*86400)
	}
	insertSegmentSample(t, s, base+3*86400)

	until := time.Unix(base+3*86400-60, 0)
	copied, err := s.CopyToArchive(until)
	if err != nil {
		t.Fatalf("copy: %v", err)
	}
	if got := copied["segment_travel_time_samples"]; got != 3 {
		t.Fatalf("first copy took %d samples, want the 3 before the cut", got)
	}
	if copied, _ = s.CopyToArchive(until); copied["segment_travel_time_samples"] != 0 {
		t.Fatalf("second copy took %d samples again", copied["segment_travel_time_samples"])
	}

	insertSegmentSample(t, s, base+3*86400+10)
	if copied, _ = s.CopyToArchive(time.Unix(base+4*86400, 0)); copied["segment_travel_time_samples"] != 2 {
		t.Fatalf("next copy took %d samples, want 2", copied["segment_travel_time_samples"])
	}
	if n := archiveRows(t, s, "segment_travel_time_samples"); n != 5 {
		t.Fatalf("archive holds %d samples, want 5", n)
	}

	var dayType string
	var duration float64
	if err := s.Archive.QueryRow(`SELECT day_type, duration_sec FROM segment_travel_time_samples LIMIT 1`).Scan(&dayType, &duration); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if dayType != "weekday" || duration != 61.5 {
		t.Fatalf("archived %q %v, want weekday 61.5", dayType, duration)
	}
}

func TestCopyToArchiveContinuesAfterMainDatabaseIsRecreated(t *testing.T) {
	s, dir := withArchiveTestDBs(t)
	base := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC).Unix()
	insertSegmentSample(t, s, base)
	if _, err := s.CopyToArchive(time.Unix(base+3600, 0)); err != nil {
		t.Fatalf("copy: %v", err)
	}

	s = openTestStore(t, filepath.Join(dir, "fresh.db"), filepath.Join(dir, "archive.db"))
	insertSegmentSample(t, s, base+7200)
	copied, err := s.CopyToArchive(time.Unix(base+10800, 0))
	if err != nil {
		t.Fatalf("copy after recreate: %v", err)
	}
	if copied["segment_travel_time_samples"] != 1 || archiveRows(t, s, "segment_travel_time_samples") != 2 {
		t.Fatalf("copied %d, archive holds %d; want 1 and 2", copied["segment_travel_time_samples"], archiveRows(t, s, "segment_travel_time_samples"))
	}
}

func TestCopyToArchiveKeepsTurnaroundsAndArrivals(t *testing.T) {
	s, _ := withArchiveTestDBs(t)
	at := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC).Unix()
	if _, err := s.DB.Exec(`INSERT INTO turnaround_samples (route_id, direction_id, day_type, duration_sec, arrived_at, departed_at, paired_departure_min)
		VALUES (25, 0, 'weekday', 240, ?, ?, 480)`, at, at+240); err != nil {
		t.Fatalf("insert stand: %v", err)
	}
	if _, err := s.DB.Exec(`INSERT INTO turnaround_arrivals (route_id, direction_id, arrived_at) VALUES (25, 0, ?)`, at); err != nil {
		t.Fatalf("insert arrival: %v", err)
	}
	if _, err := s.CopyToArchive(time.Unix(at+3600, 0)); err != nil {
		t.Fatalf("copy: %v", err)
	}
	if archiveRows(t, s, "turnaround_samples") != 1 || archiveRows(t, s, "turnaround_arrivals") != 1 {
		t.Fatalf("stands %d, arrivals %d; want 1 each", archiveRows(t, s, "turnaround_samples"), archiveRows(t, s, "turnaround_arrivals"))
	}
}

func TestCopyToArchiveTakesFinishedDaysOfAccuracyOnly(t *testing.T) {
	s, _ := withArchiveTestDBs(t)
	now := time.Now()
	yesterday, today := StatsDate(now.AddDate(0, 0, -1)), StatsDate(now)
	for _, r := range []struct{ date, metric string }{
		{yesterday, "eta_error"},
		{yesterday, "timetable_error"},
		{yesterday, "visitors"},
		{today, "eta_error"},
	} {
		if _, err := s.DB.Exec(`INSERT INTO stats_daily (date, metric, key, count) VALUES (?, ?, 'k', 3)`, r.date, r.metric); err != nil {
			t.Fatalf("insert stat: %v", err)
		}
	}
	copied, err := s.CopyToArchive(now)
	if err != nil {
		t.Fatalf("copy: %v", err)
	}
	if copied["stats_daily"] != 2 {
		t.Fatalf("copied %d stats rows, want yesterday's two accuracy rows", copied["stats_daily"])
	}
	if copied, _ = s.CopyToArchive(now); copied["stats_daily"] != 0 {
		t.Fatalf("copied %d stats rows again", copied["stats_daily"])
	}
}

func TestArchiveTimetablesStoresChangesOnly(t *testing.T) {
	s, _ := withArchiveTestDBs(t)
	a := TimetableVersion{RouteShortName: "25", InStopName: "A", OutStopName: "B", Weekdays: `{"entries":[]}`}
	b := a
	b.Weekdays = `{"entries":[{"in":"06:00"}]}`
	at := time.Unix(1_000_000, 0)
	for i, v := range []TimetableVersion{a, a, b, a} {
		if _, err := s.ArchiveTimetables([]TimetableVersion{v}, at.Add(time.Duration(i)*time.Hour)); err != nil {
			t.Fatalf("archive %d: %v", i, err)
		}
	}
	if n := archiveRows(t, s, "timetable_versions"); n != 3 {
		t.Fatalf("%d versions, want 3: a, b, then a again", n)
	}
}

func TestPruneArchiveKeepsTheVersionInForceAtTheCutoff(t *testing.T) {
	s, _ := withArchiveTestDBs(t)
	for i, stops := range []string{"[1]", "[2]", "[3]"} {
		if _, err := s.ArchiveTripStops(map[string]string{"25_0": stops}, time.Unix(int64(100*(i+1)), 0)); err != nil {
			t.Fatalf("archive stops: %v", err)
		}
	}
	insertSegmentSample(t, s, 100)
	insertSegmentSample(t, s, 400)
	if _, err := s.CopyToArchive(time.Unix(500, 0)); err != nil {
		t.Fatalf("copy: %v", err)
	}

	if _, err := s.PruneArchive(time.Unix(250, 0)); err != nil {
		t.Fatalf("prune: %v", err)
	}
	rows, err := s.Archive.Query(`SELECT stops FROM stop_times_versions ORDER BY first_seen_at`)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var kept []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatalf("scan: %v", err)
		}
		kept = append(kept, s)
	}
	if len(kept) != 2 || kept[0] != "[2]" || kept[1] != "[3]" {
		t.Fatalf("kept %v, want [2] (in force at the cutoff) and [3]", kept)
	}
	if n := archiveRows(t, s, "segment_travel_time_samples"); n != 1 {
		t.Fatalf("%d samples left, want the one after the cutoff", n)
	}
}

func TestArchiveCallsAreNoOpsWithoutArchive(t *testing.T) {
	s := openTestStore(t, filepath.Join(t.TempDir(), "main.db"), "")
	if copied, err := s.CopyToArchive(time.Now()); err != nil || len(copied) != 0 {
		t.Fatalf("copy without archive: %v %v", copied, err)
	}
	if n, err := s.ArchiveTripStops(map[string]string{"x": "[]"}, time.Now()); err != nil || n != 0 {
		t.Fatalf("stops without archive: %d %v", n, err)
	}
}

func TestArchiveVersionsSurviveConcurrentWriters(t *testing.T) {
	s, _ := withArchiveTestDBs(t)
	errs := make(chan error, 80)
	var wg sync.WaitGroup
	for i := range 40 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			trip := fmt.Sprintf("%d_0", i)
			if _, err := s.ArchiveTripStops(map[string]string{trip: "[1]", trip + "x": "[2]"}, time.Now()); err != nil {
				errs <- err
			}
			if _, err := s.ArchiveTimetables([]TimetableVersion{{RouteShortName: trip, Weekdays: "[]"}}, time.Now()); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent archive write: %v", err)
	}
	if n := archiveRows(t, s, "stop_times_versions"); n != 80 {
		t.Fatalf("%d trip versions, want 80", n)
	}
}
