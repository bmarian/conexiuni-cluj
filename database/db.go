package database

import (
	"log"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

type Database struct {
	*sqlx.DB
}

func New(dataSourceName string) (*Database, error) {
	db, err := sqlx.Connect("sqlite3", dataSourceName+"?cache=shared&mode=rwc&_journal_mode=WAL")
	if err != nil {
		return nil, err
	}

	// Connection pool settings
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)

	// Enable WAL mode for better concurrent reads
	if _, err := db.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		return nil, err
	}

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys=ON;"); err != nil {
		return nil, err
	}

	log.Println("Database connected successfully")

	return &Database{db}, nil
}

func (db *Database) Close() error {
	return db.DB.Close()
}
