package live

import (
	"conexiuni-cluj/calendar"
	"conexiuni-cluj/database"
	"conexiuni-cluj/geo"
	"conexiuni-cluj/models"
	"conexiuni-cluj/predictions"
	"conexiuni-cluj/transit"
	"database/sql"
	"fmt"
	"math"
	"sort"
	"strconv"
	"sync"
	"time"
)

const (
	segmentBucketMinutes       = 60
	allDaySegmentBucket        = -1
	minSegmentProfileSamples   = 5
	segmentProfileNeighborMins = 120
	// How long what vehicles were seen doing is kept to learn from.
	SampleRetention            = 45 * 24 * time.Hour
	segmentProfileRetention    = 400 * 24 * time.Hour
	MaxVehicleAge              = 10 * time.Minute
	minSegmentDurationSec      = 8
	maxSegmentDurationSec      = 45 * 60
	minObservedSegmentKmh      = 1.5
	maxObservedSegmentKmh      = predictions.MaxBusSpeedKmh
	segmentStopProximityMeters = 80.0
)

type segmentProfileKey struct {
	RouteID        int
	DirectionID    int
	FromStopID     int
	ToStopID       int
	DayType        string
	BucketStartMin int
}

type stopPair struct {
	FromStopID int
	ToStopID   int
}

type trackedSegmentStop struct {
	StopID   int
	ShapeIdx int
	Lat      float64
	Lon      float64
}

type tripSegmentTracker struct {
	TripID         string
	RouteID        int
	RouteShortName string
	DirectionID    int
	Shape          []models.Shape
	Cumulative     []float64
	Stops          []trackedSegmentStop
}

type vehicleSegmentState struct {
	TripID           string
	ShapeIdx         int
	SegmentStartPos  int
	SegmentStartedAt time.Time
	LastObservedAt   time.Time
	HasSegmentStart  bool
	// Timetable departure this run was matched to near its start; -1 until then.
	MatchedDepartureMin float64
	MatchedDayType      string
}

var segmentClock = time.Now

// A paired slot may sit this much before the feeder run's learned arrival.
const pairedSlotMarginMin = 3.0

type pendingSegmentSample struct {
	Key        segmentProfileKey
	Duration   time.Duration
	DistanceM  float64
	ObservedAt time.Time
}

type segmentObservationStatus string

const (
	segmentObservedAccepted    segmentObservationStatus = "accepted"
	segmentObservedInvalid     segmentObservationStatus = "invalid"
	segmentObservedStale       segmentObservationStatus = "stale"
	segmentObservedNoTracker   segmentObservationStatus = "no_tracker"
	segmentObservedReset       segmentObservationStatus = "reset"
	segmentObservedDuplicate   segmentObservationStatus = "duplicate"
	segmentObservedNoProgress  segmentObservationStatus = "no_progress"
	segmentObservedNonAdjacent segmentObservationStatus = "non_adjacent"
	segmentObservedRejected    segmentObservationStatus = "rejected"
)

type segmentObservationSummary struct {
	Total       int
	Accepted    int
	Invalid     int
	Stale       int
	NoTracker   int
	Reset       int
	Duplicate   int
	NoProgress  int
	NonAdjacent int
	Rejected    int
}

type segmentWriteSummary struct {
	Stored            int
	SampleInsertError int
	ProfilesCreated   int
	ProfilesUpdated   int
	ProfilesUnchanged int
	ProfileError      int
}

type segmentProfileWriteResult int

const (
	segmentProfileUnchanged segmentProfileWriteResult = iota
	segmentProfileCreated
	segmentProfileUpdated
	segmentProfileFailed
)

func (eng *Engine) ObserveVehicleSegmentTravelTimes(loc *time.Location, vehicles []models.Vehicle) {
	if loc == nil {
		loc = time.Local
	}
	summary := segmentObservationSummary{Total: len(vehicles)}
	writes := segmentWriteSummary{}
	for _, v := range vehicles {
		sample, status := eng.observeVehicleSegment(loc, v)
		summary.add(status)
		if status == segmentObservedAccepted {
			writes.add(eng.storeSegmentTravelSample(sample))
		}
	}
	turnarounds := eng.flushTurnaroundSamples()
	if summary.Total > 0 {
		eng.log("segment travel: snapshot vehicles=%d accepted=%d stored=%d turnarounds=%d profiles_created=%d profiles_updated=%d profiles_unchanged=%d rejected=%d ignored_reset=%d ignored_duplicate=%d ignored_no_progress=%d ignored_non_adjacent=%d ignored_no_tracker=%d stale=%d invalid=%d sample_errors=%d profile_errors=%d",
			summary.Total, summary.Accepted, writes.Stored, turnarounds,
			writes.ProfilesCreated, writes.ProfilesUpdated, writes.ProfilesUnchanged,
			summary.Rejected, summary.Reset, summary.Duplicate, summary.NoProgress, summary.NonAdjacent, summary.NoTracker,
			summary.Stale, summary.Invalid, writes.SampleInsertError, writes.ProfileError)
	}
}

