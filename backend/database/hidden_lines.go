package database

import (
	"database/sql"
	"time"
)

func (s *Store) ReadHiddenLines() ([]string, error) {
	if s.Archive == nil {
		return nil, nil
	}
	return QueryRows(s.Archive, `SELECT route_short_name FROM hidden_lines WHERE hidden = 1 ORDER BY route_short_name`, nil, func(rows *sql.Rows) (string, error) {
		var name string
		err := rows.Scan(&name)
		return name, err
	})
}

func (s *Store) SaveHiddenLines(names []string, hidden bool, at time.Time) error {
	return writeEach(s, names, `
		INSERT INTO hidden_lines (route_short_name, updated_at, hidden) VALUES (?, ?, ?)
		ON CONFLICT(route_short_name) DO UPDATE SET hidden = excluded.hidden, updated_at = excluded.updated_at`, at.Unix(), hidden)
}

func (s *Store) LastRuns() (map[int]time.Time, error) {
	out := map[int]time.Time{}
	if s.Archive == nil {
		return out, nil
	}
	err := QueryEach(s.Archive, `SELECT route_id, MAX(observed_at) FROM segment_travel_time_samples GROUP BY route_id`, nil, func(rows *sql.Rows) error {
		var route int
		var at int64
		if err := rows.Scan(&route, &at); err != nil {
			return err
		}
		out[route] = time.Unix(at, 0)
		return nil
	})
	return out, err
}
