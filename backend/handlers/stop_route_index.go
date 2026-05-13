package handlers

import (
	"conexiuni-cluj/models"
	"conexiuni-cluj/services/tranzy"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

// StopRouteDirections records which directions of a route serve a stop.
// _0-suffixed trip → Has0; _1-suffixed trip → Has1.
type StopRouteDirections struct {
	Has0 bool
	Has1 bool
}

// stopRouteIndex is a global, in-memory lookup derived from /stop_times.
// plan_routes uses it to disambiguate trip direction (_0 vs _1) and to find
// alternative routes covering the same stop sequence without loading full
// stop_info — which previously fanned out to per-route stop_times + timetable
// lookups and drove the 30s cold-cache cascade after long idle periods.
type stopRouteIndex struct {
	mu        sync.RWMutex
	ready     bool
	byStop    map[int]map[int]StopRouteDirections
	tripStops map[string][]int
	loadSF    singleflight.Group
}

var StopRoutes = &stopRouteIndex{}

func (s *stopRouteIndex) IsReady() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ready
}

func (s *stopRouteIndex) Get(stopID, routeID int) (StopRouteDirections, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.ready {
		return StopRouteDirections{}, false
	}
	m, ok := s.byStop[stopID]
	if !ok {
		return StopRouteDirections{}, false
	}
	d, ok := m[routeID]
	return d, ok
}

// RoutesThroughStop returns the (routeID → directions) map for the given stop.
// The returned map MUST be treated as read-only; it is shared across callers.
// Returns nil if the index isn't ready or the stop has no routes.
func (s *stopRouteIndex) RoutesThroughStop(stopID int) map[int]StopRouteDirections {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.ready {
		return nil
	}
	return s.byStop[stopID]
}

// TripStops returns the ordered stop_ids served by the given (normalized) trip ID.
// The returned slice MUST be treated as read-only.
func (s *stopRouteIndex) TripStops(tripID string) []int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.ready {
		return nil
	}
	return s.tripStops[tripID]
}

// Rebuild swaps in a fresh index built from api_stop_times rows. Safe to call
// repeatedly; readers see either the old or the new index atomically.
func (s *stopRouteIndex) Rebuild(apiStopTimes []models.RequestStopTime) {
	type seqEntry struct {
		seq    int
		stopID int
	}
	tripBuffer := make(map[string][]seqEntry)
	nextByStop := make(map[int]map[int]StopRouteDirections, len(apiStopTimes)/8+1)

	for _, ast := range apiStopTimes {
		tripID := NormalizeTripID(ast.TripID)
		tripBuffer[tripID] = append(tripBuffer[tripID], seqEntry{seq: ast.StopSequence, stopID: ast.StopID})

		var routeIDStr string
		var has0, has1 bool
		switch {
		case strings.HasSuffix(tripID, OUTGOING_SUFFIX):
			routeIDStr = strings.TrimSuffix(tripID, OUTGOING_SUFFIX)
			has0 = true
		case strings.HasSuffix(tripID, INCOMING_SUFFIX):
			routeIDStr = strings.TrimSuffix(tripID, INCOMING_SUFFIX)
			has1 = true
		default:
			continue
		}
		routeID, err := strconv.Atoi(routeIDStr)
		if err != nil {
			continue
		}
		m := nextByStop[ast.StopID]
		if m == nil {
			m = make(map[int]StopRouteDirections)
			nextByStop[ast.StopID] = m
		}
		d := m[routeID]
		if has0 {
			d.Has0 = true
		}
		if has1 {
			d.Has1 = true
		}
		m[routeID] = d
	}

	nextTripStops := make(map[string][]int, len(tripBuffer))
	for tid, entries := range tripBuffer {
		slices.SortFunc(entries, func(a, b seqEntry) int { return a.seq - b.seq })
		stops := make([]int, len(entries))
		for i, e := range entries {
			stops[i] = e.stopID
		}
		nextTripStops[tid] = stops
	}

	s.mu.Lock()
	s.byStop = nextByStop
	s.tripStops = nextTripStops
	s.ready = true
	s.mu.Unlock()
}

// EnsureReady loads api_stop_times (cached) and rebuilds the index when it
// hasn't been built yet. Concurrent callers dedupe via singleflight. Used by
// plan_routes for the rare case where a request arrives before warmup has
// reached its api_stop_times phase.
func (s *stopRouteIndex) EnsureReady(tranzyClient *tranzy.Client, shelfLife time.Duration) {
	if s.IsReady() {
		return
	}
	_, _, _ = s.loadSF.Do("load", func() (any, error) {
		if s.IsReady() {
			return nil, nil
		}
		apiStopTimes, err := getAPIStopTimes(tranzyClient, shelfLife, APIStopTimeFilter{})
		if err != nil || len(apiStopTimes) == 0 {
			return nil, err
		}
		s.Rebuild(apiStopTimes)
		return nil, nil
	})
}

// tripCoversSequence returns true if the trip's ordered stop list visits every
// stop in required in order (other stops may appear between them).
func tripCoversSequence(tripStops, required []int) bool {
	if len(required) == 0 || len(tripStops) < len(required) {
		return false
	}
	reqIdx := 0
	for _, s := range tripStops {
		if s == required[reqIdx] {
			reqIdx++
			if reqIdx == len(required) {
				return true
			}
		}
	}
	return false
}
