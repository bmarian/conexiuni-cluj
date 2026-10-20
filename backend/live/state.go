package live

import (
	"conexiuni-cluj/calendar"
	"sync"
	"sync/atomic"
	"time"
)

type engineState struct {
	hub                  *liveHub
	learner              learnerState
	accuracy             accuracyState
	lastArchive          lastArchiveState
	dayWindows           dayWindowsState
	activeCalendar       atomic.Pointer[calendar.Calendar]
	calendarGeneration   atomic.Int64
	applyCalendarMu      sync.Mutex
	dayWindowCache       dayWindowCacheState
	seedFills            seedFillsState
	feed                 feedState
	liveTrips            liveTripsState
	liveState            liveTableState
	liveRefreshMu        sync.Mutex
	liveViews            liveViewsState
	profileSnapshots     profileSnapshotsState
	tripSegmentTrackers  tripSegmentTrackersState
	vehicleSegmentStates vehicleSegmentsState
	samplePrune          throttle
	profilePrune         throttle
	tripScheduleCache    tripScheduleCacheState
	turnaroundPending    turnaroundPendingState
	turnaroundWrites     turnaroundWritesState
	turnaroundCache      turnaroundCacheState
	turnaroundPrune      throttle
}

func newEngineState() engineState {
	return engineState{
		accuracy: accuracyState{pending: make(map[accuracyKey]*pendingArrival)},
		dayWindows: dayWindowsState{days: map[string]int{
			calendar.Weekday:         10,
			calendar.WeekdayVacation: 5,
			calendar.Saturday:        2,
			calendar.Sunday:          2,
		}},
		seedFills:            seedFillsState{fills: map[string]string{}},
		tripSegmentTrackers:  tripSegmentTrackersState{byTrip: make(map[string]*tripSegmentTracker)},
		vehicleSegmentStates: vehicleSegmentsState{byVehicle: make(map[int]vehicleSegmentState)},
		turnaroundPending:    turnaroundPendingState{byVehicle: make(map[int]pendingTurnaround)},
	}
}

// throttle lets work run at most once an interval.
type throttle struct {
	mu   sync.Mutex
	last time.Time
}

// due is true at most once every interval, and starts the next interval when it is.
func (t *throttle) due(every time.Duration) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if time.Since(t.last) < every {
		return false
	}
	t.last = time.Now()
	return true
}
