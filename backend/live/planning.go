package live

import (
	"conexiuni-cluj/models"
	"conexiuni-cluj/router"
	"slices"
	"strconv"
	"time"
)

// LiveAt is the live table for a plan leaving at when, when one is fresh and within its horizon.
func (eng *Engine) LiveAt(when time.Time) (router.LiveSource, func(firsts []int64) int64) {
	if eng.hub == nil || !when.Before(time.Now().Add(liveScheduleHorizon)) {
		return nil, nil
	}
	table, err := eng.hub.current()
	if err != nil {
		eng.log("plan: live: %v", err)
		return nil, nil
	}
	return plannerLive(table), func(firsts []int64) int64 { return eng.refreshIn(table, firsts) }
}

// Never sooner than the next vehicle poll.
func (eng *Engine) refreshIn(table *liveTable, firsts []int64) int64 {
	if table == nil || len(firsts) == 0 {
		return 0
	}
	wait := slices.Min(firsts) - time.Now().Unix()
	if interval := livePollInterval(eng); interval > 0 {
		wait = max(wait, int64(interval.Seconds()))
	}
	return max(1, wait)
}

// plannerLive reads the stop pages' own rows, so the planner shows the buses they show.
func plannerLive(table *liveTable) router.LiveSource {
	return func(key string, stop int) []router.Arrival {
		for _, row := range table.rowsByTrip[key] {
			if row.StopID != stop {
				continue
			}
			out := make([]router.Arrival, len(row.Entries))
			for i, e := range row.Entries {
				out[i] = router.Arrival{At: e.At, Live: e.Live, VehicleID: e.VehicleID}
			}
			return out
		}
		return nil
	}
}

// Ride is the stretch of a pattern's shape between two of its stops.
func (eng *Engine) Ride(tripID string, from, to int) ([]models.Shape, bool) {
	tracker, ok := eng.getTripSegmentTracker(tripID)
	if !ok {
		return nil, false
	}
	for i, s := range tracker.Stops {
		if s.StopID != from {
			continue
		}
		for _, e := range tracker.Stops[i+1:] {
			if e.StopID == to && e.ShapeIdx >= s.ShapeIdx {
				return tracker.Shape[s.ShapeIdx : e.ShapeIdx+1], true
			}
		}
	}
	return nil, false
}

func (eng *Engine) ProfileKey(t time.Time) string {
	return eng.profileDay(t) + "|" + strconv.Itoa(segmentBucketStartMin(t))
}

func (eng *Engine) Period() int64 { return ProfileSnapshotPeriod() }

func (eng *Engine) TransferSlack() time.Duration {
	return time.Duration(eng.GraceSec() * float64(time.Second))
}

func (eng *Engine) GraceSec() float64 { return eng.learnLiveRules().graceSec }

func (eng *Engine) Horizon() time.Duration { return liveScheduleHorizon }
