package live

import (
	"conexiuni-cluj/calendar"
	"conexiuni-cluj/models"
	"conexiuni-cluj/transit"
	"sort"
	"time"
)

func (eng *Engine) GetStopTimesAt(filter transit.StopTimeFilter, refTime time.Time) ([]models.StopTime, error) {
	stopTimes, routeID, err := eng.BaseStopTimes(filter)
	if err != nil || routeID == 0 {
		return stopTimes, err
	}
	return eng.TimedStopTimes(stopTimes, routeID, refTime), nil
}

// DayOfType is the first day from from on that runs dayType's timetable.
func (eng *Engine) DayOfType(from time.Time, dayType string) time.Time {
	for i := 0; i < 7; i++ {
		if d := from.AddDate(0, 0, i); eng.ScheduleDay(d) == dayType {
			return d
		}
	}
	return from
}

func (eng *Engine) TimedStopTimes(stopTimes []models.StopTime, routeID int, refTime time.Time) []models.StopTime {
	if len(stopTimes) == 0 {
		return stopTimes
	}

	out := make([]models.StopTime, len(stopTimes))
	copy(out, stopTimes)
	dayType, bucket := eng.profileDay(refTime), segmentBucketStartMin(refTime)

	type directionProfile struct {
		segments map[stopPair]segmentProfileEstimate
		delay    float64
		hasDelay bool
	}
	byDirection := make(map[int]directionProfile)
	groupByTrip := make(map[string][]int)
	for i, st := range out {
		groupByTrip[st.TripID] = append(groupByTrip[st.TripID], i)
	}

	for tripID, indexes := range groupByTrip {
		directionID, ok := models.PatternDirection(tripID)
		if !ok {
			continue
		}
		profile, exists := byDirection[directionID]
		if !exists {
			profile.segments = eng.snapshotSegmentProfiles(routeID, directionID, dayType, bucket)
			profile.delay, profile.hasDelay = eng.snapshotScheduleDelay(routeID, directionID, dayType, bucket)
			byDirection[directionID] = profile
		}

		sort.Slice(indexes, func(i, j int) bool {
			return out[indexes[i]].StopSequence < out[indexes[j]].StopSequence
		})
		eng.timeTrip(out, indexes, tripID, dayType, bucket, profile.segments)
		// Every timetable consumer sums from the first stop; live ETAs never read it.
		if profile.hasDelay && len(indexes) > 0 {
			out[indexes[0]].OffsetArrivalTime = profile.delay
		}
	}
	return out
}

func (eng *Engine) timeTrip(stopTimes []models.StopTime, ordered []int, tripID, profileDay string, bucket int, profiles map[stopPair]segmentProfileEstimate) {
	applyScheduledOffsets(stopTimes, ordered, eng.snapshotScheduledOffsets(tripID, calendar.ScheduleOf(profileDay), bucket))
	applySegmentProfilesToTrip(stopTimes, ordered, profiles)
	for pos := 1; pos <= eng.offered.TimetableStart(tripID) && pos < len(ordered); pos++ {
		stopTimes[ordered[pos]].OffsetArrivalTime = 0
	}
}

func applyScheduledOffsets(stopTimes []models.StopTime, ordered []int, offsets []float64) {
	// A different length is a trip that changed since.
	if len(offsets) != len(ordered) {
		return
	}
	for pos := 1; pos < len(ordered); pos++ {
		if offsets[pos] >= 0 {
			stopTimes[ordered[pos]].OffsetArrivalTime = offsets[pos]
		}
	}
}

// ordered indexes one trip's rows in stop_sequence order.
func applySegmentProfilesToTrip(stopTimes []models.StopTime, ordered []int, profiles map[stopPair]segmentProfileEstimate) {
	for pos := 1; pos < len(ordered); pos++ {
		prev := stopTimes[ordered[pos-1]]
		curr := &stopTimes[ordered[pos]]
		if estimate, ok := profiles[stopPair{FromStopID: prev.StopID, ToStopID: curr.StopID}]; ok && estimate.DurationSec > 0 {
			curr.OffsetArrivalTime = estimate.DurationSec
			curr.OffsetConfidence = estimate.Confidence
		}
	}
}
