package database

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

var DB *sql.DB

var lastOptimize time.Time

func Connect(dbPath string) error {
	var err error
	connString := dbPath + "?cache=shared&mode=rwc&_journal_mode=WAL&_busy_timeout=5000"

	DB, err = sql.Open("sqlite3", connString)
	if err != nil {
		return err
	}

	errPing := DB.Ping()
	if errPing != nil {
		return errPing
	}

	if err = DB.Ping(); err != nil {
		return err
	}

	DB.SetMaxOpenConns(25)
	DB.SetMaxIdleConns(10)
	DB.SetConnMaxLifetime(0)
	DB.SetConnMaxIdleTime(5 * time.Minute)

	pragmas := []string{
		"PRAGMA synchronous = NORMAL",
		"PRAGMA cache_size = -64000",
		"PRAGMA temp_store = MEMORY",
		"PRAGMA mmap_size = 268435456",
		"PRAGMA page_size = 4096",
		"PRAGMA optimize",
	}

	for _, pragma := range pragmas {
		if _, err := DB.Exec(pragma); err != nil {
			log.Printf("Warning: Failed to set pragma '%s': %v", pragma, err)
		}
	}

	if err := checkIntegrity(); err != nil {
		// Don't fail boot — the server can still serve from Tranzy and
		// recover most caches by re-fetching. But scream loudly: silent
		// "database disk image is malformed" warnings buried in write paths
		// (one per cache miss) are easy to miss and waste hours of debugging.
		log.Printf("CRITICAL: database integrity check failed: %v -- the server will run but writes to corrupt tables will fail silently; consider running `sqlite3 %s '.recover' | sqlite3 recovered.db` and swapping the file", err, dbPath)
	}

	log.Println("Database connected")
	return nil
}

