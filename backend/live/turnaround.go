package live

import (
	"conexiuni-cluj/database"
	"conexiuni-cluj/geo"
	"conexiuni-cluj/models"
	"conexiuni-cluj/predictions"
	"conexiuni-cluj/transit"
	"database/sql"
	"sort"
	"strconv"
	"sync"
	"time"
)

// Tranzy flips the trip id as the bus arrives, so the flip is the arrival.
const (
	turnaroundWindow        = 14 * 24 * time.Hour
	turnaroundRetention     = 45 * 24 * time.Hour
	turnaroundMinSamples    = 10
	throughTerminusMeters   = 150.0
	turnaroundArrivalRadius = 250.0
	maxTurnaroundSec        = 30 * 60.0
	// Timetables planning a longer stand than this are depot turns, not through running.
	scheduledThroughLayoverMin = 10.0
)

type pendingTurnaround struct {
	RouteID            int
	DirectionID        int
	TripID             string
	ArrivedAt          time.Time
	PairedDepartureMin float64
}

type turnaroundSample struct {
	RouteID            int
	DirectionID        int
	ArrivedAt          time.Time
	DepartedAt         time.Time
	PairedDepartureMin float64
}

// What the stands seen at one terminus say about the next bus. See predictions.Turnaround.
type terminusProfile struct {
	Arrivals   int
	Departures int
	Turnaround predictions.Turnaround
}

func (eng *Engine) setPendingTurnaround(vehicleID int, p pendingTurnaround) {
	eng.turnaroundPending.Lock()
	eng.turnaroundPending.byVehicle[vehicleID] = p
	eng.turnaroundPending.Unlock()
	eng.turnaroundWrites.Lock()
	eng.turnaroundWrites.arrivals = append(eng.turnaroundWrites.arrivals, p)
	eng.turnaroundWrites.Unlock()
}

func (eng *Engine) clearPendingTurnaround(vehicleID int) {
	eng.turnaroundPending.Lock()
	delete(eng.turnaroundPending.byVehicle, vehicleID)
	eng.turnaroundPending.Unlock()
}

// The bus left its first stop: the stand between the flip and that departure is one sample.
func (eng *Engine) recordTurnaroundDeparture(vehicleID int, tripID string, departedAt time.Time) {
	eng.turnaroundPending.Lock()
	p, ok := eng.turnaroundPending.byVehicle[vehicleID]
	if ok {
		delete(eng.turnaroundPending.byVehicle, vehicleID)
	}
	eng.turnaroundPending.Unlock()
	if !ok || p.TripID != tripID {
		return
	}
	stand := departedAt.Sub(p.ArrivedAt).Seconds()
	if stand < 0 || stand > maxTurnaroundSec {
		return
	}
	eng.turnaroundWrites.Lock()
	eng.turnaroundWrites.samples = append(eng.turnaroundWrites.samples, turnaroundSample{
		RouteID: p.RouteID, DirectionID: p.DirectionID, ArrivedAt: p.ArrivedAt, DepartedAt: departedAt,
		PairedDepartureMin: p.PairedDepartureMin,
	})
	eng.turnaroundWrites.Unlock()
}