func (s *segmentObservationSummary) add(status segmentObservationStatus) {
	switch status {
	case segmentObservedAccepted:
		s.Accepted++
	case segmentObservedInvalid:
		s.Invalid++
	case segmentObservedStale:
		s.Stale++
	case segmentObservedNoTracker:
		s.NoTracker++
	case segmentObservedReset:
		s.Reset++
	case segmentObservedDuplicate:
		s.Duplicate++
	case segmentObservedNoProgress:
		s.NoProgress++
	case segmentObservedNonAdjacent:
		s.NonAdjacent++
	case segmentObservedRejected:
		s.Rejected++
	}
}

func (s *segmentWriteSummary) add(other segmentWriteSummary) {
	s.Stored += other.Stored
	s.SampleInsertError += other.SampleInsertError
	s.ProfilesCreated += other.ProfilesCreated
	s.ProfilesUpdated += other.ProfilesUpdated
	s.ProfilesUnchanged += other.ProfilesUnchanged
	s.ProfileError += other.ProfileError
}

func (s *segmentWriteSummary) addProfileResult(result segmentProfileWriteResult) {
	switch result {
	case segmentProfileCreated:
		s.ProfilesCreated++
	case segmentProfileUpdated:
		s.ProfilesUpdated++
	case segmentProfileFailed:
		s.ProfileError++
	default:
		s.ProfilesUnchanged++
	}
}

func (eng *Engine) observeVehicleSegment(loc *time.Location, v models.Vehicle) (pendingSegmentSample, segmentObservationStatus) {
	if v.ID == 0 || v.Latitude <= 0 || v.Longitude <= 0 || v.RouteID < 0 || v.TripID == "" || v.TripID == "-1" {
		return pendingSegmentSample{}, segmentObservedInvalid
	}
	observedAt, err := time.Parse(time.RFC3339, v.Timestamp)
	if err != nil {
		return pendingSegmentSample{}, segmentObservedInvalid
	}
	observedAt = observedAt.In(loc)
	if now := segmentClock(); now.Sub(observedAt) > MaxVehicleAge || observedAt.Sub(now) > 2*time.Minute {
		return pendingSegmentSample{}, segmentObservedStale
	}

	tracker, ok := eng.getTripSegmentTracker(v.TripID)
	if !ok || len(tracker.Stops) < 2 || len(tracker.Shape) < 2 {
		return pendingSegmentSample{}, segmentObservedNoTracker
	}

	shapeIdx := geo.ClosestShapeIndex(v.Latitude, v.Longitude, tracker.Shape)
	routePos := tracker.stopPosForShapeIdx(shapeIdx)
	nearStopPos := tracker.nearStopPos(v.Latitude, v.Longitude, shapeIdx)

	eng.vehicleSegmentStates.Lock()
	defer eng.vehicleSegmentStates.Unlock()

	state, exists := eng.vehicleSegmentStates.byVehicle[v.ID]
	// Same GPS report served again: no new information, keep the in-flight segment.
	if exists && state.TripID == tracker.TripID && !observedAt.After(state.LastObservedAt) {
		return pendingSegmentSample{}, segmentObservedDuplicate
	}
	if !exists || state.TripID != tracker.TripID || observedAt.After(state.LastObservedAt.Add(25*time.Minute)) ||
		shapeIdx+3 < state.ShapeIdx || (routePos < state.SegmentStartPos && nearStopPos != state.SegmentStartPos) {
		sample, status := pendingSegmentSample{}, segmentObservedReset
		gap := !exists || observedAt.After(state.LastObservedAt.Add(25*time.Minute))
		if !gap && state.TripID != tracker.TripID {
			sample, status = eng.closeTripAtTurnaround(state, tracker, v, observedAt)
		} else if gap {
			eng.clearPendingTurnaround(v.ID)
		}
		eng.vehicleSegmentStates.byVehicle[v.ID] = newVehicleSegmentState(tracker, shapeIdx, routePos, nearStopPos, observedAt)
		return sample, status
	}

	state.ShapeIdx = shapeIdx
	state.LastObservedAt = observedAt
	if nearStopPos < 0 {
		eng.vehicleSegmentStates.byVehicle[v.ID] = state
		return pendingSegmentSample{}, segmentObservedNoProgress
	}
	if nearStopPos <= state.SegmentStartPos {
		// A valid start keeps its time so dwell counts, except at the departure terminus: the layover isn't travel.
		if nearStopPos == state.SegmentStartPos && (!state.HasSegmentStart || nearStopPos == 0) && nearStopPos < len(tracker.Stops)-1 {
			state.HasSegmentStart = true
			state.SegmentStartedAt = observedAt
		}
		eng.vehicleSegmentStates.byVehicle[v.ID] = state
		return pendingSegmentSample{}, segmentObservedNoProgress
	}

	eng.noteArrival(v.ID, tracker.TripID, tracker.Stops[nearStopPos].StopID, observedAt)
	if state.SegmentStartPos == 0 {
		eng.recordTurnaroundDeparture(v.ID, tracker.TripID, state.SegmentStartedAt)
	}

	// Only stops after the one the run was matched at: that match already sat close to the timetable.
	if state.MatchedDepartureMin >= 0 {
		eng.noteTimetableArrival(tracker, nearStopPos, state.MatchedDepartureMin, observedAt, loc)
	}

	var sample pendingSegmentSample
	status := segmentObservedReset
	if state.HasSegmentStart {
		status = segmentObservedNonAdjacent
		if nearStopPos == state.SegmentStartPos+1 {
			from := tracker.Stops[state.SegmentStartPos]
			to := tracker.Stops[nearStopPos]
			sample = pendingSegmentSample{
				Key: segmentProfileKey{
					RouteID:        tracker.RouteID,
					DirectionID:    tracker.DirectionID,
					FromStopID:     from.StopID,
					ToStopID:       to.StopID,
					DayType:        eng.profileDay(observedAt),
					BucketStartMin: segmentBucketStartMin(observedAt),
				},
				Duration:   observedAt.Sub(state.SegmentStartedAt),
				DistanceM:  tracker.distanceBetweenStopPositions(state.SegmentStartPos, nearStopPos),
				ObservedAt: observedAt,
			}
			if isPlausibleSegmentSample(sample) {
				status = segmentObservedAccepted
				if nearStopPos <= scheduleDelayMaxAnchorPos && state.MatchedDepartureMin < 0 {
					state.MatchedDepartureMin, state.MatchedDayType = eng.matchRunDeparture(tracker, nearStopPos, observedAt, loc)
				}
			} else {
				status = segmentObservedRejected
			}
		}
	}

	state.SegmentStartPos = nearStopPos
	state.SegmentStartedAt = observedAt
	state.HasSegmentStart = nearStopPos < len(tracker.Stops)-1
	eng.vehicleSegmentStates.byVehicle[v.ID] = state
	return sample, status
}

