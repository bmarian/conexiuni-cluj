package database

import (
	"database/sql"
	"errors"
	"time"
)

func LoadTranzyQuota(name string) (int, time.Time, error) {
	var count int
	var resetUnix int64
	err := DB.QueryRow(
		`SELECT count, reset_at FROM tranzy_quotas WHERE name = ?`, name,
	).Scan(&count, &resetUnix)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, time.Time{}, nil
	}
	if err != nil {
		return 0, time.Time{}, err
	}
	return count, time.Unix(resetUnix, 0), nil
}

func SaveTranzyQuota(name string, count int, resetAt time.Time) error {
	_, err := DB.Exec(
		`INSERT OR REPLACE INTO tranzy_quotas (name, count, reset_at) VALUES (?, ?, ?)`,
		name, count, resetAt.Unix(),
	)
	return err
}
