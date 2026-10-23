package devicesync

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

const schema = `
CREATE TABLE IF NOT EXISTS chains (
	id        TEXT PRIMARY KEY,
	epoch     INTEGER NOT NULL,
	join_hash BLOB NOT NULL,
	created   INTEGER NOT NULL,
	seen      INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS chains_seen ON chains(seen);
CREATE TABLE IF NOT EXISTS devices (
	chain_id   TEXT NOT NULL REFERENCES chains(id) ON DELETE CASCADE,
	id         TEXT NOT NULL,
	token_hash BLOB NOT NULL UNIQUE,
	kind       TEXT NOT NULL,
	created    INTEGER NOT NULL,
	seen       INTEGER NOT NULL,
	PRIMARY KEY (chain_id, id)
);
CREATE TABLE IF NOT EXISTS records (
	chain_id TEXT NOT NULL REFERENCES chains(id) ON DELETE CASCADE,
	name     TEXT NOT NULL,
	rev      INTEGER NOT NULL,
	body     BLOB NOT NULL,
	PRIMARY KEY (chain_id, name)
);
CREATE TABLE IF NOT EXISTS wraps (
	chain_id  TEXT NOT NULL,
	device_id TEXT NOT NULL,
	epoch     INTEGER NOT NULL,
	body      BLOB NOT NULL,
	PRIMARY KEY (chain_id, device_id),
	FOREIGN KEY (chain_id, device_id) REFERENCES devices(chain_id, id) ON DELETE CASCADE
);`

func openStore(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite3", path+"?mode=rwc&_journal_mode=WAL&_busy_timeout=5000&_synchronous=NORMAL&_txlock=immediate&_foreign_keys=1")
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("sync schema: %w", err)
	}
	return db, nil
}

// day is when a device was last seen: the start of the UTC day, so a device writes it once a day at most.
func day(now time.Time) int64 {
	return now.UTC().Truncate(24 * time.Hour).Unix()
}

func (s *Service) size() (int64, error) {
	var n int64
	err := s.db.QueryRow(`SELECT page_count * page_size FROM pragma_page_count(), pragma_page_size()`).Scan(&n)
	return n, err
}

func (s *Service) pruneChains(now time.Time) error {
	_, err := s.db.Exec(`DELETE FROM chains WHERE seen < ?`, day(now.Add(-unusedFor)))
	return err
}

// inTx runs fn in one write transaction, committed when fn returns nil.
func (s *Service) inTx(fn func(tx *sql.Tx) error) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}