func newVehicleSegmentState(tracker *tripSegmentTracker, shapeIdx, routePos, nearStopPos int, observedAt time.Time) vehicleSegmentState {
	startPos := routePos
	hasSegmentStart := false
	if nearStopPos >= 0 {
		startPos = nearStopPos
		hasSegmentStart = nearStopPos < len(tracker.Stops)-1
	}
	return vehicleSegmentState{
		TripID:              tracker.TripID,
		ShapeIdx:            shapeIdx,
		SegmentStartPos:     startPos,
		SegmentStartedAt:    observedAt,
		LastObservedAt:      observedAt,
		HasSegmentStart:     hasSegmentStart,
		MatchedDepartureMin: -1,
	}
}

// Tranzy moved the bus onto its next trip: at a terminus that is the arrival.
func (eng *Engine) closeTripAtTurnaround(state vehicleSegmentState, next *tripSegmentTracker, v models.Vehicle, observedAt time.Time) (pendingSegmentSample, segmentObservationStatus) {
	eng.clearPendingTurnaround(v.ID)
	prev, ok := eng.getTripSegmentTracker(state.TripID)
	if !ok || prev.RouteID != next.RouteID || prev.DirectionID == next.DirectionID || len(prev.Stops) < 2 {
		return pendingSegmentSample{}, segmentObservedReset
	}
	first := next.Stops[0]
	if geo.Haversine(v.Latitude, v.Longitude, first.Lat, first.Lon) > turnaroundArrivalRadius {
		return pendingSegmentSample{}, segmentObservedReset
	}
	eng.setPendingTurnaround(v.ID, pendingTurnaround{
		RouteID: next.RouteID, DirectionID: next.DirectionID, TripID: next.TripID, ArrivedAt: observedAt,
		PairedDepartureMin: eng.pairedDeparture(prev, next, state),
	})

	lastPos := len(prev.Stops) - 1
	if !state.HasSegmentStart || state.SegmentStartPos != lastPos-1 {
		return pendingSegmentSample{}, segmentObservedReset
	}
	sample := pendingSegmentSample{
		Key: segmentProfileKey{
			RouteID: prev.RouteID, DirectionID: prev.DirectionID,
			FromStopID: prev.Stops[lastPos-1].StopID, ToStopID: prev.Stops[lastPos].StopID,
			DayType: eng.profileDay(observedAt), BucketStartMin: segmentBucketStartMin(observedAt),
		},
		Duration:   observedAt.Sub(state.SegmentStartedAt),
		DistanceM:  prev.distanceBetweenStopPositions(lastPos-1, lastPos),
		ObservedAt: observedAt,
	}
	if !isPlausibleSegmentSample(sample) {
		return pendingSegmentSample{}, segmentObservedRejected
	}
	return sample, segmentObservedAccepted
}

