package database

import (
	"database/sql"
	"strings"
	"time"
)

type requestError struct {
	count  int
	lastAt int64
	path   string
}

func (e requestError) add(other requestError) requestError {
	if other.lastAt > e.lastAt {
		e.lastAt, e.path = other.lastAt, other.path
	}
	e.count += other.count
	return e
}

type RequestErrorRow struct {
	Date   string
	Key    string
	Count  int
	LastAt int64
	Path   string
}

func (s *Store) RecordRequestError(key, path string, at time.Time) {
	s.statsMu.Lock()
	key = strings.Clone(key)
	s.statsBuf.errors[key] = requestError{count: 1, lastAt: at.Unix(), path: strings.Clone(path)}.add(s.statsBuf.errors[key])
	s.statsMu.Unlock()
}

func writeRequestErrors(tx *sql.Tx, today, cutoff string, errors map[string]requestError) error {
	if _, err := tx.Exec(`DELETE FROM stats_errors WHERE date < ?`, cutoff); err != nil {
		return err
	}
	upsert, err := tx.Prepare(`
		INSERT INTO stats_errors (date, key, count, last_at, path)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(date, key) DO UPDATE SET
			count = count + excluded.count,
			path = CASE WHEN excluded.last_at >= last_at THEN excluded.path ELSE path END,
			last_at = MAX(last_at, excluded.last_at)
	`)
	if err != nil {
		return err
	}
	defer func() { _ = upsert.Close() }()
	for key, e := range errors {
		if _, err := upsert.Exec(today, key, e.count, e.lastAt, e.path); err != nil {
			return err
		}
	}
	return nil
}

// Unflushed errors included.
func (s *Store) RequestErrors(since string) ([]RequestErrorRow, error) {
	rows, err := s.DB.Query(`SELECT date, key, count, last_at, path FROM stats_errors WHERE date >= ?`, since)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]RequestErrorRow, 0)
	for rows.Next() {
		var r RequestErrorRow
		if err := rows.Scan(&r.Date, &r.Key, &r.Count, &r.LastAt, &r.Path); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	today := StatsDate(time.Now())
	s.statsMu.Lock()
	for key, e := range s.statsBuf.errors {
		out = append(out, RequestErrorRow{Date: today, Key: key, Count: e.count, LastAt: e.lastAt, Path: e.path})
	}
	s.statsMu.Unlock()
	return out, nil
}
