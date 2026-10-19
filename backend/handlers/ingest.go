package handlers

import (
	"conexiuni-cluj/models"
	"conexiuni-cluj/sources"
	"database/sql"
	"encoding/json"
	"fmt"
	"runtime/debug"
	"sort"
	"sync"
	"time"
)

const (
	ingestStatic     = "static"
	ingestTimetables = "timetables"
)

type ingestState struct {
	mu              sync.RWMutex
	lastStartedAt   time.Time
	lastCompletedAt time.Time
	lastDuration    time.Duration
}

type IngestSnapshot struct {
	LastStartedAt   time.Time
	LastCompletedAt time.Time
	LastDuration    time.Duration
	NextScheduledAt time.Time
}

func (city *City) IngestStatus() IngestSnapshot {
	city.ingest.mu.RLock()
	defer city.ingest.mu.RUnlock()
	return IngestSnapshot{
		LastStartedAt:   city.ingest.lastStartedAt,
		LastCompletedAt: city.ingest.lastCompletedAt,
		LastDuration:    city.ingest.lastDuration,
		NextScheduledAt: city.nextIngestAt(time.Now()),
	}
}

func (city *City) StartIngest(r *runner) {
	r.Go(func() {
		defer func() {
			if p := recover(); p != nil {
				city.log("ingest: recovered from panic: %v\n%s", p, debug.Stack())
			}
		}()
		retry := sources.StallTimeout
		for {
			if err := city.runIngest(time.Now()); err != nil {
				// Backs off up to the source's own pace.
				wait := min(retry, city.Static.Cadence())
				retry = min(retry*2, city.Static.Cadence())
				city.log("ingest: %v; next try in %s", err, wait.Round(time.Second))
				if !r.Sleep(wait) {
					return
				}
				continue
			}
			retry = sources.StallTimeout
			next := city.nextIngestAt(time.Now())
			if next.IsZero() {
				return
			}
			city.log("ingest: next at %s (in %s)", next.Format(time.RFC3339), time.Until(next).Round(time.Minute))
			if !r.Sleep(time.Until(next)) {
				return
			}
		}
	})
}

func (city *City) fetchedAt(source string) time.Time {
	var at int64
	if err := city.DB.QueryRow(`SELECT fetched_at FROM ingest_state WHERE source = ?`, source).Scan(&at); err != nil {
		return time.Time{}
	}
	return time.Unix(at, 0)
}

func (city *City) markFetched(source string, at time.Time) {
	if _, err := city.DB.Exec(`INSERT OR REPLACE INTO ingest_state (source, fetched_at) VALUES (?, ?)`, source, at.Unix()); err != nil {
		city.log("ingest: mark %s: %v", source, err)
	}
}

func (city *City) ForgetFetches() error {
	_, err := city.DB.Exec(`DELETE FROM ingest_state`)
	return err
}

// Zero when the source is never due again.
func (city *City) dueAt(source string, cadence time.Duration, now time.Time) time.Time {
	last := city.fetchedAt(source)
	switch {
	case last.IsZero():
		return now
	case city.Options.Refresh:
		return last.Add(cadence)
	}
	return time.Time{}
}

func (city *City) nextIngestAt(now time.Time) time.Time {
	next := city.dueAt(ingestStatic, city.Static.Cadence(), now)
	if city.Timetables != nil {
		if due := city.dueAt(ingestTimetables, city.Timetables.Cadence(), now); !due.IsZero() && (next.IsZero() || due.Before(next)) {
			next = due
		}
	}
	return next
}

