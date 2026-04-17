package handlers

import "sync"

// Tracks which routes have a functional CTP timetable and which stops have at
// least one route with live data. Populated by the warmup goroutine after each
// full pass; consumed by the /api/routes and /api/stops endpoints to hide
// discontinued routes and empty stops from the UI.
//
// Before the first warmup pass completes, IsReady() returns false and callers
// must skip the filter — returning an empty list on a cold server would be
// worse than returning a few unusable entries.
type availabilityRegistry struct {
	mu                  sync.RWMutex
	ready               bool
	routesWithTimetable map[string]struct{}
	stopsWithBuses      map[int]struct{}
}

var Availability = &availabilityRegistry{
	routesWithTimetable: make(map[string]struct{}),
	stopsWithBuses:      make(map[int]struct{}),
}

// ResetForNewPass clears the previous pass's entries. Callers should populate
// via Mark* and then call MarkReady() when the pass finishes. Resetting on
// every pass means routes that lose their CTP CSV mid-week disappear from
// /api/routes after the next warmup.
func (a *availabilityRegistry) ResetForNewPass() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.routesWithTimetable = make(map[string]struct{})
	a.stopsWithBuses = make(map[int]struct{})
}

func (a *availabilityRegistry) MarkRouteHasTimetable(routeShortName string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.routesWithTimetable[routeShortName] = struct{}{}
}

func (a *availabilityRegistry) MarkStopHasBuses(stopID int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.stopsWithBuses[stopID] = struct{}{}
}

func (a *availabilityRegistry) MarkReady() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.ready = true
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
