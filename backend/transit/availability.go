package transit

import (
	"conexiuni-cluj/database"
	"database/sql"
	"fmt"
	"sync"
)

// Availability is which lines have departures and which stops they serve: what the network offers.
type Availability struct {
	mu                  sync.RWMutex
	ready               bool
	routesWithTimetable map[string]struct{}
	stopsWithBuses      map[int]struct{}
}

func (a *Availability) Refresh(d *Data) error {
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
	routes, err := database.QueryRows(d.db, routesSQL, nil, names)
	// No departure times anywhere: every route with a pattern counts.
	if err == nil && len(routes) == 0 {
		stopsSQL = `SELECT DISTINCT stop_id FROM trip_stops`
		routes, err = database.QueryRows(d.db, `SELECT DISTINCT r.route_short_name FROM routes r JOIN trips t ON t.route_id = r.route_id`, nil, names)
	}
	if err != nil {
		return fmt.Errorf("routes: %w", err)
	}
	stops, err := database.QueryRows(d.db, stopsSQL, nil, func(rows *sql.Rows) (int, error) {
		var id int
		err := rows.Scan(&id)
		return id, err
	})
	if err != nil {
		return fmt.Errorf("stops: %w", err)
	}
	withTimetable := make(map[string]struct{}, len(routes))
	for _, r := range routes {
		withTimetable[r] = struct{}{}
	}
	withBuses := make(map[int]struct{}, len(stops))
	for _, id := range stops {
		withBuses[id] = struct{}{}
	}
	a.mu.Lock()
	a.routesWithTimetable, a.stopsWithBuses, a.ready = withTimetable, withBuses, true
	a.mu.Unlock()
	return nil
}

func (a *Availability) IsReady() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.ready
}

func (a *Availability) RouteHasTimetable(routeShortName string) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	_, ok := a.routesWithTimetable[routeShortName]
	return ok
}

func (a *Availability) StopHasBuses(stopID int) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	_, ok := a.stopsWithBuses[stopID]
	return ok
}