// The departure of the new direction that the timetable pairs with the run just finished.
func (eng *Engine) pairedDeparture(prev, next *tripSegmentTracker, state vehicleSegmentState) float64 {
	if state.MatchedDepartureMin < 0 || state.MatchedDayType == "" {
		return -1
	}
	bucket := (int(state.MatchedDepartureMin) / segmentBucketMinutes) * segmentBucketMinutes
	offsets := eng.tripCumulativeOffsets(prev, state.MatchedDayType, bucket)
	if len(offsets) == 0 {
		return -1
	}
	arrival := state.MatchedDepartureMin + offsets[len(offsets)-1]/60
	departures := eng.tripDepartures(next, state.MatchedDayType)
	for _, d := range departures {
		if d >= arrival-pairedSlotMarginMin {
			return d
		}
	}
	for _, d := range departures {
		if d+1440 >= arrival-pairedSlotMarginMin && d+1440 <= arrival+60 {
			return d
		}
	}
	return -1
}

func (eng *Engine) matchRunDeparture(tracker *tripSegmentTracker, pos int, at time.Time, loc *time.Location) (float64, string) {
	dayType := eng.profileDay(at)
	departures := eng.tripDepartures(tracker, dayType)
	if len(departures) < 2 {
		return -1, ""
	}
	minutes, bucket := localMinutes(float64(at.Unix()), loc)
	offsets := eng.tripCumulativeOffsets(tracker, dayType, bucket)
	if pos >= len(offsets) {
		return -1, ""
	}
	departure, ok := matchDeparture(departures, minutes-offsets[pos]/60)
	if !ok {
		return -1, ""
	}
	return departure, dayType
}

func (eng *Engine) tripScheduleCacheFor(period int64) {
	if eng.tripScheduleCache.period == period && eng.tripScheduleCache.departures != nil {
		return
	}
	eng.tripScheduleCache.period = period
	eng.tripScheduleCache.departures = make(map[string][]float64)
	eng.tripScheduleCache.offsets = make(map[string][]float64)
}

func (eng *Engine) resetTripScheduleCache() {
	eng.tripScheduleCache.Lock()
	eng.tripScheduleCache.departures = nil
	eng.tripScheduleCache.offsets = nil
	eng.tripScheduleCache.Unlock()
}

func (eng *Engine) tripDepartures(tracker *tripSegmentTracker, dayType string) []float64 {
	key := tracker.TripID + "|" + dayType
	eng.tripScheduleCache.Lock()
	eng.tripScheduleCacheFor(ProfileSnapshotPeriod())
	deps, ok := eng.tripScheduleCache.departures[key]
	eng.tripScheduleCache.Unlock()
	if ok {
		return deps
	}
	deps = []float64{}
	if eng.DB == nil {
		return deps
	}
	if timetable, err := eng.Timetable(tracker.RouteShortName); err == nil && timetable != nil {
		deps = transit.Departures(timetable, dayType, tracker.DirectionID)
	}
	eng.tripScheduleCache.Lock()
	eng.tripScheduleCacheFor(ProfileSnapshotPeriod())
	eng.tripScheduleCache.departures[key] = deps
	eng.tripScheduleCache.Unlock()
	return deps
}

// Seconds from the trip's first stop to each stop, with this hour's learned segments.
func (eng *Engine) tripCumulativeOffsets(tracker *tripSegmentTracker, dayType string, bucket int) []float64 {
	return eng.cachedTripOffsets(tracker, tracker.TripID+"|"+dayType+"|"+strconv.Itoa(bucket), func(trip []models.StopTime, ordered []int) {
		eng.timeTrip(trip, ordered, tracker.TripID, dayType, bucket, eng.snapshotSegmentProfiles(tracker.RouteID, tracker.DirectionID, dayType, bucket))
	})
}

func (eng *Engine) tripSeedOffsets(tracker *tripSegmentTracker, dayType string, bucket int) []float64 {
	return eng.cachedTripOffsets(tracker, tracker.TripID+"|seed|"+dayType+"|"+strconv.Itoa(bucket), func(trip []models.StopTime, ordered []int) {
		eng.timeTrip(trip, ordered, tracker.TripID, dayType, bucket, nil)
	})
}

func (eng *Engine) cachedTripOffsets(tracker *tripSegmentTracker, key string, learn func(trip []models.StopTime, ordered []int)) []float64 {
	eng.tripScheduleCache.Lock()
	eng.tripScheduleCacheFor(ProfileSnapshotPeriod())
	offsets, ok := eng.tripScheduleCache.offsets[key]
	eng.tripScheduleCache.Unlock()
	if ok {
		return offsets
	}
	offsets = []float64{}
	if eng.DB == nil {
		return offsets
	}
	if rows, err := eng.StopTimesForTrips([]string{tracker.TripID}); err == nil && len(rows) > 0 {
		trip := transit.TripStopTimes(rows, tracker.TripID)
		ordered := make([]int, len(trip))
		for i := range ordered {
			ordered[i] = i
		}
		if learn != nil {
			learn(trip, ordered)
		}
		total := 0.0
		offsets = make([]float64, len(trip))
		for i, st := range trip {
			if i > 0 {
				total += st.OffsetArrivalTime
			}
			offsets[i] = total
		}
	}
	eng.tripScheduleCache.Lock()
	eng.tripScheduleCacheFor(ProfileSnapshotPeriod())
	eng.tripScheduleCache.offsets[key] = offsets
	eng.tripScheduleCache.Unlock()
	return offsets
}

