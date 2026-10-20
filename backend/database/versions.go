package database

import (
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"time"
)

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
	s.archiveWrites.Lock()
	defer s.archiveWrites.Unlock()
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
	s.archiveWrites.Lock()
	defer s.archiveWrites.Unlock()
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
