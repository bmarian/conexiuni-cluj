package database

import (
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"
)

var archiveWrites sync.Mutex

const archiveCopyChunkSec = int64(24 * time.Hour / time.Second)

type archivedTable struct {
	name    string
	timeCol string
	columns []string
	unique  bool
}

var archivedTables = []archivedTable{
	{"segment_travel_time_samples", "observed_at", []string{"route_id", "direction_id", "from_stop_id", "to_stop_id", "day_type", "bucket_start_min", "duration_sec", "observed_at"}, false},
	{"turnaround_samples", "departed_at", []string{"route_id", "direction_id", "day_type", "duration_sec", "arrived_at", "departed_at", "paired_departure_min"}, false},
	{"turnaround_arrivals", "arrived_at", []string{"route_id", "direction_id", "arrived_at"}, true},
}

var archivedStatsMetrics = []string{"eta_error", "timetable_error"}

const archiveSchema = `
	CREATE TABLE IF NOT EXISTS segment_travel_time_samples
	(
		route_id         INTEGER NOT NULL,
		direction_id     INTEGER NOT NULL,
		from_stop_id     INTEGER NOT NULL,
		to_stop_id       INTEGER NOT NULL,
		day_type         TEXT    NOT NULL,
		bucket_start_min INTEGER NOT NULL,
		duration_sec     REAL    NOT NULL,
		observed_at      INTEGER NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_segment_samples_observed_at ON segment_travel_time_samples(observed_at);

	CREATE TABLE IF NOT EXISTS turnaround_samples
	(
		route_id             INTEGER NOT NULL,
		direction_id         INTEGER NOT NULL,
		day_type             TEXT    NOT NULL,
		duration_sec         REAL    NOT NULL,
		arrived_at           INTEGER NOT NULL,
		departed_at          INTEGER NOT NULL,
		paired_departure_min REAL    NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_turnaround_samples_departed_at ON turnaround_samples(departed_at);

	CREATE TABLE IF NOT EXISTS turnaround_arrivals
	(
		route_id     INTEGER NOT NULL,
		direction_id INTEGER NOT NULL,
		arrived_at   INTEGER NOT NULL,
		PRIMARY KEY (route_id, direction_id, arrived_at)
	);
	CREATE INDEX IF NOT EXISTS idx_turnaround_arrivals_arrived_at ON turnaround_arrivals(arrived_at);

	CREATE TABLE IF NOT EXISTS stats_daily
	(
		date   TEXT    NOT NULL,
		metric TEXT    NOT NULL,
		key    TEXT    NOT NULL,
		count  INTEGER NOT NULL,
		PRIMARY KEY (date, metric, key)
	);

	CREATE TABLE IF NOT EXISTS timetable_versions
	(
		id               INTEGER PRIMARY KEY AUTOINCREMENT,
		route_short_name TEXT    NOT NULL,
		hash             TEXT    NOT NULL,
		route_long_name  TEXT    NOT NULL,
		in_stop_name     TEXT    NOT NULL,
		out_stop_name    TEXT    NOT NULL,
		weekdays         TEXT    NOT NULL,
		saturday         TEXT    NOT NULL,
		sunday           TEXT    NOT NULL,
		first_seen_at    INTEGER NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_timetable_versions_route ON timetable_versions(route_short_name, first_seen_at);

	CREATE TABLE IF NOT EXISTS stop_times_versions
	(
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		trip_id       TEXT    NOT NULL,
		hash          TEXT    NOT NULL,
		stops         TEXT    NOT NULL,
		first_seen_at INTEGER NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_stop_times_versions_trip ON stop_times_versions(trip_id, first_seen_at);

	CREATE TABLE IF NOT EXISTS archive_state
	(
		name         TEXT PRIMARY KEY,
		copied_until INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS school_years
	(
		year       INTEGER PRIMARY KEY,
		data       TEXT    NOT NULL,
		updated_at INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS tuning
	(
		name       TEXT PRIMARY KEY,
		data       TEXT    NOT NULL,
		updated_at INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS schedule_overrides
	(
		date       TEXT PRIMARY KEY,
		schedule   TEXT    NOT NULL,
		source     TEXT    NOT NULL,
		detail     TEXT    NOT NULL DEFAULT '',
		created_at INTEGER NOT NULL
	);
`

