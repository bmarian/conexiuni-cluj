package handlers

import (
	"conexiuni-cluj/database"
	"conexiuni-cluj/models"
	"database/sql"
	"log"
	"math"
	"sort"
	"sync"
	"time"
)

const (
	segmentBucketMinutes       = 60
	allDaySegmentBucket        = -1
	minSegmentProfileSamples   = 3
	segmentProfileNeighborMins = 120
	segmentSampleRetention     = 45 * 24 * time.Hour
	maxObservedVehicleAge      = 10 * time.Minute
	minSegmentDurationSec      = 8
	maxSegmentDurationSec      = 45 * 60
	minObservedSegmentKmh      = 1.5
	maxObservedSegmentKmh      = 70.0
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
}

type tripSegmentTracker struct {
	TripID      string
	RouteID     int
	DirectionID int
	Shape       []models.Shape
	Cumulative  []float64
	Stops       []trackedSegmentStop
}

type vehicleSegmentState struct {
	TripID           string
	ShapeIdx         int
	SegmentStartPos  int
	SegmentStartedAt time.Time
	LastObservedAt   time.Time
	HasSegmentStart  bool
}

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

type segmentLearningRuntimeSnapshot struct {
	ObservedAt         string `json:"observed_at,omitempty"`
	Vehicles           int    `json:"vehicles"`
	Accepted           int    `json:"accepted"`
	Stored             int    `json:"stored"`
	ProfilesCreated    int    `json:"profiles_created"`
	ProfilesUpdated    int    `json:"profiles_updated"`
	ProfilesUnchanged  int    `json:"profiles_unchanged"`
	Rejected           int    `json:"rejected"`
	IgnoredReset       int    `json:"ignored_reset"`
	IgnoredNoProgress  int    `json:"ignored_no_progress"`
	IgnoredNonAdjacent int    `json:"ignored_non_adjacent"`
	IgnoredNoTracker   int    `json:"ignored_no_tracker"`
	Stale              int    `json:"stale"`
	Invalid            int    `json:"invalid"`
	SampleErrors       int    `json:"sample_errors"`
	ProfileErrors      int    `json:"profile_errors"`
}

type segmentProfileWriteResult int

const (
	segmentProfileUnchanged segmentProfileWriteResult = iota
	segmentProfileCreated
	segmentProfileUpdated
	segmentProfileFailed
)

var (
	tripSegmentTrackers = struct {
		sync.RWMutex
		byTrip map[string]*tripSegmentTracker
	}{byTrip: make(map[string]*tripSegmentTracker)}

	vehicleSegmentStates = struct {
		sync.Mutex
		byVehicle map[int]vehicleSegmentState
	}{byVehicle: make(map[int]vehicleSegmentState)}

	segmentSamplePruneMu sync.Mutex
	lastSegmentPrune     time.Time

	segmentLearningRuntime = struct {
		sync.RWMutex
		snapshot segmentLearningRuntimeSnapshot
	}{}
)

