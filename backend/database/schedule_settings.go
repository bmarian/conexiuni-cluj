package database

import (
	"database/sql"
	"fmt"
	"time"
)

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
	s.archiveWrites.Lock()
	defer s.archiveWrites.Unlock()
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
	s.archiveWrites.Lock()
	defer s.archiveWrites.Unlock()
	_, err := s.Archive.Exec(`
		INSERT INTO schedule_overrides (date, schedule, source, detail, created_at) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(date) DO UPDATE SET schedule = excluded.schedule, source = excluded.source,
			detail = excluded.detail, created_at = excluded.created_at`,
		o.Date, o.Schedule, o.Source, o.Detail, o.CreatedAt)
	return err
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
	s.archiveWrites.Lock()
	defer s.archiveWrites.Unlock()
	_, err := s.Archive.Exec(`DELETE FROM schedule_overrides WHERE date = ?`, date)
	return err
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
	s.archiveWrites.Lock()
	defer s.archiveWrites.Unlock()
	_, err := s.Archive.Exec(`
		INSERT INTO tuning (name, data, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET data = excluded.data, updated_at = excluded.updated_at`, name, data, at.Unix())
	return err
}