func isPlausibleSegmentSample(sample pendingSegmentSample) bool {
	durationSec := sample.Duration.Seconds()
	if durationSec < minSegmentDurationSec || durationSec > maxSegmentDurationSec || sample.DistanceM <= 0 {
		return false
	}
	speedKmh := (sample.DistanceM / 1000) / (durationSec / 3600)
	return speedKmh >= minObservedSegmentKmh && speedKmh <= maxObservedSegmentKmh
}

func (eng *Engine) getTripSegmentTracker(tripID string) (*tripSegmentTracker, bool) {
	eng.tripSegmentTrackers.RLock()
	if tracker, ok := eng.tripSegmentTrackers.byTrip[tripID]; ok {
		eng.tripSegmentTrackers.RUnlock()
		return tracker, true
	}
	eng.tripSegmentTrackers.RUnlock()

	tracker, ok := eng.loadTripSegmentTracker(tripID)
	if !ok {
		return nil, false
	}

	eng.tripSegmentTrackers.Lock()
	eng.tripSegmentTrackers.byTrip[tripID] = tracker
	eng.tripSegmentTrackers.Unlock()
	return tracker, true
}

func (eng *Engine) resetTripSegmentTrackers() {
	eng.tripSegmentTrackers.Lock()
	eng.tripSegmentTrackers.byTrip = make(map[string]*tripSegmentTracker)
	eng.tripSegmentTrackers.Unlock()
}

func (eng *Engine) loadTripSegmentTracker(tripID string) (*tripSegmentTracker, bool) {
	trips, err := eng.Trips(transit.TripFilter{TripID: &tripID})
	if err != nil || len(trips) == 0 {
		return nil, false
	}
	trip := trips[0]
	shapeID := trip.ShapeID
	shapes, err := eng.Shapes([]string{shapeID})
	if err != nil || len(shapes) < 2 {
		return nil, false
	}
	sort.Slice(shapes, func(i, j int) bool {
		return shapes[i].ShapePtSequence < shapes[j].ShapePtSequence
	})

	apiStopTimes, err := database.QueryRows(eng.DB, `SELECT trip_id, stop_id, stop_sequence FROM trip_stops WHERE trip_id = ? ORDER BY stop_sequence`, []any{tripID},
		func(rows *sql.Rows) (models.TripStop, error) {
			var st models.TripStop
			err := rows.Scan(&st.TripID, &st.StopID, &st.StopSequence)
			return st, err
		})
	if err != nil || len(apiStopTimes) < 2 {
		return nil, false
	}

	allStops, err := eng.Stops(transit.StopFilter{})
	if err != nil || len(allStops) == 0 {
		return nil, false
	}
	stopByID := make(map[int]models.Stop, len(allStops))
	for _, s := range allStops {
		stopByID[s.StopID] = s
	}

	trackedStops := make([]trackedSegmentStop, 0, len(apiStopTimes))
	lastShapeIdx := -1
	for _, st := range apiStopTimes {
		stop, ok := stopByID[st.StopID]
		if !ok {
			continue
		}
		shapeIdx := geo.ClosestShapeIndex(stop.StopLat, stop.StopLon, shapes)
		if shapeIdx < lastShapeIdx {
			shapeIdx = lastShapeIdx
		}
		lastShapeIdx = shapeIdx
		trackedStops = append(trackedStops, trackedSegmentStop{
			StopID:   st.StopID,
			ShapeIdx: shapeIdx,
			Lat:      stop.StopLat,
			Lon:      stop.StopLon,
		})
	}
	if len(trackedStops) < 2 {
		return nil, false
	}

	shortName := ""
	if routes, err := eng.Routes(transit.RouteFilter{RouteID: &trip.RouteID}); err == nil && len(routes) > 0 {
		shortName = routes[0].RouteShortName
	}
	return &tripSegmentTracker{
		TripID:         tripID,
		RouteID:        trip.RouteID,
		RouteShortName: shortName,
		DirectionID:    int(trip.DirectionID),
		Shape:          shapes,
		Cumulative:     buildCumulativeShapeDistance(shapes),
		Stops:          trackedStops,
	}, true
}

func buildCumulativeShapeDistance(shapes []models.Shape) []float64 {
	cumulative := make([]float64, len(shapes))
	for i := 1; i < len(shapes); i++ {
		cumulative[i] = cumulative[i-1] + geo.Haversine(
			shapes[i-1].ShapePtLat, shapes[i-1].ShapePtLon,
			shapes[i].ShapePtLat, shapes[i].ShapePtLon,
		)
	}
	return cumulative
}