// checkIntegrity runs SQLite's PRAGMA integrity_check. It returns nil if the
// database reports "ok", or an error containing the first few problems it
// found otherwise. Cheap (sub-second on our DB size) and worth running every
// boot — the alternative is per-write warnings buried in app logs.
func checkIntegrity() error {
	rows, err := DB.Query(`PRAGMA integrity_check(5)`)
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

func InitSchemas() error {

	schema := `
		CREATE TABLE IF NOT EXISTS vehicles
        (
            id                    TEXT PRIMARY KEY,
            label                 TEXT    NOT NULL,
            latitude              REAL    NOT NULL,
            longitude             REAL    NOT NULL,
            timestamp             TEXT    NOT NULL,
            vehicle_type          INTEGER NOT NULL,
            bike_accessible       TEXT    NOT NULL,
            wheelchair_accessible TEXT    NOT NULL,
            speed                 REAL    NOT NULL,
            route_id              INTEGER NOT NULL,
            trip_id               TEXT    NOT NULL,
            raw_speed             REAL    NOT NULL DEFAULT 0,
            anchor_lat            REAL    NOT NULL DEFAULT 0,
            anchor_lon            REAL    NOT NULL DEFAULT 0,
            anchor_at             TEXT    NOT NULL DEFAULT '',
            observed_at           INTEGER NOT NULL DEFAULT 0
        );

		CREATE TABLE IF NOT EXISTS routes
        (
            route_id         INTEGER PRIMARY KEY,
            agency_id        INTEGER NOT NULL,
            route_short_name TEXT    NOT NULL,
            route_long_name  TEXT    NOT NULL,
            route_type       INTEGER NOT NULL,
            route_desc       TEXT    NOT NULL,
            route_color      TEXT    NOT NULL
        );

		CREATE TABLE IF NOT EXISTS trips
        (
            trip_id               TEXT    NOT NULL,
            route_id              INTEGER NOT NULL,
            direction_id          INTEGER NOT NULL,
            trip_headsign         TEXT    NOT NULL,
            block_id              INTEGER NOT NULL,
            shape_id              TEXT    NOT NULL,
            wheelchair_accessible INTEGER NOT NULL,
            bikes_allowed         INTEGER NOT NULL,
       		PRIMARY KEY (trip_id, route_id)
        );

		CREATE TABLE IF NOT EXISTS shapes
        (
            shape_id            TEXT    NOT NULL,
            shape_pt_lat        REAL    NOT NULL,
            shape_pt_lon        REAL    NOT NULL,
            shape_pt_sequence   INTEGER NOT NULL,
            shape_dist_traveled REAL    NOT NULL,
            PRIMARY KEY (shape_id, shape_pt_sequence)
        );

		CREATE TABLE IF NOT EXISTS stops
        (
            stop_id       INTEGER PRIMARY KEY,
            stop_name     TEXT    NOT NULL,
            stop_desc     TEXT    NOT NULL,
            stop_lat      REAL    NOT NULL,
            stop_lon      REAL    NOT NULL,
            location_type INTEGER NOT NULL,
            stop_code     TEXT    NOT NULL
        );

		CREATE TABLE IF NOT EXISTS api_stop_times
        (
            trip_id             TEXT    NOT NULL,
            stop_id             INTEGER NOT NULL,
            stop_sequence       INTEGER NOT NULL,
            PRIMARY KEY (trip_id, stop_sequence)
        );

		CREATE TABLE IF NOT EXISTS stop_times
        (
            trip_id             TEXT    NOT NULL,
            stop_id             INTEGER NOT NULL,
            offset_arrival_time REAL    NOT NULL,
            stop_sequence       INTEGER NOT NULL,
            stop_headsign       TEXT    NOT NULL,
            route_short_name    TEXT    NOT NULL,
            stop_lat            REAL    NOT NULL,
            stop_lon            REAL    NOT NULL,
            PRIMARY KEY (trip_id, stop_sequence)
        );

		CREATE TABLE IF NOT EXISTS timetable
        (
            route_short_name TEXT PRIMARY KEY,
            route_long_name  TEXT NOT NULL,
            in_stop_name     TEXT NOT NULL,
            out_stop_name    TEXT NOT NULL,
            weekdays         TEXT NOT NULL,
            saturday         TEXT NOT NULL,
            sunday           TEXT NOT NULL
        );

		CREATE TABLE IF NOT EXISTS cache_times
        (
            id        TEXT PRIMARY KEY,
            timestamp INTEGER NOT NULL,
            lifespan  INTEGER NOT NULL
        );
		
		CREATE TABLE IF NOT EXISTS stop_info
        (
            stop_id           TEXT PRIMARY KEY,
            trip_ids          TEXT NOT NULL,
            shapes_short_name TEXT NOT NULL
        );

		CREATE TABLE IF NOT EXISTS quotas
        (
            name      TEXT PRIMARY KEY,
            count     INTEGER NOT NULL,
            reset_at  INTEGER NOT NULL
        );

		CREATE TABLE IF NOT EXISTS news_cache
        (
            url   TEXT PRIMARY KEY,
            date  TEXT NOT NULL,
            title TEXT NOT NULL
        );

		CREATE TABLE IF NOT EXISTS route_changes
        (
            id               INTEGER PRIMARY KEY AUTOINCREMENT,
            route_short_name TEXT    NOT NULL,
            source           TEXT    NOT NULL,
            changes          TEXT    NOT NULL,
            signature        TEXT    NOT NULL,
            detected_at      INTEGER NOT NULL,
            push_pending     INTEGER NOT NULL DEFAULT 0
        );

		CREATE TABLE IF NOT EXISTS push_subscriptions
        (
            endpoint   TEXT PRIMARY KEY,
            p256dh     TEXT    NOT NULL,
            auth       TEXT    NOT NULL,
            locale     TEXT    NOT NULL,
            updated_at INTEGER NOT NULL
        );

		CREATE TABLE IF NOT EXISTS push_subscription_routes
        (
            endpoint         TEXT NOT NULL,
            route_short_name TEXT NOT NULL,
            PRIMARY KEY (endpoint, route_short_name)
        );

		CREATE TABLE IF NOT EXISTS stats_visitors
        (
            client_hash TEXT PRIMARY KEY,
            first_seen  TEXT NOT NULL,
            last_seen   TEXT NOT NULL
        );

		CREATE TABLE IF NOT EXISTS stats_visitors_daily
        (
            date        TEXT NOT NULL,
            client_hash TEXT NOT NULL,
            PRIMARY KEY (date, client_hash)
        );

		CREATE TABLE IF NOT EXISTS stats_daily
        (
            date   TEXT    NOT NULL,
            metric TEXT    NOT NULL,
            key    TEXT    NOT NULL,
            count  INTEGER NOT NULL,
            PRIMARY KEY (date, metric, key)
        );

		CREATE TABLE IF NOT EXISTS segment_travel_time_samples
        (
            id               INTEGER PRIMARY KEY AUTOINCREMENT,
            route_id         INTEGER NOT NULL,
            direction_id     INTEGER NOT NULL,
            from_stop_id     INTEGER NOT NULL,
            to_stop_id       INTEGER NOT NULL,
            day_type         TEXT    NOT NULL,
            bucket_start_min INTEGER NOT NULL,
            duration_sec     REAL    NOT NULL,
            observed_at      INTEGER NOT NULL
        );

		CREATE TABLE IF NOT EXISTS segment_travel_time_profiles
        (
            route_id         INTEGER NOT NULL,
            direction_id     INTEGER NOT NULL,
            from_stop_id     INTEGER NOT NULL,
            to_stop_id       INTEGER NOT NULL,
            day_type         TEXT    NOT NULL,
            bucket_start_min INTEGER NOT NULL,
            sample_count     INTEGER NOT NULL,
            median_sec       REAL    NOT NULL,
            p75_sec          REAL    NOT NULL,
            updated_at       INTEGER NOT NULL,
            PRIMARY KEY (route_id, direction_id, from_stop_id, to_stop_id, day_type, bucket_start_min)
        )
    `
	_, err := DB.Exec(schema)
	if err != nil {
		return err
	}

	// Before the indexes: an older database is missing columns some of them cover.
	if err := addMissingColumns(); err != nil {
		return err
	}

	indexes := `
		-- Vehicles indexes
		CREATE INDEX IF NOT EXISTS idx_vehicles_route_id ON vehicles(route_id);
		CREATE INDEX IF NOT EXISTS idx_vehicles_trip_id ON vehicles(trip_id);
		CREATE INDEX IF NOT EXISTS idx_vehicles_timestamp ON vehicles(timestamp);
		CREATE INDEX IF NOT EXISTS idx_vehicles_observed_at ON vehicles(observed_at);

		-- Routes indexes
		CREATE INDEX IF NOT EXISTS idx_routes_agency_id ON routes(agency_id);
		CREATE INDEX IF NOT EXISTS idx_routes_short_name ON routes(route_short_name);
		CREATE INDEX IF NOT EXISTS idx_routes_type ON routes(route_type);

		-- Trips indexes
		CREATE INDEX IF NOT EXISTS idx_trips_route_id ON trips(route_id);
		CREATE INDEX IF NOT EXISTS idx_trips_shape_id ON trips(shape_id);
		CREATE INDEX IF NOT EXISTS idx_trips_direction ON trips(direction_id);

		-- Shapes indexes
		CREATE INDEX IF NOT EXISTS idx_shapes_id ON shapes(shape_id);

		-- Stops indexes
		CREATE INDEX IF NOT EXISTS idx_stops_code ON stops(stop_code);
		CREATE INDEX IF NOT EXISTS idx_stops_location ON stops(stop_lat, stop_lon);

		-- StopTimes indexes
		CREATE INDEX IF NOT EXISTS idx_stop_times_trip_id ON stop_times(trip_id);
		CREATE INDEX IF NOT EXISTS idx_stop_times_stop_id ON stop_times(stop_id);

		-- Route change indexes
		CREATE INDEX IF NOT EXISTS idx_route_changes_route ON route_changes(route_short_name, detected_at);
		CREATE INDEX IF NOT EXISTS idx_route_changes_detected_at ON route_changes(detected_at);

		-- Push subscription indexes
		CREATE INDEX IF NOT EXISTS idx_push_subscription_routes_route ON push_subscription_routes(route_short_name);

		-- Stats indexes
		CREATE INDEX IF NOT EXISTS idx_stats_visitors_last_seen ON stats_visitors(last_seen);
		CREATE INDEX IF NOT EXISTS idx_stats_visitors_daily_date ON stats_visitors_daily(date);
		CREATE INDEX IF NOT EXISTS idx_stats_daily_metric_count ON stats_daily(metric, count DESC);
		CREATE INDEX IF NOT EXISTS idx_stats_daily_metric_date_key ON stats_daily(metric, date, key);

		-- Segment travel profile indexes
		CREATE INDEX IF NOT EXISTS idx_segment_samples_key ON segment_travel_time_samples(route_id, direction_id, from_stop_id, to_stop_id, day_type, bucket_start_min, observed_at);
		CREATE INDEX IF NOT EXISTS idx_segment_samples_observed_at ON segment_travel_time_samples(observed_at);
		CREATE INDEX IF NOT EXISTS idx_segment_profiles_route_time ON segment_travel_time_profiles(route_id, direction_id, day_type, bucket_start_min);
	`

	_, err = DB.Exec(indexes)
	if err != nil {
		return err
	}

	log.Println("Database schema and indexes initialized")
	return nil
}

// addMissingColumns brings older databases up to the current shape. SQLite has
// no "ADD COLUMN IF NOT EXISTS", so existing columns are read first.
func addMissingColumns() error {
	additions := map[string][]string{
		"vehicles": {
			"raw_speed REAL NOT NULL DEFAULT 0",
			"anchor_lat REAL NOT NULL DEFAULT 0",
			"anchor_lon REAL NOT NULL DEFAULT 0",
			"anchor_at TEXT NOT NULL DEFAULT ''",
			"observed_at INTEGER NOT NULL DEFAULT 0",
		},
		"route_changes": {
			"push_pending INTEGER NOT NULL DEFAULT 0",
		},
	}
	for table, columns := range additions {
		existing, err := tableColumns(table)
		if err != nil {
			return err
		}
		for _, def := range columns {
			name := strings.Fields(def)[0]
			if _, ok := existing[name]; ok {
				continue
			}
			if _, err := DB.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s", table, def)); err != nil {
				return fmt.Errorf("add %s.%s: %w", table, name, err)
			}
			log.Printf("schema: added column %s.%s", table, name)
		}
	}
	return nil
}