func (city *City) runIngest(now time.Time) error {
	isDue := func(due time.Time) bool { return !due.IsZero() && !due.After(now) }
	staticDue := isDue(city.dueAt(ingestStatic, city.Static.Cadence(), now))
	timetablesDue := city.Timetables != nil && isDue(city.dueAt(ingestTimetables, city.Timetables.Cadence(), now))

	city.ingest.mu.Lock()
	city.ingest.lastStartedAt = now
	city.ingest.mu.Unlock()
	defer func() {
		city.ingest.mu.Lock()
		city.ingest.lastCompletedAt = time.Now()
		city.ingest.lastDuration = city.ingest.lastCompletedAt.Sub(now)
		city.ingest.mu.Unlock()
	}()

	var snap *sources.Snapshot
	var err error
	if staticDue {
		started := time.Now()
		if snap, err = city.Static.Snapshot(); err != nil {
			return fmt.Errorf("snapshot: %w", err)
		}
		named := time.Now()
		if renamed, err := sources.Stations(snap, city.Locate); err != nil {
			city.log("ingest: stops sharing a name keep it: %v", err)
		} else if renamed > 0 {
			city.log("ingest: %d stops sharing a name told apart in %s", renamed, time.Since(named).Round(time.Millisecond))
		}
		if err := city.storeSnapshot(snap); err != nil {
			return fmt.Errorf("store snapshot: %w", err)
		}
		city.markFetched(ingestStatic, time.Now())
		city.log("ingest: snapshot stored in %s (%d routes, %d stops, %d trips, %d shape points)", time.Since(started).Round(time.Millisecond), len(snap.Routes), len(snap.Stops), len(snap.Trips), len(snap.Shapes))
	} else if snap, err = city.loadSnapshot(); err != nil {
		return fmt.Errorf("load snapshot: %w", err)
	}

	// Returned at the end, so a fresh network is still laid out.
	var timetablesErr error
	if timetablesDue {
		started := time.Now()
		timetables, err := city.Timetables.Timetables(snap)
		if err != nil {
			timetablesErr = fmt.Errorf("timetables: %w", err)
		} else {
			snap.Timetables = timetables
			city.markFetched(ingestTimetables, time.Now())
			city.log("ingest: %d timetables in %s", len(timetables), time.Since(started).Round(time.Millisecond))
		}
	}
	for _, t := range snap.Timetables {
		if err := city.storeTimetableTrackingChanges(t); err != nil {
			city.log("ingest: timetable %s: %v", t.RouteShortName, err)
		}
	}

	if staticDue || timetablesDue {
		city.deriveStopTimes(snap)
		if orphans, err := city.ScrubStopTimes(); err != nil {
			city.log("ingest: scrub: %v", err)
		} else if orphans > 0 {
			city.log("ingest: removed %d stop times of retired trips", orphans)
		}
	}

	city.refreshAvailability()
	city.refreshCentre()
	city.refreshLiveTrips(true)
	city.WarmNetwork()
	city.WarmPlanner()
	return timetablesErr
}