func (s *Store) openArchive(path string) error {
	db, err := sql.Open("sqlite3", path+"?mode=rwc&_journal_mode=WAL&_busy_timeout=5000&_synchronous=NORMAL&_txlock=immediate")
	if err != nil {
		return err
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return err
	}
	db.SetMaxOpenConns(4)
	if _, err := db.Exec(archiveSchema); err != nil {
		_ = db.Close()
		return fmt.Errorf("archive schema: %w", err)
	}
	s.Archive = db
	return nil
}

func (s *Store) CopyToArchive(until time.Time) (map[string]int, error) {
	copied := map[string]int{}
	if s.Archive == nil {
		return copied, nil
	}
	for _, t := range archivedTables {
		n, err := s.copyTableToArchive(t, until.Unix())
		copied[t.name] = n
		if err != nil {
			return copied, fmt.Errorf("%s: %w", t.name, err)
		}
	}
	n, err := s.copyStatsToArchive(until)
	copied["stats_daily"] = n
	if err != nil {
		return copied, fmt.Errorf("stats_daily: %w", err)
	}
	return copied, nil
}

func (s *Store) copyTableToArchive(t archivedTable, until int64) (int, error) {
	from, err := s.archiveWatermark(t.name)
	if err != nil {
		return 0, err
	}
	if from == 0 {
		var oldest sql.NullInt64
		if err := s.DB.QueryRow(`SELECT MIN(` + t.timeCol + `) FROM ` + t.name).Scan(&oldest); err != nil {
			return 0, err
		}
		if !oldest.Valid {
			return 0, nil
		}
		from = oldest.Int64
	}
	total := 0
	for from < until {
		to := min(from+archiveCopyChunkSec, until)
		n, err := s.copyArchiveRange(t, from, to)
		if err != nil {
			return total, err
		}
		total += n
		from = to
	}
	return total, nil
}