func tableColumns(table string) (map[string]struct{}, error) {
	rows, err := DB.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols := make(map[string]struct{})
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull, pk int
		var dflt any
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dflt, &pk); err != nil {
			return nil, err
		}
		cols[name] = struct{}{}
	}
	return cols, rows.Err()
}

// Rate-limit ANALYZE/optimize calls under concurrent cache writes.
func Optimize() error {
	if time.Since(lastOptimize) < 10*time.Minute {
		return nil
	}
	lastOptimize = time.Now()
	if _, err := DB.Exec("PRAGMA optimize"); err != nil {
		return err
	}
	if _, err := DB.Exec("ANALYZE"); err != nil {
		return err
	}
	log.Println("Database optimized")
	return nil
}

func Vacuum() error {
	if _, err := DB.Exec("VACUUM"); err != nil {
		return err
	}
	log.Println("Database vacuumed")
	return nil
}

func StartVacuumScheduler() {
	loc, err := time.LoadLocation("Europe/Bucharest")
	if err != nil {
		log.Printf("Warning: could not load Europe/Bucharest timezone, falling back to UTC: %v", err)
		loc = time.UTC
	}
	go func() {
		for {
			now := time.Now().In(loc)
			next := time.Date(now.Year(), now.Month(), now.Day(), 2, 0, 0, 0, loc)
			if !next.After(now) {
				next = next.Add(24 * time.Hour)
			}
			log.Printf("Next scheduled vacuum at %s", next.Format("2006-01-02 15:04:05"))
			time.Sleep(time.Until(next))
			if err := Vacuum(); err != nil {
				log.Printf("Scheduled vacuum failed: %v", err)
			}
		}
	}()
}

