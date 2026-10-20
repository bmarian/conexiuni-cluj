package database

import (
	"database/sql"
	"errors"
	"strconv"
)

// Above any operator's own numbering.
const allocatedIDFloor = 1_000_000

// ExternalID keeps a numeric id as is and allocates a stable one otherwise.
func (s *Store) ExternalID(kind, external string) (int, error) {
	s.idsMu.Lock()
	defer s.idsMu.Unlock()
	key := [2]string{kind, external}
	if id, ok := s.ids[key]; ok {
		return id, nil
	}
	var id int
	err := s.DB.QueryRow(`SELECT id FROM external_ids WHERE kind = ? AND external = ?`, kind, external).Scan(&id)
	if err == nil {
		s.ids[key] = id
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	if n, err := strconv.Atoi(external); err == nil && n >= 0 && n < allocatedIDFloor {
		id = n
	} else {
		var top sql.NullInt64
		if err := s.DB.QueryRow(`SELECT MAX(id) FROM external_ids WHERE kind = ? AND id >= ?`, kind, allocatedIDFloor).Scan(&top); err != nil {
			return 0, err
		}
		id = allocatedIDFloor
		if top.Valid {
			id = int(top.Int64) + 1
		}
	}
	if _, err := s.DB.Exec(`INSERT INTO external_ids (kind, external, id) VALUES (?, ?, ?)`, kind, external, id); err != nil {
		return 0, err
	}
	s.ids[key] = id
	return id, nil
}
