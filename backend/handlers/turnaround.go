package handlers

import (
	"conexiuni-cluj/database"
	"conexiuni-cluj/models"
	"conexiuni-cluj/predictions"
	"database/sql"
	"log"
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

var (
	turnaroundPending = struct {
		sync.Mutex
		byVehicle map[int]pendingTurnaround
	}{byVehicle: make(map[int]pendingTurnaround)}

	turnaroundWrites = struct {
		sync.Mutex
		samples  []turnaroundSample
		arrivals []pendingTurnaround
	}{}

	turnaroundCache = struct {
		sync.Mutex
		period int64
		stats  map[[2]int]terminusProfile
		routes map[int]map[string]predictions.Turnaround
	}{}

	turnaroundPruneMu   sync.Mutex
	lastTurnaroundPrune time.Time
)

func setPendingTurnaround(vehicleID int, p pendingTurnaround) {
	turnaroundPending.Lock()
	turnaroundPending.byVehicle[vehicleID] = p
	turnaroundPending.Unlock()
	turnaroundWrites.Lock()
	turnaroundWrites.arrivals = append(turnaroundWrites.arrivals, p)
	turnaroundWrites.Unlock()
}

func clearPendingTurnaround(vehicleID int) {
	turnaroundPending.Lock()
	delete(turnaroundPending.byVehicle, vehicleID)
	turnaroundPending.Unlock()
}

// The bus left its first stop: the stand between the flip and that departure is one sample.
func recordTurnaroundDeparture(vehicleID int, tripID string, departedAt time.Time) {
	turnaroundPending.Lock()
	p, ok := turnaroundPending.byVehicle[vehicleID]
	if ok {
		delete(turnaroundPending.byVehicle, vehicleID)
	}
	turnaroundPending.Unlock()
	if !ok || p.TripID != tripID {
		return
	}
	stand := departedAt.Sub(p.ArrivedAt).Seconds()
	if stand < 0 || stand > maxTurnaroundSec {
		return
	}
	turnaroundWrites.Lock()
	turnaroundWrites.samples = append(turnaroundWrites.samples, turnaroundSample{
		RouteID: p.RouteID, DirectionID: p.DirectionID, ArrivedAt: p.ArrivedAt, DepartedAt: departedAt,
		PairedDepartureMin: p.PairedDepartureMin,
	})
	turnaroundWrites.Unlock()
}

func flushTurnaroundSamples() int {
	turnaroundWrites.Lock()
	samples := turnaroundWrites.samples
	arrivals := turnaroundWrites.arrivals
	turnaroundWrites.samples = nil
	turnaroundWrites.arrivals = nil
	turnaroundWrites.Unlock()
	for _, a := range arrivals {
		if _, err := database.DB.Exec(`INSERT OR IGNORE INTO turnaround_arrivals (route_id, direction_id, arrived_at) VALUES (?, ?, ?)`,
			a.RouteID, a.DirectionID, a.ArrivedAt.Unix()); err != nil {
			log.Printf("turnaround: arrival insert failed: %v", err)
		}
	}
	stored := 0
	for _, s := range samples {
		if _, err := database.DB.Exec(`
			INSERT INTO turnaround_samples
			(route_id, direction_id, day_type, duration_sec, arrived_at, departed_at, paired_departure_min)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			s.RouteID, s.DirectionID, segmentDayType(s.DepartedAt), s.DepartedAt.Sub(s.ArrivedAt).Seconds(),
			s.ArrivedAt.Unix(), s.DepartedAt.Unix(), s.PairedDepartureMin); err != nil {
			log.Printf("turnaround: sample insert failed: %v", err)
			continue
		}
		stored++
	}
	if stored > 0 {
		pruneOldTurnaroundSamples()
	}
	return stored
}

func pruneOldTurnaroundSamples() {
	turnaroundPruneMu.Lock()
	defer turnaroundPruneMu.Unlock()
	if time.Since(lastTurnaroundPrune) < time.Hour {
		return
	}
	lastTurnaroundPrune = time.Now()
	cutoff := time.Now().Add(-turnaroundRetention).Unix()
	if _, err := database.DB.Exec(`DELETE FROM turnaround_samples WHERE departed_at < ?`, cutoff); err != nil {
		log.Printf("turnaround: prune failed: %v", err)
	}
	if _, err := database.DB.Exec(`DELETE FROM turnaround_arrivals WHERE arrived_at < ?`, cutoff); err != nil {
		log.Printf("turnaround: arrivals prune failed: %v", err)
	}
}

func turnaroundCacheFor(period int64) {
	if turnaroundCache.period == period && turnaroundCache.stats != nil {
		return
	}
	turnaroundCache.period = period
	turnaroundCache.stats = make(map[[2]int]terminusProfile)
	turnaroundCache.routes = make(map[int]map[string]predictions.Turnaround)
}

func resetTurnaroundCache() {
	turnaroundCache.Lock()
	turnaroundCache.stats = nil
	turnaroundCache.routes = nil
	turnaroundCache.Unlock()
}

func loadTerminusProfile(routeID, directionID int) terminusProfile {
	key := [2]int{routeID, directionID}
	turnaroundCache.Lock()
	turnaroundCacheFor(ProfileSnapshotPeriod())
	profile, ok := turnaroundCache.stats[key]
	turnaroundCache.Unlock()
	if ok {
		return profile
	}
	since := profileClock().Add(-turnaroundWindow).Unix()
	stands, err := queryRows(`
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
		log.Printf("turnaround: route=%d direction=%d: %v", routeID, directionID, err)
		return terminusProfile{}
	}
	profile = terminusProfile{Departures: len(stands)}
	_ = database.DB.QueryRow(`SELECT COUNT(*) FROM turnaround_arrivals WHERE route_id = ? AND direction_id = ? AND arrived_at >= ?`,
		routeID, directionID, since).Scan(&profile.Arrivals)
	if len(stands) >= turnaroundMinSamples {
		profile.Turnaround = learnTurnaround(stands)
	}
	turnaroundCache.Lock()
	turnaroundCacheFor(ProfileSnapshotPeriod())
	turnaroundCache.stats[key] = profile
	turnaroundCache.Unlock()
	return profile
}

// Quickest quarter is the stand, longest tenth the max wait; few samples lean on the default.
func learnTurnaround(stands []float64) predictions.Turnaround {
	n := float64(len(stands))
	weight := n / (n + turnaroundMinSamples)
	stand := weight*nearestRank(stands, 0.25) + (1-weight)*predictions.DefaultStandSec
	return predictions.Turnaround{Through: true, StandSec: stand, MaxWaitMin: nearestRank(stands, 0.9) / 60}
}

func sortedTrip(stopTimes []models.StopTime, tripID string) []models.StopTime {
	var trip []models.StopTime
	for _, st := range stopTimes {
		if st.TripID == tripID {
			trip = append(trip, st)
		}
	}
	sort.Slice(trip, func(i, j int) bool { return trip[i].StopSequence < trip[j].StopSequence })
	return trip
}

// Planned gap between arriving on the feeder direction and the next departure.
func scheduledLayoverMinutes(day models.DaySchedule, fromDurationMin float64, toDirection int) (float64, bool) {
	var arrivals, departures []float64
	for _, e := range day.Entries {
		in, okIn := parseDepartureMinutes(e.DepartureIn)
		out, okOut := parseDepartureMinutes(e.DepartureOut)
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

func turnaroundFor(routeID, directionID int, from, to []models.StopTime, timetable *models.Timetable) predictions.Turnaround {
	if len(from) < 2 || len(to) < 2 {
		return predictions.Turnaround{}
	}
	last, first := from[len(from)-1], to[0]
	if last.StopLat == 0 || first.StopLat == 0 || haversineMeters(last.StopLat, last.StopLon, first.StopLat, first.StopLon) > throughTerminusMeters {
		return predictions.Turnaround{}
	}
	profile := loadTerminusProfile(routeID, directionID)
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
func routeTurnarounds(routeID int, stopTimes []models.StopTime) map[string]predictions.Turnaround {
	turnaroundCache.Lock()
	turnaroundCacheFor(ProfileSnapshotPeriod())
	cached, ok := turnaroundCache.routes[routeID]
	turnaroundCache.Unlock()
	if ok {
		return cached
	}
	// Tranzy's own offsets, not the learned ones: the timetable pairs runs by them.
	rid := strconv.Itoa(routeID)
	rows, err := getStopTimesForTrips([]string{rid + OUTGOING_SUFFIX, rid + INCOMING_SUFFIX})
	if err != nil {
		rows = stopTimes
	}
	out := sortedTrip(rows, rid+OUTGOING_SUFFIX)
	in := sortedTrip(rows, rid+INCOMING_SUFFIX)
	var timetable *models.Timetable
	if len(out) > 0 {
		timetable, _ = getTimetableFromDB(out[0].RouteShortName)
	} else if len(in) > 0 {
		timetable, _ = getTimetableFromDB(in[0].RouteShortName)
	}
	result := map[string]predictions.Turnaround{}
	if t := turnaroundFor(routeID, 0, in, out, timetable); t.Through {
		result["0"] = t
	}
	if t := turnaroundFor(routeID, 1, out, in, timetable); t.Through {
		result["1"] = t
	}
	turnaroundCache.Lock()
	turnaroundCacheFor(ProfileSnapshotPeriod())
	turnaroundCache.routes[routeID] = result
	turnaroundCache.Unlock()
	return result
}