// One transaction, so a reader never sees half a network.
func (city *City) storeSnapshot(snap *sources.Snapshot) error {
	tx, err := city.DB.Begin()
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
	return city.Optimize()
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

func (city *City) loadSnapshot() (*sources.Snapshot, error) {
	routes, err := city.GetRoutes(RouteFilter{})
	if err != nil {
		return nil, err
	}
	stops, err := city.GetStops(StopFilter{})
	if err != nil {
		return nil, err
	}
	trips, err := city.GetTrips(TripFilter{})
	if err != nil {
		return nil, err
	}
	shapes, err := city.GetShapes(nil)
	if err != nil {
		return nil, err
	}
	tripStops, err := city.getTripStops()
	if err != nil {
		return nil, err
	}
	return &sources.Snapshot{Routes: routes, Stops: stops, Trips: trips, Shapes: shapes, TripStops: tripStops}, nil
}

// The feed's own times when it has them, else the shape at bus pace; learning overwrites both.
func (city *City) deriveStopTimes(snap *sources.Snapshot) {
	stopsByID := make(map[int]models.Stop, len(snap.Stops))
	for _, s := range snap.Stops {
		stopsByID[s.StopID] = s
	}
	shapesByID := make(map[string][]models.Shape)
	for _, s := range snap.Shapes {
		shapesByID[s.ShapeID] = append(shapesByID[s.ShapeID], s)
	}
	tripStops := make(map[string][]models.TripStop)
	for _, ts := range snap.TripStops {
		tripStops[ts.TripID] = append(tripStops[ts.TripID], ts)
	}
	for _, group := range tripStops {
		sort.Slice(group, func(i, j int) bool { return group[i].StopSequence < group[j].StopSequence })
	}
	tripsByRoute := make(map[int][]models.Trip)
	for _, t := range snap.Trips {
		tripsByRoute[t.RouteID] = append(tripsByRoute[t.RouteID], t)
	}
	for _, route := range snap.Routes {
		var rows []models.StopTime
		for _, trip := range tripsByRoute[route.RouteID] {
			shape := shapesByID[trip.ShapeID]
			scheduled := snap.Offsets[trip.TripID]
			var previous *models.Stop
			for i, ts := range tripStops[trip.TripID] {
				current := stopsByID[ts.StopID]
				offset := 0.0
				if i < len(scheduled) && scheduled[i] > 0 {
					offset = scheduled[i]
				} else if previous != nil && ts.StopSequence != 0 && len(shape) > 0 {
					offset = calculateStopOffset(*previous, current, shape)
				}
				rows = append(rows, models.StopTime{
					TripID: trip.TripID, StopID: ts.StopID, OffsetArrivalTime: offset, StopSequence: ts.StopSequence,
					StopHeadsign: current.StopName, RouteShortName: route.RouteShortName, StopLat: current.StopLat, StopLon: current.StopLon,
				})
				previous = &current
			}
		}
		if len(rows) == 0 {
			continue
		}
		if err := city.storeStopTimesTrackingChanges(route.RouteShortName, rows); err != nil {
			city.log("ingest: stop times %s: %v", route.RouteShortName, err)
		}
	}
	if snap.HourlyOffsets != nil {
		if err := city.storeScheduledOffsets(snap.HourlyOffsets); err != nil {
			city.log("ingest: scheduled offsets: %v", err)
		}
	}
}

func (city *City) storeScheduledOffsets(byTrip map[string]map[string][][]float64) error {
	clear := func(tx *sql.Tx) error {
		_, err := tx.Exec(`DELETE FROM scheduled_offsets`)
		return err
	}
	err := batchReplace(city.DB, clear, `INSERT INTO scheduled_offsets (trip_id, day_type, bucket_start_min, offsets) VALUES (?, ?, ?, ?)`,
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
	if err != nil {
		return err
	}
	city.resetProfileSnapshots()
	city.resetTripScheduleCache()
	return nil
}

type availabilityRegistry struct {
	mu                  sync.RWMutex
	ready               bool
	routesWithTimetable map[string]struct{}
	stopsWithBuses      map[int]struct{}
}

func (city *City) refreshAvailability() {
	routesSQL := `
		SELECT route_short_name FROM timetable
		WHERE weekdays LIKE '%departure_in%' OR saturday LIKE '%departure_in%' OR sunday LIKE '%departure_in%'`
	stopsSQL := `
		SELECT DISTINCT ts.stop_id FROM trip_stops ts
		JOIN trips t ON ts.trip_id = t.trip_id
		JOIN routes r ON t.route_id = r.route_id
		JOIN timetable tt ON tt.route_short_name = r.route_short_name`
	names := func(rows *sql.Rows) (string, error) {
		var s string
		err := rows.Scan(&s)
		return s, err
	}
	routes, err := queryRows(city.DB, routesSQL, nil, names)
	// No departure times anywhere: every route with a pattern counts.
	if err == nil && len(routes) == 0 {
		stopsSQL = `SELECT DISTINCT stop_id FROM trip_stops`
		routes, err = queryRows(city.DB, `SELECT DISTINCT r.route_short_name FROM routes r JOIN trips t ON t.route_id = r.route_id`, nil, names)
	}
	if err != nil {
		city.log("availability: routes: %v", err)
		return
	}
	stops, err := queryRows(city.DB, stopsSQL, nil, func(rows *sql.Rows) (int, error) {
		var id int
		err := rows.Scan(&id)
		return id, err
	})
	if err != nil {
		city.log("availability: stops: %v", err)
		return
	}
	withTimetable := make(map[string]struct{}, len(routes))
	for _, r := range routes {
		withTimetable[r] = struct{}{}
	}
	withBuses := make(map[int]struct{}, len(stops))
	for _, id := range stops {
		withBuses[id] = struct{}{}
	}
	city.availability.mu.Lock()
	city.availability.routesWithTimetable, city.availability.stopsWithBuses, city.availability.ready = withTimetable, withBuses, true
	city.availability.mu.Unlock()
}

func (a *availabilityRegistry) IsReady() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.ready
}

func (a *availabilityRegistry) RouteHasTimetable(routeShortName string) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	_, ok := a.routesWithTimetable[routeShortName]
	return ok
}

func (a *availabilityRegistry) StopHasBuses(stopID int) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	_, ok := a.stopsWithBuses[stopID]
	return ok
}
