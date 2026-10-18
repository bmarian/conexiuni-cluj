package handlers

import (
	"conexiuni-cluj/calendar"
	"conexiuni-cluj/predictions"
	"conexiuni-cluj/router"
	"sync"
	"sync/atomic"
	"time"
)

type accuracyState struct {
	sync.Mutex
	gen     int
	pending map[accuracyKey]*pendingArrival
}

type lastArchiveState struct {
	sync.Mutex
	run *archiveRun
}

type dayWindowsState struct {
	sync.RWMutex
	days map[string]int
}

type dayWindowCacheState struct {
	sync.Mutex
	key     string
	windows map[string]dayWindow
}

type seedFillsState struct {
	sync.RWMutex
	fills map[string]string
}

type feedState struct {
	sync.Mutex
	status feedStatus
}

type liveTripsState struct {
	sync.RWMutex
	period int64
	day    string
	byTrip map[string]*liveTrip
	rules  liveRules
}

type liveTableState struct {
	sync.RWMutex
	table *liveTable
}

type liveViewsState struct {
	sync.Mutex
	day  string
	seen map[string]struct{}
}

type networkCacheState struct {
	sync.Mutex
	current  *builtNetwork
	building bool
}

type plannerState struct {
	sync.Mutex
	stopsKey uint64
	streets  *router.Streets
	places   *router.Places
	period   int64
	networks map[string]*router.Network
}

type profileSnapshotsState struct {
	sync.Mutex
	period    int64
	segments  map[profileSnapshotKey][]segmentProfileRow
	delays    map[profileSnapshotKey][]scheduleDelayRow
	scheduled map[string]map[scheduledOffsetsKey][]float64
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

type turnaroundPendingState struct {
	sync.Mutex
	byVehicle map[int]pendingTurnaround
}

type turnaroundWritesState struct {
	sync.Mutex
	samples  []turnaroundSample
	arrivals []pendingTurnaround
}

type turnaroundCacheState struct {
	sync.Mutex
	period int64
	stats  map[[2]int]terminusProfile
	routes map[int]map[string]predictions.Turnaround
}

type cityState struct {
	hub                     *liveHub
	ingest                  ingestState
	availability            availabilityRegistry
	learner                 learnerState
	accuracy                accuracyState
	lastArchive             lastArchiveState
	dayWindows              dayWindowsState
	activeCalendar          atomic.Pointer[calendar.Calendar]
	calendarGeneration      atomic.Int64
	applyCalendarMu         sync.Mutex
	dayWindowCache          dayWindowCacheState
	seedFills               seedFillsState
	feed                    feedState
	liveTrips               liveTripsState
	liveState               liveTableState
	liveRefreshMu           sync.Mutex
	liveViews               liveViewsState
	networkCache            networkCacheState
	networkBuildMu          sync.Mutex
	planner                 plannerState
	profileSnapshots        profileSnapshotsState
	tripSegmentTrackers     tripSegmentTrackersState
	vehicleSegmentStates    vehicleSegmentsState
	segmentSamplePruneMu    sync.Mutex
	lastSegmentPrune        time.Time
	segmentProfilePruneMu   sync.Mutex
	lastSegmentProfilePrune time.Time
	tripScheduleCache       tripScheduleCacheState
	turnaroundPending       turnaroundPendingState
	turnaroundWrites        turnaroundWritesState
	turnaroundCache         turnaroundCacheState
	turnaroundPruneMu       sync.Mutex
	lastTurnaroundPrune     time.Time
	pushWake                chan struct{}
}

func newCityState() cityState {
	return cityState{
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