func ObserveVehicleSegmentTravelTimes(loc *time.Location, vehicles []models.Vehicle) {
	if loc == nil {
		loc = time.Local
	}
	summary := segmentObservationSummary{Total: len(vehicles)}
	writes := segmentWriteSummary{}
	for _, v := range vehicles {
		sample, status := observeVehicleSegment(loc, v)
		summary.add(status)
		if status == segmentObservedAccepted {
			writes.add(storeSegmentTravelSample(sample))
		}
	}
	if summary.Total > 0 {
		snapshot := segmentLearningRuntimeSnapshot{
			ObservedAt:         time.Now().In(loc).Format(time.RFC3339),
			Vehicles:           summary.Total,
			Accepted:           summary.Accepted,
			Stored:             writes.Stored,
			ProfilesCreated:    writes.ProfilesCreated,
			ProfilesUpdated:    writes.ProfilesUpdated,
			ProfilesUnchanged:  writes.ProfilesUnchanged,
			Rejected:           summary.Rejected,
			IgnoredReset:       summary.Reset,
			IgnoredNoProgress:  summary.NoProgress,
			IgnoredNonAdjacent: summary.NonAdjacent,
			IgnoredNoTracker:   summary.NoTracker,
			Stale:              summary.Stale,
			Invalid:            summary.Invalid,
			SampleErrors:       writes.SampleInsertError,
			ProfileErrors:      writes.ProfileError,
		}
		rememberSegmentLearningSnapshot(snapshot)
		log.Printf("segment travel: snapshot vehicles=%d accepted=%d stored=%d profiles_created=%d profiles_updated=%d profiles_unchanged=%d rejected=%d ignored_reset=%d ignored_no_progress=%d ignored_non_adjacent=%d ignored_no_tracker=%d stale=%d invalid=%d sample_errors=%d profile_errors=%d",
			snapshot.Vehicles,
			snapshot.Accepted,
			snapshot.Stored,
			snapshot.ProfilesCreated,
			snapshot.ProfilesUpdated,
			snapshot.ProfilesUnchanged,
			snapshot.Rejected,
			snapshot.IgnoredReset,
			snapshot.IgnoredNoProgress,
			snapshot.IgnoredNonAdjacent,
			snapshot.IgnoredNoTracker,
			snapshot.Stale,
			snapshot.Invalid,
			snapshot.SampleErrors,
			snapshot.ProfileErrors,
		)
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

func rememberSegmentLearningSnapshot(snapshot segmentLearningRuntimeSnapshot) {
	segmentLearningRuntime.Lock()
	segmentLearningRuntime.snapshot = snapshot
	segmentLearningRuntime.Unlock()
}

func currentSegmentLearningSnapshot() segmentLearningRuntimeSnapshot {
	segmentLearningRuntime.RLock()
	defer segmentLearningRuntime.RUnlock()
	return segmentLearningRuntime.snapshot
}

func observeVehicleSegment(loc *time.Location, v models.Vehicle) (pendingSegmentSample, segmentObservationStatus) {
	if v.ID == 0 || v.Latitude <= 0 || v.Longitude <= 0 || v.RouteID < 0 || v.TripID == "" || v.TripID == "-1" {
		return pendingSegmentSample{}, segmentObservedInvalid
	}
	observedAt, err := time.Parse(time.RFC3339, v.Timestamp)
	if err != nil {
		return pendingSegmentSample{}, segmentObservedInvalid
	}
	observedAt = observedAt.In(loc)
	if time.Since(observedAt) > maxObservedVehicleAge || time.Until(observedAt) > 2*time.Minute {
		return pendingSegmentSample{}, segmentObservedStale
	}

	tracker, ok := getTripSegmentTracker(v.TripID)
	if !ok || len(tracker.Stops) < 2 || len(tracker.Shape) < 2 {
		return pendingSegmentSample{}, segmentObservedNoTracker
	}

	shapeIdx := closestShapeIndexLatLon(v.Latitude, v.Longitude, tracker.Shape)
	stopPos := tracker.stopPosForShapeIdx(shapeIdx)

	vehicleSegmentStates.Lock()
	defer vehicleSegmentStates.Unlock()

	state, exists := vehicleSegmentStates.byVehicle[v.ID]
	if !exists || state.TripID != tracker.TripID || observedAt.After(state.LastObservedAt.Add(25*time.Minute)) ||
		!observedAt.After(state.LastObservedAt) || shapeIdx+3 < state.ShapeIdx || stopPos < state.SegmentStartPos {
		vehicleSegmentStates.byVehicle[v.ID] = newVehicleSegmentState(tracker.TripID, shapeIdx, stopPos, observedAt)
		return pendingSegmentSample{}, segmentObservedReset
	}

	state.ShapeIdx = shapeIdx
	state.LastObservedAt = observedAt
	if stopPos <= state.SegmentStartPos {
		vehicleSegmentStates.byVehicle[v.ID] = state
		return pendingSegmentSample{}, segmentObservedNoProgress
	}

	var sample pendingSegmentSample
	status := segmentObservedReset
	if state.HasSegmentStart {
		status = segmentObservedNonAdjacent
		if stopPos == state.SegmentStartPos+1 {
			from := tracker.Stops[state.SegmentStartPos]
			to := tracker.Stops[stopPos]
			sample = pendingSegmentSample{
				Key: segmentProfileKey{
					RouteID:        tracker.RouteID,
					DirectionID:    tracker.DirectionID,
					FromStopID:     from.StopID,
					ToStopID:       to.StopID,
					DayType:        segmentDayType(observedAt),
					BucketStartMin: segmentBucketStartMin(observedAt),
				},
				Duration:   observedAt.Sub(state.SegmentStartedAt),
				DistanceM:  tracker.distanceBetweenStopPositions(state.SegmentStartPos, stopPos),
				ObservedAt: observedAt,
			}
			if isPlausibleSegmentSample(sample) {
				status = segmentObservedAccepted
			} else {
				status = segmentObservedRejected
			}
		}
	}

	state.SegmentStartPos = stopPos
	state.SegmentStartedAt = observedAt
	state.HasSegmentStart = stopPos >= 0 && stopPos < len(tracker.Stops)-1
	vehicleSegmentStates.byVehicle[v.ID] = state
	return sample, status
}

func newVehicleSegmentState(tripID string, shapeIdx, stopPos int, observedAt time.Time) vehicleSegmentState {
	return vehicleSegmentState{
		TripID:           tripID,
		ShapeIdx:         shapeIdx,
		SegmentStartPos:  stopPos,
		SegmentStartedAt: observedAt,
		LastObservedAt:   observedAt,
		HasSegmentStart:  stopPos >= 0,
	}
}

func isPlausibleSegmentSample(sample pendingSegmentSample) bool {
	durationSec := sample.Duration.Seconds()
	if durationSec < minSegmentDurationSec || durationSec > maxSegmentDurationSec || sample.DistanceM <= 0 {
		return false
	}
	speedKmh := (sample.DistanceM / 1000) / (durationSec / 3600)
	return speedKmh >= minObservedSegmentKmh && speedKmh <= maxObservedSegmentKmh
}

func getTripSegmentTracker(tripID string) (*tripSegmentTracker, bool) {
	tripID = NormalizeTripID(tripID)
	tripSegmentTrackers.RLock()
	if tracker, ok := tripSegmentTrackers.byTrip[tripID]; ok {
		tripSegmentTrackers.RUnlock()
		return tracker, true
	}
	tripSegmentTrackers.RUnlock()

	tracker, ok := loadTripSegmentTracker(tripID)
	if !ok {
		return nil, false
	}

	tripSegmentTrackers.Lock()
	tripSegmentTrackers.byTrip[tripID] = tracker
	tripSegmentTrackers.Unlock()
	return tracker, true
}

func loadTripSegmentTracker(tripID string) (*tripSegmentTracker, bool) {
	trips, err := getTripsFromDB(TripFilter{TripID: &tripID})
	if err != nil || len(trips) == 0 {
		return nil, false
	}
	trip := trips[0]
	shapeID := trip.ShapeID
	shapes, err := getShapesFromDB(ShapeFilter{ShapeID: &shapeID})
	if err != nil || len(shapes) < 2 {
		return nil, false
	}
	sort.Slice(shapes, func(i, j int) bool {
		return shapes[i].ShapePtSequence < shapes[j].ShapePtSequence
	})

	apiStopTimes, err := getAPIStopTimesFromDB(APIStopTimeFilter{TripID: &tripID})
	if err != nil || len(apiStopTimes) < 2 {
		return nil, false
	}
	sort.Slice(apiStopTimes, func(i, j int) bool {
		return apiStopTimes[i].StopSequence < apiStopTimes[j].StopSequence
	})

	allStops, err := getStopsFromDB(StopFilter{})
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
		shapeIdx := closestShapeIndex(stop, shapes)
		if shapeIdx < lastShapeIdx {
			shapeIdx = lastShapeIdx
		}
		lastShapeIdx = shapeIdx
		trackedStops = append(trackedStops, trackedSegmentStop{StopID: st.StopID, ShapeIdx: shapeIdx})
	}
	if len(trackedStops) < 2 {
		return nil, false
	}

	return &tripSegmentTracker{
		TripID:      tripID,
		RouteID:     trip.RouteID,
		DirectionID: int(trip.DirectionID),
		Shape:       shapes,
		Cumulative:  buildCumulativeShapeDistance(shapes),
		Stops:       trackedStops,
	}, true
}

func buildCumulativeShapeDistance(shapes []models.Shape) []float64 {
	cumulative := make([]float64, len(shapes))
	for i := 1; i < len(shapes); i++ {
		cumulative[i] = cumulative[i-1] + haversineMeters(
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

func storeSegmentTravelSample(sample pendingSegmentSample) segmentWriteSummary {
	summary := segmentWriteSummary{}
	if _, err := database.DB.Exec(`
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
		log.Printf("segment travel: sample insert failed: %v", err)
		summary.SampleInsertError++
		return summary
	}

	summary.Stored++
	summary.addProfileResult(recomputeSegmentProfile(sample.Key))
	allDay := sample.Key
	allDay.BucketStartMin = allDaySegmentBucket
	summary.addProfileResult(recomputeSegmentProfile(allDay))
	pruneOldSegmentSamples()
	return summary
}

func recomputeSegmentProfile(key segmentProfileKey) segmentProfileWriteResult {
	oldCount, oldMedian, oldP75, hadOldProfile := loadSegmentProfileSnapshot(key)
	query := `
		SELECT duration_sec
		FROM segment_travel_time_samples
		WHERE route_id = ?
		  AND direction_id = ?
		  AND from_stop_id = ?
		  AND to_stop_id = ?
		  AND day_type = ?
		  AND observed_at >= ?`
	args := []any{key.RouteID, key.DirectionID, key.FromStopID, key.ToStopID, key.DayType, time.Now().Add(-segmentSampleRetention).Unix()}
	if key.BucketStartMin != allDaySegmentBucket {
		query += ` AND bucket_start_min = ?`
		args = append(args, key.BucketStartMin)
	}
	query += ` ORDER BY duration_sec`

	durations, err := queryRows(query, args, func(rows *sql.Rows) (float64, error) {
		var duration float64
		err := rows.Scan(&duration)
		return duration, err
	})
	if err != nil || len(durations) == 0 {
		if err != nil {
			log.Printf("segment travel: profile recompute failed: %v", err)
			return segmentProfileFailed
		}
		return segmentProfileUnchanged
	}

	median := nearestRank(durations, 0.50)
	p75 := nearestRank(durations, 0.75)
	if _, err := database.DB.Exec(`
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
		len(durations), median, p75, time.Now().Unix(),
	); err != nil {
		log.Printf("segment travel: profile upsert failed: %v", err)
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

func loadSegmentProfileSnapshot(key segmentProfileKey) (int, float64, float64, bool) {
	var count int
	var median, p75 float64
	err := database.DB.QueryRow(`
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

func pruneOldSegmentSamples() {
	segmentSamplePruneMu.Lock()
	defer segmentSamplePruneMu.Unlock()
	if time.Since(lastSegmentPrune) < time.Hour {
		return
	}
	lastSegmentPrune = time.Now()
	res, err := database.DB.Exec(`DELETE FROM segment_travel_time_samples WHERE observed_at < ?`, time.Now().Add(-segmentSampleRetention).Unix())
	if err != nil {
		log.Printf("segment travel: sample prune failed: %v", err)
		return
	}
	if n, err := res.RowsAffected(); err == nil && n > 0 {
		log.Printf("segment travel: pruned %d samples older than %s", n, segmentSampleRetention)
	}
}

func loadSegmentProfileDurations(routeID, directionID int, refTime time.Time) (map[stopPair]float64, error) {
	dayType := segmentDayType(refTime)
	bucket := segmentBucketStartMin(refTime)

	rows, err := database.DB.Query(`
		SELECT from_stop_id, to_stop_id, bucket_start_min, median_sec
		FROM segment_travel_time_profiles
		WHERE route_id = ?
		  AND direction_id = ?
		  AND day_type = ?
		  AND sample_count >= ?`,
		routeID, directionID, dayType, minSegmentProfileSamples,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type selectedProfile struct {
		duration float64
		priority int
	}
	selected := make(map[stopPair]selectedProfile)
	for rows.Next() {
		var fromStopID, toStopID, profileBucket int
		var medianSec float64
		if err := rows.Scan(&fromStopID, &toStopID, &profileBucket, &medianSec); err != nil {
			return nil, err
		}
		priority, ok := segmentProfilePriority(profileBucket, bucket)
		if !ok {
			continue
		}
		pair := stopPair{FromStopID: fromStopID, ToStopID: toStopID}
		if current, exists := selected[pair]; !exists || priority < current.priority {
			selected[pair] = selectedProfile{duration: medianSec, priority: priority}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make(map[stopPair]float64, len(selected))
	for pair, profile := range selected {
		out[pair] = profile.duration
	}
	return out, nil
}

func segmentProfilePriority(profileBucket, wantedBucket int) (int, bool) {
	if profileBucket == wantedBucket {
		return 0, true
	}
	if profileBucket == allDaySegmentBucket {
		return 10_000, true
	}
	diff := int(math.Abs(float64(profileBucket - wantedBucket)))
	if wrap := 1440 - diff; wrap < diff {
		diff = wrap
	}
	if diff <= segmentProfileNeighborMins {
		return 100 + diff, true
	}
	return 0, false
}

func segmentDayType(t time.Time) string {
	switch t.Weekday() {
	case time.Saturday:
		return "saturday"
	case time.Sunday:
		return "sunday"
	default:
		return "weekday"
	}
}

func segmentBucketStartMin(t time.Time) int {
	minutes := t.Hour()*60 + t.Minute()
	return (minutes / segmentBucketMinutes) * segmentBucketMinutes
}

func directionIDFromTripID(tripID string) (int, bool) {
	switch {
	case len(tripID) >= 2 && tripID[len(tripID)-2:] == OUTGOING_SUFFIX:
		return 0, true
	case len(tripID) >= 2 && tripID[len(tripID)-2:] == INCOMING_SUFFIX:
		return 1, true
	default:
		return 0, false
	}
}
