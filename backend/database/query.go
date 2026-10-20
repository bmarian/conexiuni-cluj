package database

import (
	"database/sql"
	"fmt"
	"strings"
)

// QueryRows scans every row of query with scan; an empty result is an empty slice.
func QueryRows[T any](db *sql.DB, query string, args []any, scan func(*sql.Rows) (T, error)) ([]T, error) {
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]T, 0)
	for rows.Next() {
		item, err := scan(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func BatchExec(db *sql.DB, insertSQL string, exec func(*sql.Stmt) error) error {
	return BatchReplace(db, nil, insertSQL, exec)
}

// BatchReplace runs clear, then exec's inserts through one prepared statement, all in one transaction.
func BatchReplace(db *sql.DB, clear func(*sql.Tx) error, insertSQL string, exec func(*sql.Stmt) error) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("error starting transaction: %w", err)
	}
	if clear != nil {
		if err := clear(tx); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	stmt, err := tx.Prepare(insertSQL)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("error preparing statement: %w", err)
	}
	defer stmt.Close()

	if err := exec(stmt); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func WhereClause(conditions []string) string {
	if len(conditions) == 0 {
		return ""
	}
	return " WHERE " + strings.Join(conditions, " AND ")
}

// QueryEach calls each for every row of query.
func QueryEach(db *sql.DB, query string, args []any, each func(*sql.Rows) error) error {
	rows, err := db.Query(query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		if err := each(rows); err != nil {
			return err
		}
	}
	return rows.Err()
}
