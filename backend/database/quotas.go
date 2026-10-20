package database

import (
	"database/sql"
	"errors"
	"time"
)

func (s *Store) LoadQuota(name string) (int, time.Time, error) {
	var count int
	var resetUnix int64
	err := s.DB.QueryRow(
		`SELECT count, reset_at FROM quotas WHERE name = ?`, name,
	).Scan(&count, &resetUnix)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, time.Time{}, nil
	}
	if err != nil {
		return 0, time.Time{}, err
	}
	return count, time.Unix(resetUnix, 0), nil
}

func (s *Store) SaveQuota(name string, count int, resetAt time.Time) error {
	_, err := s.DB.Exec(
		`INSERT OR REPLACE INTO quotas (name, count, reset_at) VALUES (?, ?, ?)`,
		name, count, resetAt.Unix(),
	)
	return err
}