func (s *Store) copyArchiveRange(t archivedTable, from, to int64) (int, error) {
	archiveWrites.Lock()
	defer archiveWrites.Unlock()
	cols := strings.Join(t.columns, ", ")
	rows, err := s.DB.Query(`SELECT `+cols+` FROM `+t.name+` WHERE `+t.timeCol+` >= ? AND `+t.timeCol+` < ?`, from, to)
	if err != nil {
		return 0, err
	}
	defer func() { _ = rows.Close() }()

	tx, err := s.Archive.Begin()
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	verb := "INSERT"
	if t.unique {
		verb = "INSERT OR IGNORE"
	}
	stmt, err := tx.Prepare(verb + ` INTO ` + t.name + ` (` + cols + `) VALUES (` + strings.TrimSuffix(strings.Repeat("?,", len(t.columns)), ",") + `)`)
	if err != nil {
		return 0, err
	}
	defer func() { _ = stmt.Close() }()

	values := make([]any, len(t.columns))
	targets := make([]any, len(values))
	for i := range values {
		targets[i] = &values[i]
	}
	n := 0
	for rows.Next() {
		if err := rows.Scan(targets...); err != nil {
			return 0, err
		}
		if _, err := stmt.Exec(values...); err != nil {
			return 0, err
		}
		n++
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if err := setArchiveWatermark(tx, t.name, to); err != nil {
		return 0, err
	}
	return n, tx.Commit()
}

func (s *Store) copyStatsToArchive(until time.Time) (int, error) {
	archiveWrites.Lock()
	defer archiveWrites.Unlock()
	from, err := s.archiveWatermark("stats_daily")
	if err != nil {
		return 0, err
	}
	fromDate := ""
	if from > 0 {
		fromDate = StatsDate(time.Unix(from, 0))
	}
	local := until.In(StatsLocation())
	midnight := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, local.Location())
	untilDate := StatsDate(midnight)
	if fromDate >= untilDate {
		return 0, nil
	}

	args := []any{fromDate, untilDate}
	for _, m := range archivedStatsMetrics {
		args = append(args, m)
	}
	rows, err := s.DB.Query(`
		SELECT date, metric, key, count FROM stats_daily
		WHERE date >= ? AND date < ? AND metric IN (`+strings.TrimSuffix(strings.Repeat("?,", len(archivedStatsMetrics)), ",")+`)`,
		args...)
	if err != nil {
		return 0, err
	}
	defer func() { _ = rows.Close() }()

	tx, err := s.Archive.Begin()
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	stmt, err := tx.Prepare(`
		INSERT INTO stats_daily (date, metric, key, count) VALUES (?, ?, ?, ?)
		ON CONFLICT(date, metric, key) DO UPDATE SET count = excluded.count`)
	if err != nil {
		return 0, err
	}
	defer func() { _ = stmt.Close() }()
	n := 0
	for rows.Next() {
		var date, metric, key string
		var count int
		if err := rows.Scan(&date, &metric, &key, &count); err != nil {
			return 0, err
		}
		if _, err := stmt.Exec(date, metric, key, count); err != nil {
			return 0, err
		}
		n++
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if err := setArchiveWatermark(tx, "stats_daily", midnight.Unix()); err != nil {
		return 0, err
	}
	return n, tx.Commit()
}

func (s *Store) archiveWatermark(name string) (int64, error) {
	var until int64
	err := s.Archive.QueryRow(`SELECT copied_until FROM archive_state WHERE name = ?`, name).Scan(&until)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return until, err
}

func setArchiveWatermark(tx *sql.Tx, name string, until int64) error {
	_, err := tx.Exec(`
		INSERT INTO archive_state (name, copied_until) VALUES (?, ?)
		ON CONFLICT(name) DO UPDATE SET copied_until = excluded.copied_until`, name, until)
	return err
}

func (s *Store) PruneArchive(before time.Time) (int, error) {
	if s.Archive == nil {
		return 0, nil
	}
	archiveWrites.Lock()
	defer archiveWrites.Unlock()
	cutoff := before.Unix()
	pruned := 0
	for _, t := range archivedTables {
		res, err := s.Archive.Exec(`DELETE FROM `+t.name+` WHERE `+t.timeCol+` < ?`, cutoff)
		if err != nil {
			return pruned, fmt.Errorf("%s: %w", t.name, err)
		}
		n, _ := res.RowsAffected()
		pruned += int(n)
	}
	for _, v := range []struct{ table, key string }{
		{"timetable_versions", "route_short_name"},
		{"stop_times_versions", "trip_id"},
	} {
		res, err := s.Archive.Exec(`
			DELETE FROM `+v.table+`
			WHERE first_seen_at < ?1
			  AND EXISTS (SELECT 1 FROM `+v.table+` n
			              WHERE n.`+v.key+` = `+v.table+`.`+v.key+`
			                AND n.first_seen_at > `+v.table+`.first_seen_at
			                AND n.first_seen_at <= ?1)`, cutoff)
		if err != nil {
			return pruned, fmt.Errorf("%s: %w", v.table, err)
		}
		n, _ := res.RowsAffected()
		pruned += int(n)
	}
	return pruned, nil
}

type TimetableVersion struct {
	RouteShortName string
	RouteLongName  string
	InStopName     string
	OutStopName    string
	Weekdays       string
	Saturday       string
	Sunday         string
}

func (v TimetableVersion) hash() string {
	return versionHash(v.RouteLongName, v.InStopName, v.OutStopName, v.Weekdays, v.Saturday, v.Sunday)
}

func (s *Store) ArchiveTimetables(versions []TimetableVersion, at time.Time) (int, error) {
	if s.Archive == nil || len(versions) == 0 {
		return 0, nil
	}
	archiveWrites.Lock()
	defer archiveWrites.Unlock()
	tx, err := s.Archive.Begin()
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	stored := 0
	for _, v := range versions {
		hash := v.hash()
		same, err := latestVersionIs(tx, `SELECT hash FROM timetable_versions WHERE route_short_name = ? ORDER BY first_seen_at DESC, id DESC LIMIT 1`, v.RouteShortName, hash)
		if err != nil {
			return 0, err
		}
		if same {
			continue
		}
		if _, err := tx.Exec(`
			INSERT INTO timetable_versions
			(route_short_name, hash, route_long_name, in_stop_name, out_stop_name, weekdays, saturday, sunday, first_seen_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			v.RouteShortName, hash, v.RouteLongName, v.InStopName, v.OutStopName, v.Weekdays, v.Saturday, v.Sunday, at.Unix()); err != nil {
			return 0, err
		}
		stored++
	}
	return stored, tx.Commit()
}

func (s *Store) ArchiveTripStops(stopsByTrip map[string]string, at time.Time) (int, error) {
	if s.Archive == nil || len(stopsByTrip) == 0 {
		return 0, nil
	}
	archiveWrites.Lock()
	defer archiveWrites.Unlock()
	tx, err := s.Archive.Begin()
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	stored := 0
	for tripID, stops := range stopsByTrip {
		hash := versionHash(stops)
		same, err := latestVersionIs(tx, `SELECT hash FROM stop_times_versions WHERE trip_id = ? ORDER BY first_seen_at DESC, id DESC LIMIT 1`, tripID, hash)
		if err != nil {
			return 0, err
		}
		if same {
			continue
		}
		if _, err := tx.Exec(`INSERT INTO stop_times_versions (trip_id, hash, stops, first_seen_at) VALUES (?, ?, ?, ?)`,
			tripID, hash, stops, at.Unix()); err != nil {
			return 0, err
		}
		stored++
	}
	return stored, tx.Commit()
}

func latestVersionIs(tx *sql.Tx, query, key, hash string) (bool, error) {
	var latest string
	err := tx.QueryRow(query, key).Scan(&latest)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return latest == hash, err
}

func versionHash(parts ...string) string {
	h := sha1.New()
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

type ArchiveSummary struct {
	Bytes    int64
	Samples  int
	OldestAt int64
}

func (s *Store) SummarizeArchive() (ArchiveSummary, error) {
	var out ArchiveSummary
	if s.Archive == nil {
		return out, nil
	}
	var pages, pageSize int64
	if err := s.Archive.QueryRow(`PRAGMA page_count`).Scan(&pages); err != nil {
		return out, err
	}
	if err := s.Archive.QueryRow(`PRAGMA page_size`).Scan(&pageSize); err != nil {
		return out, err
	}
	out.Bytes = pages * pageSize
	var oldest sql.NullInt64
	err := s.Archive.QueryRow(`SELECT COUNT(*), MIN(observed_at) FROM segment_travel_time_samples`).Scan(&out.Samples, &oldest)
	out.OldestAt = oldest.Int64
	return out, err
}

func (s *Store) SchoolYears() (map[int]string, error) {
	out := map[int]string{}
	if s.Archive == nil {
		return out, nil
	}
	rows, err := s.Archive.Query(`SELECT year, data FROM school_years`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var year int
		var data string
		if err := rows.Scan(&year, &data); err != nil {
			return nil, err
		}
		out[year] = data
	}
	return out, rows.Err()
}

func (s *Store) SaveSchoolYear(year int, data string, at time.Time) error {
	if s.Archive == nil {
		return fmt.Errorf("the archive database is not open")
	}
	archiveWrites.Lock()
	defer archiveWrites.Unlock()
	_, err := s.Archive.Exec(`
		INSERT INTO school_years (year, data, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(year) DO UPDATE SET data = excluded.data, updated_at = excluded.updated_at`, year, data, at.Unix())
	return err
}

type ScheduleOverride struct {
	Date      string `json:"date"`
	Schedule  string `json:"schedule"`
	Source    string `json:"source"`
	Detail    string `json:"detail"`
	CreatedAt int64  `json:"created_at"`
}

func (s *Store) ScheduleOverrides() ([]ScheduleOverride, error) {
	if s.Archive == nil {
		return nil, nil
	}
	rows, err := s.Archive.Query(`SELECT date, schedule, source, detail, created_at FROM schedule_overrides ORDER BY date`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []ScheduleOverride
	for rows.Next() {
		var o ScheduleOverride
		if err := rows.Scan(&o.Date, &o.Schedule, &o.Source, &o.Detail, &o.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

func (s *Store) SaveScheduleOverride(o ScheduleOverride) error {
	if s.Archive == nil {
		return fmt.Errorf("the archive database is not open")
	}
	archiveWrites.Lock()
	defer archiveWrites.Unlock()
	_, err := s.Archive.Exec(`
		INSERT INTO schedule_overrides (date, schedule, source, detail, created_at) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(date) DO UPDATE SET schedule = excluded.schedule, source = excluded.source,
			detail = excluded.detail, created_at = excluded.created_at`,
		o.Date, o.Schedule, o.Source, o.Detail, o.CreatedAt)
	return err
}

type TimetableAt struct {
	FirstSeenAt int64
	Weekdays    string
	Saturday    string
	Sunday      string
}

func (s *Store) TimetableHistory(routeShortName string) ([]TimetableAt, error) {
	if s.Archive == nil {
		return nil, nil
	}
	rows, err := s.Archive.Query(`
		SELECT first_seen_at, weekdays, saturday, sunday FROM timetable_versions
		WHERE route_short_name = ? ORDER BY first_seen_at, id`, routeShortName)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []TimetableAt
	for rows.Next() {
		var v TimetableAt
		if err := rows.Scan(&v.FirstSeenAt, &v.Weekdays, &v.Saturday, &v.Sunday); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *Store) RestoreFromArchive(table string, from, to int64, label func(at int64) (string, bool)) (int, error) {
	if s.Archive == nil || from >= to {
		return 0, nil
	}
	var t archivedTable
	for _, candidate := range archivedTables {
		if candidate.name == table {
			t = candidate
		}
	}
	timeIdx, dayIdx := -1, -1
	for i, c := range t.columns {
		switch c {
		case t.timeCol:
			timeIdx = i
		case "day_type":
			dayIdx = i
		}
	}
	if timeIdx < 0 || dayIdx < 0 {
		return 0, fmt.Errorf("%s cannot be restored", table)
	}

	cols := strings.Join(t.columns, ", ")
	rows, err := s.Archive.Query(`SELECT `+cols+` FROM `+t.name+` WHERE `+t.timeCol+` >= ? AND `+t.timeCol+` < ?`, from, to)
	if err != nil {
		return 0, err
	}
	defer func() { _ = rows.Close() }()
	tx, err := s.DB.Begin()
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	stmt, err := tx.Prepare(`INSERT INTO ` + t.name + ` (` + cols + `) VALUES (` + strings.TrimSuffix(strings.Repeat("?,", len(t.columns)), ",") + `)`)
	if err != nil {
		return 0, err
	}
	defer func() { _ = stmt.Close() }()

	values := make([]any, len(t.columns))
	targets := make([]any, len(values))
	for i := range values {
		targets[i] = &values[i]
	}
	n := 0
	for rows.Next() {
		if err := rows.Scan(targets...); err != nil {
			return 0, err
		}
		at, ok := values[timeIdx].(int64)
		if !ok {
			continue
		}
		dayType, keep := label(at)
		if !keep {
			continue
		}
		values[dayIdx] = dayType
		if _, err := stmt.Exec(values...); err != nil {
			return 0, err
		}
		n++
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	return n, tx.Commit()
}

func (s *Store) ScheduleOverrideOn(date string) (ScheduleOverride, bool, error) {
	var o ScheduleOverride
	if s.Archive == nil {
		return o, false, nil
	}
	err := s.Archive.QueryRow(`SELECT date, schedule, source, detail, created_at FROM schedule_overrides WHERE date = ?`, date).
		Scan(&o.Date, &o.Schedule, &o.Source, &o.Detail, &o.CreatedAt)
	if err == sql.ErrNoRows {
		return o, false, nil
	}
	return o, err == nil, err
}

func (s *Store) DeleteScheduleOverride(date string) error {
	if s.Archive == nil {
		return fmt.Errorf("the archive database is not open")
	}
	archiveWrites.Lock()
	defer archiveWrites.Unlock()
	_, err := s.Archive.Exec(`DELETE FROM schedule_overrides WHERE date = ?`, date)
	return err
}

func (s *Store) ArchivedSegmentSamples(from, to int64, fn func(route, direction, fromStop, toStop, bucket int, duration float64)) error {
	if s.Archive == nil {
		return nil
	}
	rows, err := s.Archive.Query(`
		SELECT route_id, direction_id, from_stop_id, to_stop_id, bucket_start_min, duration_sec
		FROM segment_travel_time_samples
		WHERE observed_at >= ? AND observed_at < ?`, from, to)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var route, direction, fromStop, toStop, bucket int
		var duration float64
		if err := rows.Scan(&route, &direction, &fromStop, &toStop, &bucket, &duration); err != nil {
			return err
		}
		fn(route, direction, fromStop, toStop, bucket, duration)
	}
	return rows.Err()
}

func (s *Store) ArchiveHasSegmentSamples(from, to int64) (bool, error) {
	if s.Archive == nil {
		return false, nil
	}
	var found bool
	err := s.Archive.QueryRow(`SELECT EXISTS (SELECT 1 FROM segment_travel_time_samples WHERE observed_at >= ? AND observed_at < ?)`, from, to).Scan(&found)
	return found, err
}

func (s *Store) Tuning(name string) (string, bool, error) {
	if s.Archive == nil {
		return "", false, nil
	}
	var data string
	err := s.Archive.QueryRow(`SELECT data FROM tuning WHERE name = ?`, name).Scan(&data)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	return data, err == nil, err
}

func (s *Store) SaveTuning(name, data string, at time.Time) error {
	if s.Archive == nil {
		return fmt.Errorf("the archive database is not open")
	}
	archiveWrites.Lock()
	defer archiveWrites.Unlock()
	_, err := s.Archive.Exec(`
		INSERT INTO tuning (name, data, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET data = excluded.data, updated_at = excluded.updated_at`, name, data, at.Unix())
	return err
}