func (eng *Engine) flushTurnaroundSamples() int {
	eng.turnaroundWrites.Lock()
	samples := eng.turnaroundWrites.samples
	arrivals := eng.turnaroundWrites.arrivals
	eng.turnaroundWrites.samples = nil
	eng.turnaroundWrites.arrivals = nil
	eng.turnaroundWrites.Unlock()
	for _, a := range arrivals {
		if _, err := eng.DB.Exec(`INSERT OR IGNORE INTO turnaround_arrivals (route_id, direction_id, arrived_at) VALUES (?, ?, ?)`,
			a.RouteID, a.DirectionID, a.ArrivedAt.Unix()); err != nil {
			eng.log("turnaround: arrival insert failed: %v", err)
		}
	}
	stored := 0
	for _, s := range samples {
		if _, err := eng.DB.Exec(`
			INSERT INTO turnaround_samples
			(route_id, direction_id, day_type, duration_sec, arrived_at, departed_at, paired_departure_min)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			s.RouteID, s.DirectionID, eng.profileDay(s.DepartedAt), s.DepartedAt.Sub(s.ArrivedAt).Seconds(),
			s.ArrivedAt.Unix(), s.DepartedAt.Unix(), s.PairedDepartureMin); err != nil {
			eng.log("turnaround: sample insert failed: %v", err)
			continue
		}
		stored++
	}
	if stored > 0 {
		eng.pruneOldTurnaroundSamples()
	}
	return stored
}

func (eng *Engine) pruneOldTurnaroundSamples() {
	if !eng.turnaroundPrune.due(time.Hour) {
		return
	}
	cutoff := time.Now().Add(-turnaroundRetention).Unix()
	if _, err := eng.pruneByDayWindow("turnaround_samples", "departed_at", turnaroundRetention, time.Now()); err != nil {
		eng.log("turnaround: prune failed: %v", err)
	}
	if _, err := eng.DB.Exec(`DELETE FROM turnaround_arrivals WHERE arrived_at < ?`, cutoff); err != nil {
		eng.log("turnaround: arrivals prune failed: %v", err)
	}
}

func (eng *Engine) turnaroundCacheFor(period int64) {
	if eng.turnaroundCache.period == period && eng.turnaroundCache.stats != nil {
		return
	}
	eng.turnaroundCache.period = period
	eng.turnaroundCache.stats = make(map[[2]int]terminusProfile)
	eng.turnaroundCache.routes = make(map[int]map[string]predictions.Turnaround)
}

func (eng *Engine) loadTerminusProfile(routeID, directionID int) terminusProfile {
	key := [2]int{routeID, directionID}
	eng.turnaroundCache.Lock()
	eng.turnaroundCacheFor(ProfileSnapshotPeriod())
	profile, ok := eng.turnaroundCache.stats[key]
	eng.turnaroundCache.Unlock()
	if ok {
		return profile
	}
	since := profileClock().Add(-turnaroundWindow).Unix()
	stands, err := database.QueryRows(eng.DB, `
		SELECT duration_sec FROM turnaround_samples
		WHERE route_id = ? AND direction_id = ? AND departed_at >= ?
		ORDER BY duration_sec`,
		[]any{routeID, directionID, since},
		func(rows *sql.Rows) (float64, error) {
			var d float64
			err := rows.Scan(&d)
			return d, err
		})
	if err != nil {
		eng.log("turnaround: route=%d direction=%d: %v", routeID, directionID, err)
		return terminusProfile{}
	}
	profile = terminusProfile{Departures: len(stands)}
	_ = eng.DB.QueryRow(`SELECT COUNT(*) FROM turnaround_arrivals WHERE route_id = ? AND direction_id = ? AND arrived_at >= ?`,
		routeID, directionID, since).Scan(&profile.Arrivals)
	if len(stands) >= turnaroundMinSamples {
		profile.Turnaround = learnTurnaround(stands)
	}
	eng.turnaroundCache.Lock()
	eng.turnaroundCacheFor(ProfileSnapshotPeriod())
	eng.turnaroundCache.stats[key] = profile
	eng.turnaroundCache.Unlock()
	return profile
}

// Quickest quarter is the stand, longest tenth the max wait; few samples lean on the default.
func learnTurnaround(stands []float64) predictions.Turnaround {
	n := float64(len(stands))
	weight := n / (n + turnaroundMinSamples)
	stand := weight*nearestRank(stands, 0.25) + (1-weight)*predictions.DefaultStandSec
	return predictions.Turnaround{Through: true, StandSec: stand, MaxWaitMin: nearestRank(stands, 0.9) / 60}
}

// Planned gap between arriving on the feeder direction and the next departure.
func scheduledLayoverMinutes(day models.DaySchedule, fromDurationMin float64, toDirection int) (float64, bool) {
	var arrivals, departures []float64
	for _, e := range day.Entries {
		in, okIn := transit.ParseDeparture(e.DepartureIn)
		out, okOut := transit.ParseDeparture(e.DepartureOut)
		if !okIn || !okOut {
			continue
		}
		if toDirection == 1 {
			arrivals = append(arrivals, float64(in)+fromDurationMin)
			departures = append(departures, float64(out))
		} else {
			arrivals = append(arrivals, float64(out)+fromDurationMin)
			departures = append(departures, float64(in))
		}
	}
	sort.Float64s(departures)
	var gaps []float64
	for _, arrival := range arrivals {
		// A departure a minute before the computed arrival is this run's own slot, rounded.
		i := sort.SearchFloat64s(departures, arrival-1)
		if i < len(departures) {
			gaps = append(gaps, departures[i]-arrival)
		}
	}
	if len(gaps) == 0 {
		return 0, false
	}
	return median(gaps), true
}

func tripDurationMinutes(trip []models.StopTime) float64 {
	total := 0.0
	for i, st := range trip {
		if i > 0 {
			total += st.OffsetArrivalTime
		}
	}
	return total / 60
}

func (eng *Engine) turnaroundFor(routeID, directionID int, from, to []models.StopTime, timetable *models.Timetable) predictions.Turnaround {
	if len(from) < 2 || len(to) < 2 {
		return predictions.Turnaround{}
	}
	last, first := from[len(from)-1], to[0]
	if last.StopLat == 0 || first.StopLat == 0 || geo.Haversine(last.StopLat, last.StopLon, first.StopLat, first.StopLon) > throughTerminusMeters {
		return predictions.Turnaround{}
	}
	profile := eng.loadTerminusProfile(routeID, directionID)
	if profile.Arrivals >= turnaroundMinSamples && profile.Departures*2 < profile.Arrivals {
		return predictions.Turnaround{}
	}
	if profile.Departures >= turnaroundMinSamples {
		return profile.Turnaround
	}
	// Before stands are learned: a short planned stand at its own arrival stop. A loop is usually a depot.
	if timetable == nil || last.StopID == first.StopID {
		return predictions.Turnaround{}
	}
	duration := tripDurationMinutes(from)
	for _, day := range []models.DaySchedule{timetable.Weekdays, timetable.Saturday, timetable.Sunday} {
		if layover, ok := scheduledLayoverMinutes(day, duration, directionID); ok {
			return predictions.Turnaround{Through: layover <= scheduledThroughLayoverMin, StandSec: predictions.DefaultStandSec}
		}
	}
	return predictions.Turnaround{}
}

// Both termini of a route, keyed by the direction being departed.
func (eng *Engine) routeTurnarounds(routeID int, stopTimes []models.StopTime) map[string]predictions.Turnaround {
	eng.turnaroundCache.Lock()
	eng.turnaroundCacheFor(ProfileSnapshotPeriod())
	cached, ok := eng.turnaroundCache.routes[routeID]
	eng.turnaroundCache.Unlock()
	if ok {
		return cached
	}
	// Tranzy's own offsets, not the learned ones: the timetable pairs runs by them.
	rid := strconv.Itoa(routeID)
	rows, err := eng.StopTimesForTrips([]string{rid + models.OutgoingSuffix, rid + models.IncomingSuffix})
	if err != nil {
		rows = stopTimes
	}
	out := transit.TripStopTimes(rows, rid+models.OutgoingSuffix)
	in := transit.TripStopTimes(rows, rid+models.IncomingSuffix)
	var timetable *models.Timetable
	if len(out) > 0 {
		timetable, _ = eng.Timetable(out[0].RouteShortName)
	} else if len(in) > 0 {
		timetable, _ = eng.Timetable(in[0].RouteShortName)
	}
	result := map[string]predictions.Turnaround{}
	if t := eng.turnaroundFor(routeID, 0, in, out, timetable); t.Through {
		result["0"] = t
	}
	if t := eng.turnaroundFor(routeID, 1, out, in, timetable); t.Through {
		result["1"] = t
	}
	eng.turnaroundCache.Lock()
	eng.turnaroundCacheFor(ProfileSnapshotPeriod())
	eng.turnaroundCache.routes[routeID] = result
	eng.turnaroundCache.Unlock()
	return result
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
