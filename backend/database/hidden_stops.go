package database

import (
	"fmt"
	"time"
)

func (s *Store) ReadHiddenStops() ([]int, error) {
	if s.Archive == nil {
		return nil, nil
	}
	rows, err := s.Archive.Query(`SELECT stop_id FROM hidden_stops WHERE hidden = 1 ORDER BY stop_id`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// Never undoes an admin's choice.
func (s *Store) SeedHiddenStops(ids []int, at time.Time) error {
	return s.writeHiddenStops(ids, `INSERT OR IGNORE INTO hidden_stops (stop_id, hidden, updated_at) VALUES (?, 1, ?)`, at.Unix())
}

func (s *Store) SaveHiddenStops(ids []int, hidden bool, at time.Time) error {
	return s.writeHiddenStops(ids, `
		INSERT INTO hidden_stops (stop_id, updated_at, hidden) VALUES (?, ?, ?)
		ON CONFLICT(stop_id) DO UPDATE SET hidden = excluded.hidden, updated_at = excluded.updated_at`, at.Unix(), hidden)
}

func (s *Store) writeHiddenStops(ids []int, query string, args ...any) error {
	if len(ids) == 0 {
		return nil
	}
	if s.Archive == nil {
		return fmt.Errorf("the archive database is not open")
	}
	s.archiveWrites.Lock()
	defer s.archiveWrites.Unlock()
	tx, err := s.Archive.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, id := range ids {
		if _, err := tx.Exec(query, append([]any{id}, args...)...); err != nil {
			return err
		}
	}
	return tx.Commit()
}
