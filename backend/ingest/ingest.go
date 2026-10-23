// Package ingest reads a city's feeds into its database when they are due, and derives the stop times the rest reads.
package ingest

import (
	"conexiuni-cluj/database"
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/geo"
	"conexiuni-cluj/jobs"
	"conexiuni-cluj/models"
	"conexiuni-cluj/transit"
	"conexiuni-cluj/updates"
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

type Importer struct {
	store   *database.Store
	data    *transit.Data
	offered *transit.Availability
	updates *updates.Service

	Static     feeds.Static
	Timetables feeds.Timetables
	Locate     feeds.Locate
	// False reuses whatever the database holds.
	Refresh bool

	// Called within a run, in this order: once the network is stored, once the feed's hourly times are, and at the end of every run.
	NetworkStored func()
	OffsetsStored func()
	Imported      func(stored bool)

	mu              sync.RWMutex
	lastStartedAt   time.Time
	lastCompletedAt time.Time
	lastDuration    time.Duration
}

func New(store *database.Store, data *transit.Data, offered *transit.Availability, updates *updates.Service) *Importer {
	nothing := func() {}
	return &Importer{store: store, data: data, offered: offered, updates: updates, NetworkStored: nothing, OffsetsStored: nothing, Imported: func(bool) {}}
}

type Status struct {
	LastStartedAt   time.Time
	LastCompletedAt time.Time
	LastDuration    time.Duration
	NextScheduledAt time.Time
}

func (im *Importer) Status() Status {
	im.mu.RLock()
	defer im.mu.RUnlock()
	return Status{
		LastStartedAt:   im.lastStartedAt,
		LastCompletedAt: im.lastCompletedAt,
		LastDuration:    im.lastDuration,
		NextScheduledAt: im.nextIngestAt(time.Now()),
	}
}

func (im *Importer) Start(r *jobs.Runner) {
	r.Go(func() {
		defer func() {
			if p := recover(); p != nil {
				im.store.Logf("ingest: recovered from panic: %v\n%s", p, debug.Stack())
			}
		}()
		retry := feeds.StallTimeout
		for {
			if err := im.Run(time.Now()); err != nil {
				// Backs off up to the source's own pace.
				wait := min(retry, im.Static.Cadence())
				retry = min(retry*2, im.Static.Cadence())
				im.store.Logf("ingest: %v; next try in %s", err, wait.Round(time.Second))
				if !r.Sleep(wait) {
					return
				}
				continue
			}
			retry = feeds.StallTimeout
			next := im.nextIngestAt(time.Now())
			if next.IsZero() {
				return
			}
			im.store.Logf("ingest: next at %s (in %s)", next.Format(time.RFC3339), time.Until(next).Round(time.Minute))
			if !r.Sleep(time.Until(next)) {
				return
			}
		}
	})
}

// Zero when the source is never due again.
func (im *Importer) dueAt(source string, cadence time.Duration, now time.Time) time.Time {
	last := im.store.FetchedAt(source)
	switch {
	case last.IsZero():
		return now
	case im.Refresh:
		return last.Add(cadence)
	}
	return time.Time{}
}

func (im *Importer) nextIngestAt(now time.Time) time.Time {
	next := im.dueAt(ingestStatic, im.Static.Cadence(), now)
	if im.Timetables != nil {
		if due := im.dueAt(ingestTimetables, im.Timetables.Cadence(), now); !due.IsZero() && (next.IsZero() || due.Before(next)) {
			next = due
		}
	}
	return next
}

// Run reads what is due at now and lays the network out again.
func (im *Importer) Run(now time.Time) error {
	isDue := func(due time.Time) bool { return !due.IsZero() && !due.After(now) }
	staticDue := isDue(im.dueAt(ingestStatic, im.Static.Cadence(), now))
	timetablesDue := im.Timetables != nil && isDue(im.dueAt(ingestTimetables, im.Timetables.Cadence(), now))

	im.mu.Lock()
	im.lastStartedAt = now
	im.mu.Unlock()
	defer func() {
		im.mu.Lock()
		im.lastCompletedAt = time.Now()
		im.lastDuration = im.lastCompletedAt.Sub(now)
		im.mu.Unlock()
	}()

	var snap *feeds.Snapshot
	var err error
	stored := false
	if staticDue {
		started := time.Now()
		if snap, err = im.Static.Snapshot(); err != nil {
			return fmt.Errorf("snapshot: %w", err)
		}
		named := time.Now()
		if renamed, err := feeds.Stations(snap, im.Locate); err != nil {
			im.store.Logf("ingest: stops sharing a name keep it: %v", err)
		} else if renamed > 0 {
			im.store.Logf("ingest: %d stops sharing a name told apart in %s", renamed, time.Since(named).Round(time.Millisecond))
		}
		if err := im.data.StoreNetwork(snap); err != nil {
			return fmt.Errorf("store snapshot: %w", err)
		}
		im.NetworkStored()
		stored = true
		im.store.MarkFetched(ingestStatic, time.Now())
		im.store.Logf("ingest: snapshot stored in %s (%d routes, %d stops, %d trips, %d shape points)", time.Since(started).Round(time.Millisecond), len(snap.Routes), len(snap.Stops), len(snap.Trips), len(snap.Shapes))
	} else if snap, err = im.data.Snapshot(); err != nil {
		return fmt.Errorf("load snapshot: %w", err)
	}

	// Returned at the end, so a fresh network is still laid out.
	var timetablesErr error
	if timetablesDue {
		started := time.Now()
		timetables, err := im.Timetables.Timetables(snap)
		if err != nil {
			timetablesErr = fmt.Errorf("timetables: %w", err)
		} else {
			snap.Timetables, stored = timetables, true
			im.store.MarkFetched(ingestTimetables, time.Now())
			im.store.Logf("ingest: %d timetables in %s", len(timetables), time.Since(started).Round(time.Millisecond))
		}
	}
	for _, t := range snap.Timetables {
		if err := im.updates.TrackTimetable(t); err != nil {
			im.store.Logf("ingest: timetable %s: %v", t.RouteShortName, err)
		}
	}

	if staticDue || timetablesDue {
		im.deriveStopTimes(snap)
		if orphans, err := im.data.ScrubStopTimes(); err != nil {
			im.store.Logf("ingest: scrub: %v", err)
		} else if orphans > 0 {
			im.store.Logf("ingest: removed %d stop times of retired trips", orphans)
		}
	}

	if err := im.offered.Refresh(im.data); err != nil {
		im.store.Logf("availability: %v", err)
	}
	im.Imported(stored)
	return timetablesErr
}

// The feed's own times when it has them, else the shape at bus pace; learning overwrites both.
func (im *Importer) deriveStopTimes(snap *feeds.Snapshot) {
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
					offset = geo.StopOffset(*previous, current, shape)
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
		if err := im.updates.TrackStopTimes(route.RouteShortName, rows); err != nil {
			im.store.Logf("ingest: stop times %s: %v", route.RouteShortName, err)
		}
	}
	if snap.HourlyOffsets != nil {
		if err := im.data.StoreScheduledOffsets(snap.HourlyOffsets); err != nil {
			im.store.Logf("ingest: scheduled offsets: %v", err)
		} else {
			im.OffsetsStored()
		}
	}
}
