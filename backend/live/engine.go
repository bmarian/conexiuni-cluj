package live

import (
	"conexiuni-cluj/database"
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/jobs"
	"conexiuni-cluj/models"
	"conexiuni-cluj/transit"
	"log"
	"time"
)

type Options struct {
	VehicleInterval  VehicleIntervalConfig
	LearningEnabled  bool
	LearningMaxQuota int
	// False reuses whatever the database holds.
	Refresh bool
}

// Engine is a city's live side: where its vehicles are and when they arrive, what it learns from them, and its calendar.
type Engine struct {
	Slug     string
	Location *time.Location
	*database.Store
	*transit.Data
	// nil for a schedule-only city.
	Realtime feeds.Realtime
	Learning feeds.Realtime
	Options  Options
	// What the network offers, kept by the city.
	offered *transit.Availability
	// The city's background work while it runs.
	Runner func() *jobs.Runner
	// Called once the calendar reads differently, for what was built from it elsewhere.
	ScheduleChanged func()
	SchoolCalendar  SchoolCalendar

	engineState
}

// New is a city's live side; learning defaults to realtime.
func New(slug string, loc *time.Location, store *database.Store, data *transit.Data, offered *transit.Availability, realtime, learning feeds.Realtime, opts Options) *Engine {
	if learning == nil {
		learning = realtime
	}
	eng := &Engine{Slug: slug, Location: loc, Store: store, Data: data, Realtime: realtime, Learning: learning, Options: opts, offered: offered,
		Runner: jobs.None, ScheduleChanged: func() {}}
	eng.engineState = newEngineState()
	eng.hub = newLiveHub(eng)
	return eng
}

func (eng *Engine) running() *jobs.Runner { return eng.Runner() }

// RouteIDs implements feeds.Tracing.
func (eng *Engine) RouteIDs() map[string]int {
	eng.liveTrips.RLock()
	defer eng.liveTrips.RUnlock()
	return eng.liveTrips.routeIDs
}

// Along implements feeds.Tracing.
func (eng *Engine) Along(routeID, direction int, lat, lon float64) (float64, float64, bool) {
	eng.liveTrips.RLock()
	lt := eng.liveTrips.byTrip[models.PatternID(routeID, direction)]
	eng.liveTrips.RUnlock()
	if lt == nil || lt.trip.Shape == nil || len(lt.trip.Shape.Points) == 0 {
		return 0, 0, false
	}
	idx, off := lt.trip.Shape.Closest(lat, lon)
	return lt.trip.Shape.Cumulative[idx], off, true
}

// StopIndex implements feeds.Placement.
func (eng *Engine) StopIndex(routeID, direction, stopID int) int {
	eng.liveTrips.RLock()
	lt := eng.liveTrips.byTrip[models.PatternID(routeID, direction)]
	eng.liveTrips.RUnlock()
	if lt == nil {
		return -1
	}
	for i, stop := range lt.trip.Stops {
		if stop.ID == stopID {
			return i
		}
	}
	return -1
}

func (eng *Engine) log(format string, args ...any) {
	log.Printf(eng.Slug+": "+format, args...)
}

// NetworkStored forgets the patterns' geometry, once a new network is stored.
func (eng *Engine) NetworkStored() { eng.resetTripSegmentTrackers() }

// OffsetsStored forgets the times built from the feed's hourly offsets, once new ones are stored.
func (eng *Engine) OffsetsStored() {
	eng.resetProfileSnapshots()
	eng.resetTripScheduleCache()
}

// RefreshTrips lays the live trips out again from the stored network.
func (eng *Engine) RefreshTrips() { eng.refreshLiveTrips(true) }