func StartDSTCacheInvalidationScheduler() {
	loc, err := time.LoadLocation("Europe/Bucharest")
	if err != nil {
		log.Printf("Warning: could not load Europe/Bucharest timezone; DST cache invalidation scheduler disabled: %v", err)
		return
	}

	go func() {
		for {
			now := time.Now().In(loc)
			nextCheck := time.Date(now.Year(), now.Month(), now.Day(), 4, 0, 0, 0, loc)
			if !nextCheck.After(now) {
				nextCheck = nextCheck.Add(24 * time.Hour)
			}

			log.Printf("Next DST offset check at %s", nextCheck.Format("2006-01-02 15:04:05 MST"))

			if wait := time.Until(nextCheck); wait > 0 {
				time.Sleep(wait)
			}

			checkTime := time.Now().In(loc)
			yesterday := checkTime.AddDate(0, 0, -1)

			_, currentOffset := checkTime.Zone()
			_, previousOffset := yesterday.Zone()

			if currentOffset != previousOffset {
				if err := InvalidateAllCaches(); err != nil {
					log.Printf("DST offset change detected but cache invalidation failed: %v", err)
				} else {
					log.Printf("DST offset change detected (%d -> %d); all cache entries invalidated", previousOffset, currentOffset)
				}
			} else {
				log.Printf("No DST offset change detected at daily check")
			}
		}
	}()
}
