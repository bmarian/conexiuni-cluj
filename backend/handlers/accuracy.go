package handlers

import (
	"conexiuni-cluj/database"
	"conexiuni-cluj/predictions"
	"math"
	"strconv"
	"sync"
	"time"
)

// Shown live ETAs, scored against when the bus actually reached the stop.

const (
	accuracyMaxShownMin = 30
	accuracyBinSec      = 30
	// Misses beyond this land in the last bin.
	accuracyMaxErrSec = 15 * 60
)

var accuracyHorizons = []struct {
	Label    string
	From, To int
}{
	{"1-2", 1, 2},
	{"3-5", 3, 5},
	{"6-10", 6, 10},
	{"11-20", 11, 20},
	{"21-30", 21, 30},
}

var etaKindNames = map[predictions.EtaKind]string{
	predictions.EtaOnTrip:   "on_trip",
	predictions.EtaStanding: "standing",
	predictions.EtaFeeder:   "feeder",
}

type accuracyKey struct {
	vehicle int
	trip    string
	stop    int
}

type shownTime struct {
	minutes int
	at      int64
	kind    predictions.EtaKind
}

type pendingArrival struct {
	shown []shownTime
	gen   int
}

var accuracy = struct {
	sync.Mutex
	gen     int
	pending map[accuracyKey]*pendingArrival
}{pending: make(map[accuracyKey]*pendingArrival)}

// noteShownTimes keeps the first prediction behind each whole minute a screen could show.
func noteShownTimes(table *liveTable) {
	now := table.AsOf.Unix()
	accuracy.Lock()
	defer accuracy.Unlock()
	accuracy.gen++
	gen := accuracy.gen
	for _, rows := range table.rowsByTrip {
		for _, row := range rows {
			for _, e := range row.Entries {
				if !e.Live || e.VehicleID == 0 {
					continue
				}
				key := accuracyKey{vehicle: e.VehicleID, trip: row.TripID, stop: row.StopID}
				minutes := int(math.Round(float64(e.At-now) / 60))
				p := accuracy.pending[key]
				if p == nil {
					if minutes < 1 || minutes > accuracyMaxShownMin {
						continue
					}
					p = &pendingArrival{}
					accuracy.pending[key] = p
				}
				p.gen = gen
				if minutes >= 1 && minutes <= accuracyMaxShownMin && !p.has(minutes) {
					p.shown = append(p.shown, shownTime{minutes: minutes, at: e.At, kind: e.kind})
				}
			}
		}
	}
	// Two polls without the bus at that stop: it went past unseen, or the trip ended.
	for key, p := range accuracy.pending {
		if gen-p.gen >= 2 {
			delete(accuracy.pending, key)
		}
	}
}

func (p *pendingArrival) has(minutes int) bool {
	for _, s := range p.shown {
		if s.minutes == minutes {
			return true
		}
	}
	return false
}

func noteArrival(vehicleID int, tripID string, stopID int, at time.Time) {
	key := accuracyKey{vehicle: vehicleID, trip: tripID, stop: stopID}
	accuracy.Lock()
	p := accuracy.pending[key]
	delete(accuracy.pending, key)
	accuracy.Unlock()
	if p == nil {
		return
	}
	for _, s := range p.shown {
		database.RecordEvent("eta_error", accuracyStatsKey(s, at.Unix()-s.at))
	}
}

// accuracyStatsKey is "kind|horizon|bin"; bin b holds misses in [30b, 30b+30) s, positive when the bus came later.
func accuracyStatsKey(s shownTime, errSec int64) string {
	horizon := ""
	for _, h := range accuracyHorizons {
		if s.minutes >= h.From && s.minutes <= h.To {
			horizon = h.Label
		}
	}
	return etaKindNames[s.kind] + "|" + horizon + "|" + strconv.Itoa(errorBin(errSec))
}

func errorBin(errSec int64) int {
	errSec = max(-accuracyMaxErrSec, min(accuracyMaxErrSec, errSec))
	return int(math.Floor(float64(errSec) / accuracyBinSec))
}

// Scored twice: against our timetable and against CTP's departure alone.
func noteTimetableArrival(tracker *tripSegmentTracker, pos int, departureMin float64, at time.Time, loc *time.Location) {
	arrivedMin, bucket := localMinutes(float64(at.Unix()), loc)
	dayType := profileDay(at)
	learned, seed := tripCumulativeOffsets(tracker, dayType, bucket), tripSeedOffsets(tracker)
	if pos >= len(learned) || pos >= len(seed) {
		return
	}
	delay, _ := snapshotScheduleDelay(tracker.RouteID, tracker.DirectionID, dayType, bucket)
	miss := func(offsetSec float64) string {
		return strconv.Itoa(errorBin(int64(math.Round(wrapDayMinutes(arrivedMin-departureMin-offsetSec/60) * 60))))
	}
	database.RecordEvent("timetable_error", "ours|"+miss(delay+learned[pos]))
	database.RecordEvent("timetable_error", "ctp|"+miss(seed[pos]))
}
