package database

import (
	"fmt"
	"strings"
)

func (s *Store) initSchemas() error {

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

		CREATE TABLE IF NOT EXISTS trip_stops
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

		-- A feed's own stop_times offsets for the runs leaving in each hour, a JSON array by stop position
		CREATE TABLE IF NOT EXISTS scheduled_offsets
        (
            trip_id          TEXT    NOT NULL,
            day_type         TEXT    NOT NULL,
            bucket_start_min INTEGER NOT NULL,
            offsets          TEXT    NOT NULL,
            PRIMARY KEY (trip_id, day_type, bucket_start_min)
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

		CREATE TABLE IF NOT EXISTS external_ids
        (
            kind     TEXT    NOT NULL,
            external TEXT    NOT NULL,
            id       INTEGER NOT NULL,
            PRIMARY KEY (kind, external)
        );

		CREATE TABLE IF NOT EXISTS ingest_state
        (
            source    TEXT PRIMARY KEY,
            fetched_at INTEGER NOT NULL
        );

		-- Where the buses of traced lines stood, one row per place they moved to.
		CREATE TABLE IF NOT EXISTS line_traces
        (
            line     TEXT    NOT NULL,
            vehicle  TEXT    NOT NULL,
            lat      REAL    NOT NULL,
            lon      REAL    NOT NULL,
            first_at INTEGER NOT NULL,
            last_at  INTEGER NOT NULL
        );

		CREATE TABLE IF NOT EXISTS traced_lines
        (
            line TEXT PRIMARY KEY
        );

		DROP TABLE IF EXISTS cache_times;
		DROP TABLE IF EXISTS stop_info;
		DROP TABLE IF EXISTS api_stop_times;

		CREATE TABLE IF NOT EXISTS quotas
        (
            name      TEXT PRIMARY KEY,
            count     INTEGER NOT NULL,
            reset_at  INTEGER NOT NULL
        );

		DROP TABLE IF EXISTS news_cache;

		CREATE TABLE IF NOT EXISTS news
        (
            url          TEXT PRIMARY KEY,
            title        TEXT    NOT NULL,
            published_at INTEGER NOT NULL
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
            host       TEXT    NOT NULL DEFAULT '',
            updated_at INTEGER NOT NULL
        );

		CREATE TABLE IF NOT EXISTS push_subscription_routes
        (
            endpoint         TEXT NOT NULL,
            route_short_name TEXT NOT NULL,
            PRIMARY KEY (endpoint, route_short_name)
        );

		CREATE TABLE IF NOT EXISTS push_reminders
        (
            endpoint      TEXT    NOT NULL,
            id            TEXT    NOT NULL,
            stop_id       INTEGER NOT NULL,
            stop_name     TEXT    NOT NULL,
            route_id      INTEGER NOT NULL,
            trip_id       TEXT    NOT NULL,
            line          TEXT    NOT NULL,
            destination   TEXT    NOT NULL,
            days          TEXT    NOT NULL,
            date          TEXT    NOT NULL,
            slots         TEXT    NOT NULL,
            walk          INTEGER,
            walk_at       INTEGER NOT NULL DEFAULT 0,
            last_fired_at INTEGER NOT NULL DEFAULT 0,
            leave_at      INTEGER NOT NULL DEFAULT 0,
            PRIMARY KEY (endpoint, id)
        );

		CREATE TABLE IF NOT EXISTS stats_visitors_daily
        (
            date        TEXT NOT NULL,
            client_hash TEXT NOT NULL,
            PRIMARY KEY (date, client_hash)
        );

		CREATE TABLE IF NOT EXISTS stats_clients_daily
        (
            date        TEXT NOT NULL,
            client      TEXT NOT NULL,
            client_hash TEXT NOT NULL,
            PRIMARY KEY (date, client, client_hash)
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
        );

		CREATE TABLE IF NOT EXISTS schedule_delay_profiles
        (
            route_id         INTEGER NOT NULL,
            direction_id     INTEGER NOT NULL,
            day_type         TEXT    NOT NULL,
            bucket_start_min INTEGER NOT NULL,
            run_count        INTEGER NOT NULL,
            delay_sec        REAL    NOT NULL,
            spread_sec       REAL    NOT NULL DEFAULT 0,
            updated_at       INTEGER NOT NULL,
            PRIMARY KEY (route_id, direction_id, day_type, bucket_start_min)
        );

		CREATE TABLE IF NOT EXISTS turnaround_samples
        (
            id                    INTEGER PRIMARY KEY AUTOINCREMENT,
            route_id              INTEGER NOT NULL,
            direction_id          INTEGER NOT NULL,
            day_type              TEXT    NOT NULL,
            duration_sec          REAL    NOT NULL,
            arrived_at            INTEGER NOT NULL,
            departed_at           INTEGER NOT NULL,
            paired_departure_min  REAL    NOT NULL DEFAULT -1
        );
		CREATE INDEX IF NOT EXISTS turnaround_samples_route ON turnaround_samples (route_id, direction_id, departed_at);

		CREATE TABLE IF NOT EXISTS turnaround_arrivals
        (
            route_id         INTEGER NOT NULL,
            direction_id     INTEGER NOT NULL,
            arrived_at       INTEGER NOT NULL,
            PRIMARY KEY (route_id, direction_id, arrived_at)
        )
    `
	_, err := s.DB.Exec(schema)
	if err != nil {
		return err
	}

	// Before the indexes: an older database is missing columns some of them cover.
	if err := s.addMissingColumns(); err != nil {
		return err
	}
	if err := s.retireLifetimeVisitors(); err != nil {
		return err
	}

	indexes := `
		-- Vehicles indexes
		CREATE INDEX IF NOT EXISTS idx_line_traces ON line_traces(line, vehicle, first_at);
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
		CREATE INDEX IF NOT EXISTS idx_stats_visitors_daily_date ON stats_visitors_daily(date);
		CREATE INDEX IF NOT EXISTS idx_stats_visitors_daily_hash ON stats_visitors_daily(client_hash);
		CREATE INDEX IF NOT EXISTS idx_stats_daily_metric_count ON stats_daily(metric, count DESC);
		CREATE INDEX IF NOT EXISTS idx_stats_daily_metric_date_key ON stats_daily(metric, date, key);

		-- Segment travel profile indexes
		CREATE INDEX IF NOT EXISTS idx_segment_samples_key ON segment_travel_time_samples(route_id, direction_id, from_stop_id, to_stop_id, day_type, bucket_start_min, observed_at);
		CREATE INDEX IF NOT EXISTS idx_segment_samples_observed_at ON segment_travel_time_samples(observed_at);
		CREATE INDEX IF NOT EXISTS idx_segment_profiles_route_time ON segment_travel_time_profiles(route_id, direction_id, day_type, bucket_start_min);
	`

	_, err = s.DB.Exec(indexes)
	if err != nil {
		return err
	}

	s.Logf("Database schema and indexes initialized")
	return nil
}

// SQLite has no ADD COLUMN IF NOT EXISTS.
func (s *Store) addMissingColumns() error {
	additions := map[string][]string{
		"vehicles": {
			"raw_speed REAL NOT NULL DEFAULT 0",
			"anchor_lat REAL NOT NULL DEFAULT 0",
			"anchor_lon REAL NOT NULL DEFAULT 0",
			"anchor_at TEXT NOT NULL DEFAULT ''",
			"observed_at INTEGER NOT NULL DEFAULT 0",
			"prev_trip_id TEXT NOT NULL DEFAULT ''",
			"trip_changed_at TEXT NOT NULL DEFAULT ''",
		},
		"schedule_delay_profiles": {
			"spread_sec REAL NOT NULL DEFAULT 0",
		},
		"route_changes": {
			"push_pending INTEGER NOT NULL DEFAULT 0",
		},
		"push_reminders": {
			"walk INTEGER",
			"walk_at INTEGER NOT NULL DEFAULT 0",
			"leave_at INTEGER NOT NULL DEFAULT 0",
		},
		"push_subscriptions": {
			"host TEXT NOT NULL DEFAULT ''",
		},
		"stops": {
			"station_id INTEGER NOT NULL DEFAULT 0",
		},
	}
	for table, columns := range additions {
		existing, err := s.tableColumns(table)
		if err != nil {
			return err
		}
		for _, def := range columns {
			name := strings.Fields(def)[0]
			if _, ok := existing[name]; ok {
				continue
			}
			if _, err := s.DB.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s", table, def)); err != nil {
				return fmt.Errorf("add %s.%s: %w", table, name, err)
			}
			s.Logf("schema: added column %s.%s", table, name)
		}
	}
	return nil
}

// Moves stats_visitors into stats_daily and drops it.
func (s *Store) retireLifetimeVisitors() error {
	cols, err := s.tableColumns("stats_visitors")
	if err != nil || len(cols) == 0 {
		return err
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, stmt := range []string{
		`INSERT INTO stats_daily (date, metric, key, count)
			SELECT first_seen, 'new_visitors', '', COUNT(*) FROM stats_visitors WHERE true GROUP BY first_seen
			ON CONFLICT(date, metric, key) DO UPDATE SET count = excluded.count`,
		`INSERT INTO stats_daily (date, metric, key, count)
			SELECT date, 'visitors', '', COUNT(*) FROM stats_visitors_daily WHERE true GROUP BY date
			ON CONFLICT(date, metric, key) DO UPDATE SET count = excluded.count`,
		`DROP TABLE stats_visitors`,
	} {
		if _, err := tx.Exec(stmt); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.Logf("schema: moved stats_visitors into daily counts")
	return nil
}

func (s *Store) tableColumns(table string) (map[string]struct{}, error) {
	rows, err := s.DB.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
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
