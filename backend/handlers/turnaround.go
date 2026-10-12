package handlers

import (
	"conexiuni-cluj/database"
	"conexiuni-cluj/models"
	"database/sql"
	"log"
	"math"
	"sort"
	"strconv"
	"sync"
	"time"
)

// A bus that ends one direction and continues as the other after a short stand.
// Tranzy flips the trip id as the bus rolls in, so the flip is the arrival.
const (
	turnaroundWindow           = 14 * 24 * time.Hour
	turnaroundRetention        = 45 * 24 * time.Hour
	turnaroundMinSamples       = 10
	turnaroundThroughMaxSec    = 5 * 60.0
	throughTerminusMeters      = 150.0
	scheduledThroughLayoverMin = 10.0
	turnaroundArrivalRadius    = 250.0
	maxTurnaroundSec           = 30 * 60.0
	minLearnedStandSec         = 45.0
	bootstrapStandSec          = 90.0
	leavesEarlyMin             = 2.0
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

type turnaroundStats struct {
	Count     int
	Arrivals  int
	MedianSec float64
	P25Sec    float64
	// Median of departure minus paired slot, over samples that had a slot.
	SlotLatenessMin float64
	SlotSamples     int
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
		stats  map[[2]int]turnaroundStats
		routes map[int]map[string]models.Turnaround
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
	turnaroundCache.stats = make(map[[2]int]turnaroundStats)
	turnaroundCache.routes = make(map[int]map[string]models.Turnaround)
}

func resetTurnaroundCache() {
	turnaroundCache.Lock()
	turnaroundCache.stats = nil
	turnaroundCache.routes = nil
	turnaroundCache.Unlock()
}

func loadTurnaroundStats(routeID, directionID int) turnaroundStats {
	key := [2]int{routeID, directionID}
	turnaroundCache.Lock()
	turnaroundCacheFor(ProfileSnapshotPeriod())
	stats, ok := turnaroundCache.stats[key]
	turnaroundCache.Unlock()
	if ok {
		return stats
	}
	since := profileClock().Add(-turnaroundWindow).Unix()
	type sampleRow struct {
		duration, paired float64
		departed         int64
	}
	samples, err := queryRows(`
		SELECT duration_sec, paired_departure_min, departed_at FROM turnaround_samples
		WHERE route_id = ? AND direction_id = ? AND departed_at >= ?
		ORDER BY duration_sec`,
		[]any{routeID, directionID, since},
		func(rows *sql.Rows) (sampleRow, error) {
			var r sampleRow
			err := rows.Scan(&r.duration, &r.paired, &r.departed)
			return r, err
		})
	if err != nil {
		log.Printf("turnaround: route=%d direction=%d: %v", routeID, directionID, err)
		return turnaroundStats{}
	}
	stats = turnaroundStats{Count: len(samples)}
	_ = database.DB.QueryRow(`SELECT COUNT(*) FROM turnaround_arrivals WHERE route_id = ? AND direction_id = ? AND arrived_at >= ?`,
		routeID, directionID, since).Scan(&stats.Arrivals)
	if len(samples) > 0 {
		durations := make([]float64, len(samples))
		var slotLateness []float64
		loc := time.Local
		for i, s := range samples {
			durations[i] = s.duration
			if s.paired >= 0 {
				departedMin, _ := localMinutes(float64(s.departed), loc)
				slotLateness = append(slotLateness, wrapDayMinutes(departedMin-s.paired))
			}
		}
		stats.MedianSec = nearestRank(durations, 0.5)
		stats.P25Sec = nearestRank(durations, 0.25)
		stats.SlotSamples = len(slotLateness)
		if len(slotLateness) > 0 {
			stats.SlotLatenessMin = median(slotLateness)
		}
	}
	turnaroundCache.Lock()
	turnaroundCacheFor(ProfileSnapshotPeriod())
	turnaroundCache.stats[key] = stats
	turnaroundCache.Unlock()
	return stats
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

// Minutes the timetable leaves between arriving on the feeder direction and the next departure.
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
	if len(arrivals) == 0 {
		return 0, false
	}
	sort.Float64s(departures)
	var gaps []float64
	for _, arrival := range arrivals {
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

func turnaroundFor(routeID, directionID int, from, to []models.StopTime, timetable *models.Timetable) models.Turnaround {
	if len(from) < 2 || len(to) < 2 {
		return models.Turnaround{}
	}
	last, first := from[len(from)-1], to[0]
	if last.StopLat == 0 || first.StopLat == 0 || haversineMeters(last.StopLat, last.StopLon, first.StopLat, first.StopLon) > throughTerminusMeters {
		return models.Turnaround{}
	}
	// The quickest quarter is the stand itself, the rest is waiting for a slot. Buses
	// that mostly leave well before their slot are not keeping that timetable.
	stats := loadTurnaroundStats(routeID, directionID)
	if stats.Arrivals >= turnaroundMinSamples && stats.Count*2 < stats.Arrivals {
		return models.Turnaround{}
	}
	if stats.Count >= turnaroundMinSamples {
		leaves := stats.SlotSamples >= turnaroundMinSamples && stats.SlotLatenessMin <= -leavesEarlyMin
		return models.Turnaround{Through: stats.MedianSec <= turnaroundThroughMaxSec, Sec: math.Round(math.Max(minLearnedStandSec, stats.P25Sec)), Leaves: leaves}
	}
	// Until the stand has been measured, only a terminus with its own arrival stop
	// counts: a loop back into the same stop is usually a depot with a long wait.
	if timetable == nil || last.StopID == first.StopID {
		return models.Turnaround{}
	}
	duration := tripDurationMinutes(from)
	for _, day := range []models.DaySchedule{timetable.Weekdays, timetable.Saturday, timetable.Sunday} {
		layover, ok := scheduledLayoverMinutes(day, duration, directionID)
		if !ok {
			continue
		}
		return models.Turnaround{Through: layover <= scheduledThroughLayoverMin, Sec: bootstrapStandSec}
	}
	return models.Turnaround{}
}

// Both termini of a route, keyed by the direction being departed.
func routeTurnarounds(routeID int, stopTimes []models.StopTime) map[string]models.Turnaround {
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
	result := map[string]models.Turnaround{}
	if t := turnaroundFor(routeID, 0, in, out, timetable); t.Sec > 0 {
		result["0"] = t
	}
	if t := turnaroundFor(routeID, 1, out, in, timetable); t.Sec > 0 {
		result["1"] = t
	}
	turnaroundCache.Lock()
	turnaroundCacheFor(ProfileSnapshotPeriod())
	turnaroundCache.routes[routeID] = result
	turnaroundCache.Unlock()
	return result
}

func applyTurnaroundToStopTimes(stopTimes []models.StopTime, routeID int) {
	turnarounds := routeTurnarounds(routeID, stopTimes)
	if len(turnarounds) == 0 {
		return
	}
	firstByTrip := make(map[string]int)
	for i, st := range stopTimes {
		if j, ok := firstByTrip[st.TripID]; !ok || st.StopSequence < stopTimes[j].StopSequence {
			firstByTrip[st.TripID] = i
		}
	}
	for tripID, i := range firstByTrip {
		dir, ok := directionIDFromTripID(tripID)
		if !ok {
			continue
		}
		if t, ok := turnarounds[strconv.Itoa(dir)]; ok {
			stopTimes[i].ThroughRunning = t.Through
			stopTimes[i].TurnaroundSec = t.Sec
			stopTimes[i].TurnaroundLeaves = t.Leaves
		}
	}
}
