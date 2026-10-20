package database

import (
	"database/sql"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Store struct {
	DB      *sql.DB
	Archive *sql.DB
	name    string

	optimizeMu   sync.Mutex
	lastOptimize time.Time
	// One writer at a time on the archive.
	archiveWrites sync.Mutex

	visitorRetentionDays int
	statsMu              sync.Mutex
	statsBuf             *statsBuffer
	idsMu                sync.Mutex
	// Allocated once and never changed, so kept for the life of the store.
	ids map[[2]string]int
}

func Open(dbPath, archivePath string, visitorRetentionDays int) (*Store, error) {
	db, err := sql.Open("sqlite3", dbPath+"?cache=shared&mode=rwc&_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(0)
	db.SetConnMaxIdleTime(5 * time.Minute)
	for _, pragma := range []string{
		"PRAGMA synchronous = NORMAL",
		"PRAGMA cache_size = -64000",
		"PRAGMA temp_store = MEMORY",
		"PRAGMA mmap_size = 268435456",
		"PRAGMA page_size = 4096",
		"PRAGMA optimize",
	} {
		if _, err := db.Exec(pragma); err != nil {
			log.Printf("Warning: Failed to set pragma '%s': %v", pragma, err)
		}
	}
	if err := checkIntegrity(db); err != nil {
		// Not fatal: the network is fetched again on the next ingest.
		log.Printf("CRITICAL: %s integrity check failed: %v -- writes to corrupt tables will fail silently; consider `sqlite3 %s '.recover' | sqlite3 recovered.db`", dbPath, err, dbPath)
	}
	s := &Store{DB: db, name: strings.TrimSuffix(filepath.Base(dbPath), ".db"), statsBuf: newStatsBuffer(), visitorRetentionDays: visitorRetentionDays, ids: map[[2]string]int{}}
	if err := s.initSchemas(); err != nil {
		return nil, err
	}
	if archivePath != "" {
		if err := s.openArchive(archivePath); err != nil {
			log.Printf("Warning: archive %s unavailable, no history is kept: %v", archivePath, err)
		}
	}
	return s, nil
}

func (s *Store) Logf(format string, args ...any) {
	log.Printf(s.name+": "+format, args...)
}

func (s *Store) Close() {
	if s.Archive != nil {
		_ = s.Archive.Close()
	}
	_ = s.DB.Close()
}

func checkIntegrity(db *sql.DB) error {
	rows, err := db.Query(`PRAGMA integrity_check(5)`)
	if err != nil {
		return err
	}
	defer rows.Close()

	var problems []string
	for rows.Next() {
		var msg string
		if err := rows.Scan(&msg); err != nil {
			return err
		}
		if msg != "ok" {
			problems = append(problems, msg)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(problems) > 0 {
		return fmt.Errorf("%d problems: %s", len(problems), strings.Join(problems, "; "))
	}
	return nil
}