func (t *tripSegmentTracker) stopPosForShapeIdx(shapeIdx int) int {
	pos := sort.Search(len(t.Stops), func(i int) bool {
		return t.Stops[i].ShapeIdx > shapeIdx
	}) - 1
	return pos
}

func (t *tripSegmentTracker) nearStopPos(lat, lon float64, shapeIdx int) int {
	routePos := t.stopPosForShapeIdx(shapeIdx)
	bestPos := -1
	bestDist := segmentStopProximityMeters
	for _, pos := range []int{routePos, routePos + 1} {
		if pos < 0 || pos >= len(t.Stops) {
			continue
		}
		stop := t.Stops[pos]
		d := geo.Haversine(lat, lon, stop.Lat, stop.Lon)
		if d <= bestDist {
			bestDist = d
			bestPos = pos
		}
	}
	return bestPos
}

func (t *tripSegmentTracker) distanceBetweenStopPositions(fromPos, toPos int) float64 {
	if fromPos < 0 || toPos < 0 || fromPos >= len(t.Stops) || toPos >= len(t.Stops) {
		return 0
	}
	fromIdx := t.Stops[fromPos].ShapeIdx
	toIdx := t.Stops[toPos].ShapeIdx
	if fromIdx < 0 || toIdx < 0 || fromIdx >= len(t.Cumulative) || toIdx >= len(t.Cumulative) || fromIdx >= toIdx {
		return 0
	}
	return t.Cumulative[toIdx] - t.Cumulative[fromIdx]
}

