package database

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func withArchiveTestDBs(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	connectTestMain(t, filepath.Join(dir, "main.db"))
	if err := ConnectArchive(filepath.Join(dir, "archive.db")); err != nil {
		t.Fatalf("connect archive: %v", err)
	}
	t.Cleanup(func() {
		_ = Archive.Close()
		Archive = nil
	})
	return dir
}

func connectTestMain(t *testing.T, path string) {
	t.Helper()
	if err := Connect(path); err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := InitSchemas(); err != nil {
		t.Fatalf("init schemas: %v", err)
	}
	db := DB
	t.Cleanup(func() { _ = db.Close() })
}

func insertSegmentSample(t *testing.T, observedAt int64) {
	t.Helper()
	if _, err := DB.Exec(`
		INSERT INTO segment_travel_time_samples
		(route_id, direction_id, from_stop_id, to_stop_id, day_type, bucket_start_min, duration_sec, observed_at)
		VALUES (25, 1, 10, 11, 'weekday', 480, 61.5, ?)`, observedAt); err != nil {
		t.Fatalf("insert sample: %v", err)
	}
}

func archiveRows(t *testing.T, table string) int {
	t.Helper()
	var n int
	if err := Archive.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

func TestCopyToArchiveCopiesEachRowOnce(t *testing.T) {
	withArchiveTestDBs(t)
	base := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC).Unix()
	for day := range int64(3) {
		insertSegmentSample(t, base+day*86400)
	}
	insertSegmentSample(t, base+3*86400)

	until := time.Unix(base+3*86400-60, 0)
	copied, err := CopyToArchive(until)
	if err != nil {
		t.Fatalf("copy: %v", err)
	}
	if got := copied["segment_travel_time_samples"]; got != 3 {
		t.Fatalf("first copy took %d samples, want the 3 before the cut", got)
	}
	if copied, _ = CopyToArchive(until); copied["segment_travel_time_samples"] != 0 {
		t.Fatalf("second copy took %d samples again", copied["segment_travel_time_samples"])
	}

	insertSegmentSample(t, base+3*86400+10)
	if copied, _ = CopyToArchive(time.Unix(base+4*86400, 0)); copied["segment_travel_time_samples"] != 2 {
		t.Fatalf("next copy took %d samples, want 2", copied["segment_travel_time_samples"])
	}
	if n := archiveRows(t, "segment_travel_time_samples"); n != 5 {
		t.Fatalf("archive holds %d samples, want 5", n)
	}

	var dayType string
	var duration float64
	if err := Archive.QueryRow(`SELECT day_type, duration_sec FROM segment_travel_time_samples LIMIT 1`).Scan(&dayType, &duration); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if dayType != "weekday" || duration != 61.5 {
		t.Fatalf("archived %q %v, want weekday 61.5", dayType, duration)
	}
}

func TestCopyToArchiveContinuesAfterMainDatabaseIsRecreated(t *testing.T) {
	dir := withArchiveTestDBs(t)
	base := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC).Unix()
	insertSegmentSample(t, base)
	if _, err := CopyToArchive(time.Unix(base+3600, 0)); err != nil {
		t.Fatalf("copy: %v", err)
	}

	connectTestMain(t, filepath.Join(dir, "fresh.db"))
	insertSegmentSample(t, base+7200)
	copied, err := CopyToArchive(time.Unix(base+10800, 0))
	if err != nil {
		t.Fatalf("copy after recreate: %v", err)
	}
	if copied["segment_travel_time_samples"] != 1 || archiveRows(t, "segment_travel_time_samples") != 2 {
		t.Fatalf("copied %d, archive holds %d; want 1 and 2", copied["segment_travel_time_samples"], archiveRows(t, "segment_travel_time_samples"))
	}
}

func TestCopyToArchiveKeepsTurnaroundsAndArrivals(t *testing.T) {
	withArchiveTestDBs(t)
	at := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC).Unix()
	if _, err := DB.Exec(`INSERT INTO turnaround_samples (route_id, direction_id, day_type, duration_sec, arrived_at, departed_at, paired_departure_min)
		VALUES (25, 0, 'weekday', 240, ?, ?, 480)`, at, at+240); err != nil {
		t.Fatalf("insert stand: %v", err)
	}
	if _, err := DB.Exec(`INSERT INTO turnaround_arrivals (route_id, direction_id, arrived_at) VALUES (25, 0, ?)`, at); err != nil {
		t.Fatalf("insert arrival: %v", err)
	}
	if _, err := CopyToArchive(time.Unix(at+3600, 0)); err != nil {
		t.Fatalf("copy: %v", err)
	}
	if archiveRows(t, "turnaround_samples") != 1 || archiveRows(t, "turnaround_arrivals") != 1 {
		t.Fatalf("stands %d, arrivals %d; want 1 each", archiveRows(t, "turnaround_samples"), archiveRows(t, "turnaround_arrivals"))
	}
}

