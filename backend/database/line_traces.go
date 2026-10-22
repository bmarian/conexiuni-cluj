package database

import "time"

// LineFix is a place a bus of a line stood at, from its first to its last sighting there.
type LineFix struct {
	Line, Vehicle string
	Lat, Lon      float64
	First, Last   time.Time
}

func (s *Store) AddLineFixes(fixes []LineFix) error {
	if len(fixes) == 0 {
		return nil
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, f := range fixes {
		if _, err := tx.Exec(`INSERT INTO line_traces (line, vehicle, lat, lon, first_at, last_at) VALUES (?, ?, ?, ?, ?, ?)`,
			f.Line, f.Vehicle, f.Lat, f.Lon, f.First.Unix(), f.Last.Unix()); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// LineFixes are a line's fixes since a time, each bus's in the order it made them.
func (s *Store) LineFixes(line string, since time.Time) ([]LineFix, error) {
	rows, err := s.DB.Query(`SELECT vehicle, lat, lon, first_at, last_at FROM line_traces WHERE line = ? AND first_at >= ? ORDER BY vehicle, first_at`, line, since.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LineFix
	for rows.Next() {
		f := LineFix{Line: line}
		var first, last int64
		if err := rows.Scan(&f.Vehicle, &f.Lat, &f.Lon, &first, &last); err != nil {
			return nil, err
		}
		f.First, f.Last = time.Unix(first, 0), time.Unix(last, 0)
		out = append(out, f)
	}
	return out, rows.Err()
}

func (s *Store) PruneLineFixes(line string, before time.Time) error {
	_, err := s.DB.Exec(`DELETE FROM line_traces WHERE line = ? AND first_at < ?`, line, before.Unix())
	return err
}

// SetTracedLines replaces the lines worth tracing and drops the fixes of lines no longer among them.
func (s *Store) SetTracedLines(lines []string) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM traced_lines`); err != nil {
		return err
	}
	for _, l := range lines {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO traced_lines (line) VALUES (?)`, l); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`DELETE FROM line_traces WHERE line NOT IN (SELECT line FROM traced_lines)`); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) TracedLines() ([]string, error) {
	rows, err := s.DB.Query(`SELECT line FROM traced_lines ORDER BY line`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var l string
		if err := rows.Scan(&l); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}
