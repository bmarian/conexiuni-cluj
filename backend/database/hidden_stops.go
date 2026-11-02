package database

import "time"

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
	return writeEach(s, ids, `INSERT OR IGNORE INTO hidden_stops (stop_id, hidden, updated_at) VALUES (?, 1, ?)`, at.Unix())
}

func (s *Store) SaveHiddenStops(ids []int, hidden bool, at time.Time) error {
	return writeEach(s, ids, `
		INSERT INTO hidden_stops (stop_id, updated_at, hidden) VALUES (?, ?, ?)
		ON CONFLICT(stop_id) DO UPDATE SET hidden = excluded.hidden, updated_at = excluded.updated_at`, at.Unix(), hidden)
}
