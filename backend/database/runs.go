package database

import (
	"database/sql"
	"strings"
)

// RunDirections are the stored directions of the given trips; trips the last import didn't list are left out.
func (s *Store) RunDirections(runIDs []string) (map[string]int, error) {
	out := make(map[string]int, len(runIDs))
	if len(runIDs) == 0 {
		return out, nil
	}
	args := make([]any, len(runIDs))
	for i, id := range runIDs {
		args[i] = id
	}
	type run struct {
		id        string
		direction int
	}
	runs, err := QueryRows(s.DB, `SELECT run_id, direction FROM runs WHERE run_id IN (`+strings.TrimSuffix(strings.Repeat("?,", len(runIDs)), ",")+`)`, args,
		func(rows *sql.Rows) (run, error) {
			var r run
			err := rows.Scan(&r.id, &r.direction)
			return r, err
		})
	for _, r := range runs {
		out[r.id] = r.direction
	}
	return out, err
}