func (eng *Engine) storeSegmentTravelSample(sample pendingSegmentSample) segmentWriteSummary {
	summary := segmentWriteSummary{}
	if _, err := eng.DB.Exec(`
		INSERT INTO segment_travel_time_samples
		(route_id, direction_id, from_stop_id, to_stop_id, day_type, bucket_start_min, duration_sec, observed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		sample.Key.RouteID,
		sample.Key.DirectionID,
		sample.Key.FromStopID,
		sample.Key.ToStopID,
		sample.Key.DayType,
		sample.Key.BucketStartMin,
		sample.Duration.Seconds(),
		sample.ObservedAt.Unix(),
	); err != nil {
		eng.log("segment travel: sample insert failed: %v", err)
		summary.SampleInsertError++
		return summary
	}

	summary.Stored++
	summary.addProfileResult(eng.recomputeSegmentProfile(sample.Key))
	allDay := sample.Key
	allDay.BucketStartMin = allDaySegmentBucket
	summary.addProfileResult(eng.recomputeSegmentProfile(allDay))
	eng.pruneOldSegmentSamples()
	eng.pruneStaleSegmentProfiles()
	return summary
}

func (eng *Engine) recomputeSegmentProfile(key segmentProfileKey) segmentProfileWriteResult {
	oldCount, oldMedian, oldP75, hadOldProfile := eng.loadSegmentProfileSnapshot(key)
	query := `
		SELECT duration_sec
		FROM segment_travel_time_samples
		WHERE route_id = ?
		  AND direction_id = ?
		  AND from_stop_id = ?
		  AND to_stop_id = ?
		  AND day_type = ?
		  AND `
	inWindow, windowArgs := eng.currentDayWindow(key.DayType, time.Now()).sql("observed_at")
	query += inWindow
	args := append([]any{key.RouteID, key.DirectionID, key.FromStopID, key.ToStopID, key.DayType}, windowArgs...)
	if key.BucketStartMin != allDaySegmentBucket {
		query += ` AND bucket_start_min = ?`
		args = append(args, key.BucketStartMin)
	}
	query += ` ORDER BY duration_sec`

	durations, err := database.QueryRows(eng.DB, query, args, func(rows *sql.Rows) (float64, error) {
		var duration float64
		err := rows.Scan(&duration)
		return duration, err
	})
	if err != nil || len(durations) == 0 {
		if err != nil {
			eng.log("segment travel: profile recompute failed: %v", err)
			return segmentProfileFailed
		}
		return segmentProfileUnchanged
	}

	median, p75, err := upsertSegmentProfile(eng.DB, key, durations, time.Now())
	if err != nil {
		eng.log("segment travel: profile upsert failed: %v", err)
		return segmentProfileFailed
	}

	if !hadOldProfile || oldCount != len(durations) || math.Abs(oldMedian-median) >= 0.5 || math.Abs(oldP75-p75) >= 0.5 {
		if hadOldProfile {
			return segmentProfileUpdated
		}
		return segmentProfileCreated
	}
	return segmentProfileUnchanged
}

type sqlExecer interface {
	Exec(query string, args ...any) (sql.Result, error)
}

func upsertSegmentProfile(db sqlExecer, key segmentProfileKey, sorted []float64, now time.Time) (float64, float64, error) {
	median := nearestRank(sorted, 0.50)
	p75 := nearestRank(sorted, 0.75)
	_, err := db.Exec(`
		INSERT INTO segment_travel_time_profiles
		(route_id, direction_id, from_stop_id, to_stop_id, day_type, bucket_start_min, sample_count, median_sec, p75_sec, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(route_id, direction_id, from_stop_id, to_stop_id, day_type, bucket_start_min)
		DO UPDATE SET
			sample_count = excluded.sample_count,
			median_sec = excluded.median_sec,
			p75_sec = excluded.p75_sec,
			updated_at = excluded.updated_at`,
		key.RouteID, key.DirectionID, key.FromStopID, key.ToStopID, key.DayType, key.BucketStartMin,
		len(sorted), median, p75, now.Unix(),
	)
	return median, p75, err
}

func (eng *Engine) rebuildSegmentProfiles(now time.Time) (int, error) {
	pairs, err := database.QueryRows(eng.DB, `SELECT DISTINCT route_id, direction_id FROM segment_travel_time_samples`, nil, func(rows *sql.Rows) ([2]int, error) {
		var p [2]int
		err := rows.Scan(&p[0], &p[1])
		return p, err
	})
	if err != nil {
		return 0, err
	}
	stored := 0
	for _, p := range pairs {
		n, err := eng.rebuildRouteSegmentProfiles(p[0], p[1], now)
		stored += n
		if err != nil {
			return stored, fmt.Errorf("route %d direction %d: %w", p[0], p[1], err)
		}
	}
	return stored, nil
}

func (eng *Engine) rebuildRouteSegmentProfiles(routeID, directionID int, now time.Time) (int, error) {
	type sample struct {
		key        segmentProfileKey
		duration   float64
		observedAt int64
	}
	samples, err := database.QueryRows(eng.DB, `
		SELECT from_stop_id, to_stop_id, day_type, bucket_start_min, duration_sec, observed_at
		FROM segment_travel_time_samples
		WHERE route_id = ? AND direction_id = ?`,
		[]any{routeID, directionID},
		func(rows *sql.Rows) (sample, error) {
			s := sample{key: segmentProfileKey{RouteID: routeID, DirectionID: directionID}}
			err := rows.Scan(&s.key.FromStopID, &s.key.ToStopID, &s.key.DayType, &s.key.BucketStartMin, &s.duration, &s.observedAt)
			return s, err
		})
	if err != nil {
		return 0, err
	}
	windows := map[string]dayWindow{}
	groups := map[segmentProfileKey][]float64{}
	for _, s := range samples {
		w, ok := windows[s.key.DayType]
		if !ok {
			w = eng.currentDayWindow(s.key.DayType, now)
			windows[s.key.DayType] = w
		}
		if !w.contains(s.observedAt) {
			continue
		}
		groups[s.key] = append(groups[s.key], s.duration)
		allDay := s.key
		allDay.BucketStartMin = allDaySegmentBucket
		groups[allDay] = append(groups[allDay], s.duration)
	}

	tx, err := eng.DB.Begin()
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	for key, durations := range groups {
		sort.Float64s(durations)
		if _, _, err := upsertSegmentProfile(tx, key, durations, now); err != nil {
			return 0, err
		}
	}
	return len(groups), tx.Commit()
}

func (eng *Engine) loadSegmentProfileSnapshot(key segmentProfileKey) (int, float64, float64, bool) {
	var count int
	var median, p75 float64
	err := eng.DB.QueryRow(`
		SELECT sample_count, median_sec, p75_sec
		FROM segment_travel_time_profiles
		WHERE route_id = ?
		  AND direction_id = ?
		  AND from_stop_id = ?
		  AND to_stop_id = ?
		  AND day_type = ?
		  AND bucket_start_min = ?`,
		key.RouteID,
		key.DirectionID,
		key.FromStopID,
		key.ToStopID,
		key.DayType,
		key.BucketStartMin,
	).Scan(&count, &median, &p75)
	if err != nil {
		return 0, 0, 0, false
	}
	return count, median, p75, true
}

func nearestRank(sortedValues []float64, percentile float64) float64 {
	if len(sortedValues) == 0 {
		return 0
	}
	idx := int(math.Ceil(percentile*float64(len(sortedValues)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sortedValues) {
		idx = len(sortedValues) - 1
	}
	return sortedValues[idx]
}

func (eng *Engine) pruneOldSegmentSamples() {
	if !eng.samplePrune.due(time.Hour) {
		return
	}
	n, err := eng.pruneByDayWindow("segment_travel_time_samples", "observed_at", SampleRetention, time.Now())
	if err != nil {
		eng.log("segment travel: sample prune failed: %v", err)
		return
	}
	if n > 0 {
		eng.log("segment travel: pruned %d samples older than %s and outside their day's window", n, SampleRetention)
	}
}

func (eng *Engine) pruneStaleSegmentProfiles() {
	if !eng.profilePrune.due(24 * time.Hour) {
		return
	}
	res, err := eng.DB.Exec(`DELETE FROM segment_travel_time_profiles WHERE updated_at < ?`, time.Now().Add(-segmentProfileRetention).Unix())
	if err != nil {
		eng.log("segment travel: profile prune failed: %v", err)
		return
	}
	if n, err := res.RowsAffected(); err == nil && n > 0 {
		eng.log("segment travel: pruned %d profiles not updated in %s", n, segmentProfileRetention)
	}
}

type segmentProfileEstimate struct {
	DurationSec float64
	Confidence  float64
}

type segmentProfileRow struct {
	pair        stopPair
	dayType     string
	bucket      int
	sampleCount int
	medianSec   float64
}

// All day types: weekday data beats Tranzy's offsets for a segment not yet measured on Sunday.
func (eng *Engine) loadSegmentProfileRows(routeID, directionID int) ([]segmentProfileRow, error) {
	return database.QueryRows(eng.DB, `
		SELECT from_stop_id, to_stop_id, day_type, bucket_start_min, sample_count, median_sec
		FROM segment_travel_time_profiles
		WHERE route_id = ?
		  AND direction_id = ?
		  AND sample_count >= ?`,
		[]any{routeID, directionID, minSegmentProfileSamples},
		func(rows *sql.Rows) (segmentProfileRow, error) {
			var r segmentProfileRow
			err := rows.Scan(&r.pair.FromStopID, &r.pair.ToStopID, &r.dayType, &r.bucket, &r.sampleCount, &r.medianSec)
			return r, err
		})
}

// Same day type first, by bucket distance, then the other day types.
func selectSegmentProfiles(rows []segmentProfileRow, dayType string, bucket int) map[stopPair]segmentProfileEstimate {
	type selectedProfile struct {
		duration    float64
		priority    int
		sampleCount int
	}
	selected := make(map[stopPair]selectedProfile)
	for _, r := range rows {
		priority, ok := segmentProfilePriority(r.bucket, bucket)
		if !ok {
			continue
		}
		if calendar.Siblings(r.dayType, dayType) {
			priority += segmentProfileSiblingDayPenalty
		} else if r.dayType != dayType {
			priority += segmentProfileOtherDayPenalty
		}
		if current, exists := selected[r.pair]; !exists || priority < current.priority {
			selected[r.pair] = selectedProfile{duration: r.medianSec, priority: priority, sampleCount: r.sampleCount}
		}
	}

	out := make(map[stopPair]segmentProfileEstimate, len(selected))
	for pair, profile := range selected {
		out[pair] = segmentProfileEstimate{
			DurationSec: profile.duration,
			Confidence:  segmentProfileConfidence(profile.priority, profile.sampleCount),
		}
	}
	return out
}

const (
	segmentProfileExactPriority     = 0
	segmentProfileNeighborBase      = 100
	segmentProfileAllDayPriority    = 10_000
	segmentProfileSiblingDayPenalty = 50_000
	segmentProfileOtherDayPenalty   = 100_000
)

func segmentProfilePriority(profileBucket, wantedBucket int) (int, bool) {
	if profileBucket == wantedBucket {
		return segmentProfileExactPriority, true
	}
	if profileBucket == allDaySegmentBucket {
		return segmentProfileAllDayPriority, true
	}
	diff := int(math.Abs(float64(profileBucket - wantedBucket)))
	if wrap := 1440 - diff; wrap < diff {
		diff = wrap
	}
	if diff <= segmentProfileNeighborMins {
		return segmentProfileNeighborBase + diff, true
	}
	return 0, false
}

func segmentProfileConfidence(priority, sampleCount int) float64 {
	bucketFactor := 1.0
	switch {
	case priority >= segmentProfileOtherDayPenalty:
		priority -= segmentProfileOtherDayPenalty
		bucketFactor = 0.5
	case priority >= segmentProfileSiblingDayPenalty:
		priority -= segmentProfileSiblingDayPenalty
		bucketFactor = 0.75
	}
	switch {
	case priority >= segmentProfileAllDayPriority:
		bucketFactor *= 0.4
	case priority >= segmentProfileNeighborBase:
		bucketFactor *= 1.0 - float64(priority-segmentProfileNeighborBase)/float64(2*segmentProfileNeighborMins)
	}
	countFactor := float64(sampleCount) / float64(sampleCount+10)
	return bucketFactor * countFactor
}

func segmentBucketStartMin(t time.Time) int {
	minutes := t.Hour()*60 + t.Minute()
	return (minutes / segmentBucketMinutes) * segmentBucketMinutes
}

type tripSegmentTrackersState struct {
	sync.RWMutex
	byTrip map[string]*tripSegmentTracker
}

type vehicleSegmentsState struct {
	sync.Mutex
	byVehicle map[int]vehicleSegmentState
}

type tripScheduleCacheState struct {
	sync.Mutex
	period     int64
	departures map[string][]float64
	offsets    map[string][]float64
}