func TestCopyToArchiveTakesFinishedDaysOfAccuracyOnly(t *testing.T) {
	withArchiveTestDBs(t)
	now := time.Now()
	yesterday, today := StatsDate(now.AddDate(0, 0, -1)), StatsDate(now)
	for _, r := range []struct{ date, metric string }{
		{yesterday, "eta_error"},
		{yesterday, "timetable_error"},
		{yesterday, "visitors"},
		{today, "eta_error"},
	} {
		if _, err := DB.Exec(`INSERT INTO stats_daily (date, metric, key, count) VALUES (?, ?, 'k', 3)`, r.date, r.metric); err != nil {
			t.Fatalf("insert stat: %v", err)
		}
	}
	copied, err := CopyToArchive(now)
	if err != nil {
		t.Fatalf("copy: %v", err)
	}
	if copied["stats_daily"] != 2 {
		t.Fatalf("copied %d stats rows, want yesterday's two accuracy rows", copied["stats_daily"])
	}
	if copied, _ = CopyToArchive(now); copied["stats_daily"] != 0 {
		t.Fatalf("copied %d stats rows again", copied["stats_daily"])
	}
}

func TestArchiveTimetablesStoresChangesOnly(t *testing.T) {
	withArchiveTestDBs(t)
	a := TimetableVersion{RouteShortName: "25", InStopName: "A", OutStopName: "B", Weekdays: `{"entries":[]}`}
	b := a
	b.Weekdays = `{"entries":[{"in":"06:00"}]}`
	at := time.Unix(1_000_000, 0)
	for i, v := range []TimetableVersion{a, a, b, a} {
		if _, err := ArchiveTimetables([]TimetableVersion{v}, at.Add(time.Duration(i)*time.Hour)); err != nil {
			t.Fatalf("archive %d: %v", i, err)
		}
	}
	if n := archiveRows(t, "timetable_versions"); n != 3 {
		t.Fatalf("%d versions, want 3: a, b, then a again", n)
	}
}

func TestPruneArchiveKeepsTheVersionInForceAtTheCutoff(t *testing.T) {
	withArchiveTestDBs(t)
	for i, stops := range []string{"[1]", "[2]", "[3]"} {
		if _, err := ArchiveTripStops(map[string]string{"25_0": stops}, time.Unix(int64(100*(i+1)), 0)); err != nil {
			t.Fatalf("archive stops: %v", err)
		}
	}
	insertSegmentSample(t, 100)
	insertSegmentSample(t, 400)
	if _, err := CopyToArchive(time.Unix(500, 0)); err != nil {
		t.Fatalf("copy: %v", err)
	}

	if _, err := PruneArchive(time.Unix(250, 0)); err != nil {
		t.Fatalf("prune: %v", err)
	}
	rows, err := Archive.Query(`SELECT stops FROM stop_times_versions ORDER BY first_seen_at`)
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
	if n := archiveRows(t, "segment_travel_time_samples"); n != 1 {
		t.Fatalf("%d samples left, want the one after the cutoff", n)
	}
}

func TestArchiveCallsAreNoOpsWithoutArchive(t *testing.T) {
	connectTestMain(t, filepath.Join(t.TempDir(), "main.db"))
	if copied, err := CopyToArchive(time.Now()); err != nil || len(copied) != 0 {
		t.Fatalf("copy without archive: %v %v", copied, err)
	}
	if n, err := ArchiveTripStops(map[string]string{"x": "[]"}, time.Now()); err != nil || n != 0 {
		t.Fatalf("stops without archive: %d %v", n, err)
	}
}

func TestArchiveVersionsSurviveConcurrentWriters(t *testing.T) {
	withArchiveTestDBs(t)
	errs := make(chan error, 80)
	var wg sync.WaitGroup
	for i := range 40 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			trip := fmt.Sprintf("%d_0", i)
			if _, err := ArchiveTripStops(map[string]string{trip: "[1]", trip + "x": "[2]"}, time.Now()); err != nil {
				errs <- err
			}
			if _, err := ArchiveTimetables([]TimetableVersion{{RouteShortName: trip, Weekdays: "[]"}}, time.Now()); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent archive write: %v", err)
	}
	if n := archiveRows(t, "stop_times_versions"); n != 80 {
		t.Fatalf("%d trip versions, want 80", n)
	}
}
