package database

import (
	"database/sql"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

var DB *sql.DB

func Connect(dbPath string) error {
	var err error
	DB, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		return err
	}

	if err = DB.Ping(); err != nil {
		return err
	}

	log.Println("Database connected successfully")
	return nil
}

func InitSchemas() error {
	schema := `
		CREATE TABLE IF NOT EXISTS vehicles
        (
            id                    TEXT PRIMARY KEY,
            label                 TEXT    NOT NULL,
            latitude              REAL,
            longitude             REAL,
            timestamp             TEXT    NOT NULL,
            vehicle_type          INTEGER NOT NULL,
            bike_accessible       TEXT,
            wheelchair_accessible TEXT,
            speed                 REAL,
            route_id              INTEGER,
            trip_id               TEXT
        );
    `

	_, err := DB.Exec(schema)
	if err != nil {
		return err
	}

	log.Println("Database schema initialized")
	return nil
}
