package database

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestOpenDropsWebPush(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.db")
	old, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`CREATE TABLE route_changes (id INTEGER PRIMARY KEY AUTOINCREMENT, route_short_name TEXT NOT NULL, source TEXT NOT NULL,
			changes TEXT NOT NULL, signature TEXT NOT NULL, detected_at INTEGER NOT NULL, push_pending INTEGER NOT NULL DEFAULT 0)`,
		`INSERT INTO route_changes (route_short_name, source, changes, signature, detected_at, push_pending) VALUES ('24', 'timetable', '[]', 's', 1, 1)`,
		`CREATE TABLE push_subscriptions (endpoint TEXT PRIMARY KEY)`,
		`CREATE TABLE push_subscription_routes (endpoint TEXT NOT NULL, route_short_name TEXT NOT NULL)`,
		`CREATE INDEX idx_push_subscription_routes_route ON push_subscription_routes(route_short_name)`,
		`CREATE TABLE push_reminders (endpoint TEXT NOT NULL, id TEXT NOT NULL)`,
	} {
		if _, err := old.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	if err := old.Close(); err != nil {
		t.Fatal(err)
	}

	for range 2 {
		s := openTestStore(t, path, filepath.Join(dir, "b.db"))
		var tables int
		if err := s.DB.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name LIKE 'push%' OR name LIKE 'idx_push%'`).Scan(&tables); err != nil {
			t.Fatal(err)
		}
		cols, err := s.tableColumns("route_changes")
		if err != nil {
			t.Fatal(err)
		}
		var changes int
		if err := s.DB.QueryRow(`SELECT COUNT(*) FROM route_changes`).Scan(&changes); err != nil {
			t.Fatal(err)
		}
		if _, pending := cols["push_pending"]; tables != 0 || pending || changes != 1 {
			t.Fatalf("push tables %d, push_pending %v, changes %d", tables, pending, changes)
		}
		s.Close()
	}
}
