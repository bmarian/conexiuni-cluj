package transit

import (
	"conexiuni-cluj/database"
	"conexiuni-cluj/feeds"
	"database/sql"
	"encoding/json"
	"fmt"
)

// StoreNetwork replaces the lines, stops, patterns and shapes with snap's, in one transaction so a reader never sees half a network.
func (d *Data) StoreNetwork(snap *feeds.Snapshot) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, table := range []string{"routes", "stops", "trips", "shapes", "trip_stops"} {
		if _, err := tx.Exec(`DELETE FROM ` + table); err != nil {
			return err
		}
	}
	if err := insertAll(tx, `INSERT OR REPLACE INTO routes (route_id, agency_id, route_short_name, route_long_name, route_type, route_desc, route_color) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		len(snap.Routes), func(i int) []any {
			r := snap.Routes[i]
			return []any{r.RouteID, r.AgencyID, r.RouteShortName, r.RouteLongName, r.RouteType, r.RouteDesc, r.RouteColor}
		}); err != nil {
		return fmt.Errorf("routes: %w", err)
	}
	if err := insertAll(tx, `INSERT OR REPLACE INTO stops (stop_id, stop_name, stop_desc, stop_lat, stop_lon, location_type, stop_code, station_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		len(snap.Stops), func(i int) []any {
			s := snap.Stops[i]
			return []any{s.StopID, s.StopName, s.StopDesc, s.StopLat, s.StopLon, s.LocationType, s.StopCode, s.StationID}
		}); err != nil {
		return fmt.Errorf("stops: %w", err)
	}
	if err := insertAll(tx, `INSERT OR REPLACE INTO trips (trip_id, route_id, direction_id, trip_headsign, block_id, shape_id, wheelchair_accessible, bikes_allowed) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		len(snap.Trips), func(i int) []any {
			t := snap.Trips[i]
			return []any{t.TripID, t.RouteID, t.DirectionID, t.TripHeadsign, t.BlockID, t.ShapeID, t.WheelchairAccessible, t.BikesAllowed}
		}); err != nil {
		return fmt.Errorf("trips: %w", err)
	}
	if err := insertAll(tx, `INSERT OR REPLACE INTO shapes (shape_id, shape_pt_lat, shape_pt_lon, shape_pt_sequence, shape_dist_traveled) VALUES (?, ?, ?, ?, ?)`,
		len(snap.Shapes), func(i int) []any {
			s := snap.Shapes[i]
			return []any{s.ShapeID, s.ShapePtLat, s.ShapePtLon, s.ShapePtSequence, s.ShapeDistTraveled}
		}); err != nil {
		return fmt.Errorf("shapes: %w", err)
	}
	if err := insertAll(tx, `INSERT OR REPLACE INTO trip_stops (trip_id, stop_id, stop_sequence) VALUES (?, ?, ?)`,
		len(snap.TripStops), func(i int) []any {
			s := snap.TripStops[i]
			return []any{s.TripID, s.StopID, s.StopSequence}
		}); err != nil {
		return fmt.Errorf("trip stops: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return d.store.Optimize()
}

func insertAll(tx *sql.Tx, query string, n int, row func(i int) []any) error {
	stmt, err := tx.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for i := 0; i < n; i++ {
		if _, err := stmt.Exec(row(i)...); err != nil {
			return err
		}
	}
	return nil
}

// Snapshot is the network as stored, without times.
func (d *Data) Snapshot() (*feeds.Snapshot, error) {
	routes, err := d.Routes(RouteFilter{})
	if err != nil {
		return nil, err
	}
	stops, err := d.Stops(StopFilter{})
	if err != nil {
		return nil, err
	}
	trips, err := d.Trips(TripFilter{})
	if err != nil {
		return nil, err
	}
	shapes, err := d.Shapes(nil)
	if err != nil {
		return nil, err
	}
	tripStops, err := d.TripStops()
	if err != nil {
		return nil, err
	}
	return &feeds.Snapshot{Routes: routes, Stops: stops, Trips: trips, Shapes: shapes, TripStops: tripStops}, nil
}

// StoreScheduledOffsets replaces the feed's own times by pattern, kind of day and hour of departure.
func (d *Data) StoreScheduledOffsets(byTrip map[string]map[string][][]float64) error {
	clear := func(tx *sql.Tx) error {
		_, err := tx.Exec(`DELETE FROM scheduled_offsets`)
		return err
	}
	err := database.BatchReplace(d.db, clear, `INSERT INTO scheduled_offsets (trip_id, day_type, bucket_start_min, offsets) VALUES (?, ?, ?, ?)`,
		func(stmt *sql.Stmt) error {
			for tripID, byDay := range byTrip {
				for dayType, hours := range byDay {
					for hour, offsets := range hours {
						data, err := json.Marshal(offsets)
						if err != nil {
							return err
						}
						if _, err := stmt.Exec(tripID, dayType, hour*60, string(data)); err != nil {
							return err
						}
					}
				}
			}
			return nil
		})
	return err
}
