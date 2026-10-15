package handlers

import (
	"conexiuni-cluj/database"
	"database/sql"
	"fmt"
	"strings"
)

func queryRows[T any](query string, args []any, scan func(*sql.Rows) (T, error)) ([]T, error) {
	rows, err := database.DB.Query(query, args...)
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

func batchExec(insertSQL string, exec func(*sql.Stmt) error) error {
	return batchReplace(nil, insertSQL, exec)
}

func batchReplace(clear func(*sql.Tx) error, insertSQL string, exec func(*sql.Stmt) error) error {
	tx, err := database.DB.Begin()
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

func whereClause(conditions []string) string {
	if len(conditions) == 0 {
		return ""
	}
	return " WHERE " + strings.Join(conditions, " AND ")
}
