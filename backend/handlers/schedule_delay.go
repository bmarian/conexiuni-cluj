package handlers

import (
	"conexiuni-cluj/database"
	"conexiuni-cluj/models"
	"database/sql"
	"fmt"
	"log"
	"math"
	"sort"
	"strconv"
	"time"
)

// How much later than timetable + summed segment profiles buses really arrive.
const (
	scheduleDelayWindow     = 14 * 24 * time.Hour
	scheduleDelayRefresh    = time.Hour
	scheduleDelayMinRuns    = 5
	scheduleDelayShrinkRuns = 10
	// A run is only tied to a departure when it was seen near the start of its trip.
	scheduleDelayMaxAnchorPos = 3
	scheduleDelayMaxMatchMin  = 8.0
)

type scheduleDelayKey struct {
	RouteID        int
	DirectionID    int
	DayType        string
	BucketStartMin int
}

type scheduleResiduals struct {
	seconds []float64
	runs    map[int]struct{}
}

type runSegment struct {
	FromStopID int
	ToStopID   int
	DayType    string
	StartedAt  float64
	EndedAt    int64
}

func StartScheduleDelayLearner(loc *time.Location) {
	go func() {
		time.Sleep(2 * time.Minute)
		for {
			started := time.Now()
			if n, err := recomputeScheduleDelays(loc, started); err != nil {
				log.Printf("schedule delay: %v", err)
			} else {
				log.Printf("schedule delay: %d profiles in %s", n, time.Since(started).Round(time.Millisecond))
			}
			time.Sleep(scheduleDelayRefresh)
		}
	}()
}

func recomputeScheduleDelays(loc *time.Location, now time.Time) (int, error) {
	routes, err := queryRows(`SELECT route_id, route_short_name FROM routes`, nil, func(rows *sql.Rows) (models.Route, error) {
		var r models.Route
		err := rows.Scan(&r.RouteID, &r.RouteShortName)
		return r, err
	})
	if err != nil {
		return 0, err
	}

	residuals := make(map[scheduleDelayKey]*scheduleResiduals)
	runID := 0
	for _, route := range routes {
		timetable, err := getTimetableFromDB(route.RouteShortName)
		if err != nil || timetable == nil {
			continue
		}
		rid := strconv.Itoa(route.RouteID)
		stopTimes, err := getStopTimesForTrips([]string{rid + OUTGOING_SUFFIX, rid + INCOMING_SUFFIX})
		if err != nil {
			return 0, err
		}
		for directionID, suffix := range []string{OUTGOING_SUFFIX, INCOMING_SUFFIX} {
			stops := filterAndSortStopTimesBySuffix(stopTimes, suffix)
			if len(stops) < 2 {
				continue
			}
			if err := collectScheduleResiduals(route.RouteID, directionID, stops, timetable, loc, now, residuals, &runID); err != nil {
				return 0, fmt.Errorf("route %s direction %d: %w", route.RouteShortName, directionID, err)
			}
		}
	}
	return storeScheduleDelays(residuals, now)
}

func collectScheduleResiduals(
	routeID, directionID int,
	stops []models.StopTime,
	timetable *models.Timetable,
	loc *time.Location,
	now time.Time,
	into map[scheduleDelayKey]*scheduleResiduals,
	runID *int,
) error {
	segments, err := queryRows(`
		SELECT from_stop_id, to_stop_id, day_type, observed_at - duration_sec, observed_at
		FROM segment_travel_time_samples
		WHERE route_id = ?
		  AND direction_id = ?
		  AND observed_at >= ?`,
		[]any{routeID, directionID, now.Add(-scheduleDelayWindow).Unix()},
		func(rows *sql.Rows) (runSegment, error) {
			var s runSegment
			err := rows.Scan(&s.FromStopID, &s.ToStopID, &s.DayType, &s.StartedAt, &s.EndedAt)
			return s, err
		})
	if err != nil || len(segments) == 0 {
		return err
	}

	position := make(map[int]int, len(stops))
	for i, st := range stops {
		if _, ok := position[st.StopID]; !ok {
			position[st.StopID] = i
		}
	}

	// A run's segments chain: each starts at the moment and stop the previous one ended.
	type chainKey struct {
		at   int64
		stop int
	}
	byEnd := make(map[chainKey]runSegment, len(segments))
	continued := make(map[chainKey]bool, len(segments))
	for _, s := range segments {
		if _, dup := byEnd[chainKey{s.EndedAt, s.ToStopID}]; !dup {
			byEnd[chainKey{s.EndedAt, s.ToStopID}] = s
		}
		continued[chainKey{int64(math.Round(s.StartedAt)), s.FromStopID}] = true
	}

	profileRows := make(map[string][]segmentProfileRow)
	cumulative := make(map[string][]float64)
	offsetsAt := func(dayType string, bucket int) ([]float64, error) {
		key := dayType + "|" + strconv.Itoa(bucket)
		if c, ok := cumulative[key]; ok {
			return c, nil
		}
		rows, ok := profileRows[dayType]
		if !ok {
			var err error
			if rows, err = loadSegmentProfileRows(routeID, directionID, dayType); err != nil {
				return nil, err
			}
			profileRows[dayType] = rows
		}
		trip := make([]models.StopTime, len(stops))
		copy(trip, stops)
		ordered := make([]int, len(trip))
		for i := range ordered {
			ordered[i] = i
		}
		applySegmentProfilesToTrip(trip, ordered, selectSegmentProfiles(rows, bucket))
		c := make([]float64, len(trip))
		total := 0.0
		for i, st := range trip {
			total += st.OffsetArrivalTime
			c[i] = total
		}
		cumulative[key] = c
		return c, nil
	}
	departures := make(map[string][]float64)

	for _, tail := range segments {
		if continued[chainKey{tail.EndedAt, tail.ToStopID}] {
			continue
		}
		seen := map[int]float64{tail.ToStopID: float64(tail.EndedAt), tail.FromStopID: tail.StartedAt}
		cur := tail
		for {
			prev, ok := byEnd[chainKey{int64(math.Round(cur.StartedAt)), cur.FromStopID}]
			if !ok {
				break
			}
			seen[prev.FromStopID] = prev.StartedAt
			cur = prev
		}

		anchorPos, anchorAt := -1, 0.0
		for stopID, at := range seen {
			if pos, ok := position[stopID]; ok && (anchorPos < 0 || pos < anchorPos) {
				anchorPos, anchorAt = pos, at
			}
		}
		if anchorPos < 0 || anchorPos > scheduleDelayMaxAnchorPos {
			continue
		}

		deps, ok := departures[tail.DayType]
		if !ok {
			deps = scheduleDepartureMinutes(timetable, tail.DayType, directionID)
			departures[tail.DayType] = deps
		}
		if len(deps) < 2 {
			continue
		}

		anchorMin, anchorBucket := localMinutes(anchorAt, loc)
		offsets, err := offsetsAt(tail.DayType, anchorBucket)
		if err != nil {
			return err
		}
		departure, ok := matchDeparture(deps, anchorMin-offsets[anchorPos]/60)
		if !ok {
			continue
		}

		*runID++
		for stopID, at := range seen {
			pos, ok := position[stopID]
			if !ok || pos == 0 {
				continue
			}
			arrivedMin, bucket := localMinutes(at, loc)
			offsets, err := offsetsAt(tail.DayType, bucket)
			if err != nil {
				return err
			}
			residual := wrapDayMinutes(arrivedMin-(departure+offsets[pos]/60)) * 60
			for _, b := range []int{bucket, allDaySegmentBucket} {
				key := scheduleDelayKey{routeID, directionID, tail.DayType, b}
				r := into[key]
				if r == nil {
					r = &scheduleResiduals{runs: make(map[int]struct{})}
					into[key] = r
				}
				r.seconds = append(r.seconds, residual)
				r.runs[*runID] = struct{}{}
			}
		}
	}
	return nil
}

// The timetable departure a run belongs to, unless it sits too far between two.
func matchDeparture(sorted []float64, implied float64) (float64, bool) {
	best := -1
	for i, d := range sorted {
		if best < 0 || math.Abs(wrapDayMinutes(implied-d)) < math.Abs(wrapDayMinutes(implied-sorted[best])) {
			best = i
		}
	}
	gap := math.Inf(1)
	if best > 0 {
		gap = sorted[best] - sorted[best-1]
	}
	if best+1 < len(sorted) {
		gap = math.Min(gap, sorted[best+1]-sorted[best])
	}
	if math.Abs(wrapDayMinutes(implied-sorted[best])) > math.Min(scheduleDelayMaxMatchMin, 0.4*gap) {
		return 0, false
	}
	return sorted[best], true
}

func scheduleDepartureMinutes(t *models.Timetable, dayType string, directionID int) []float64 {
	day := t.Weekdays
	switch dayType {
	case "saturday":
		day = t.Saturday
	case "sunday":
		day = t.Sunday
	}
	var out []float64
	for _, e := range day.Entries {
		raw := e.DepartureIn
		if directionID == 1 {
			raw = e.DepartureOut
		}
		if m, ok := parseDepartureMinutes(raw); ok {
			out = append(out, float64(m))
		}
	}
	sort.Float64s(out)
	return out
}

func localMinutes(unixSec float64, loc *time.Location) (float64, int) {
	sec := math.Floor(unixSec)
	t := time.Unix(int64(sec), int64((unixSec-sec)*1e9)).In(loc)
	minutes := float64(t.Hour()*60+t.Minute()) + (float64(t.Second())+float64(t.Nanosecond())/1e9)/60
	return minutes, segmentBucketStartMin(t)
}

func wrapDayMinutes(m float64) float64 {
	for m > 720 {
		m -= 1440
	}
	for m <= -720 {
		m += 1440
	}
	return m
}

func storeScheduleDelays(residuals map[scheduleDelayKey]*scheduleResiduals, now time.Time) (int, error) {
	tx, err := database.DB.Begin()
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`DELETE FROM schedule_delay_profiles`); err != nil {
		return 0, err
	}
	stmt, err := tx.Prepare(`
		INSERT INTO schedule_delay_profiles
		(route_id, direction_id, day_type, bucket_start_min, run_count, delay_sec, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer func() { _ = stmt.Close() }()

	stored := 0
	for key, r := range residuals {
		runs := len(r.runs)
		if runs < scheduleDelayMinRuns {
			continue
		}
		// Shrunk toward zero so a handful of runs cannot shift a whole hour.
		delay := median(r.seconds) * float64(runs) / float64(runs+scheduleDelayShrinkRuns)
		if _, err := stmt.Exec(key.RouteID, key.DirectionID, key.DayType, key.BucketStartMin, runs, delay, now.Unix()); err != nil {
			return 0, err
		}
		stored++
	}
	return stored, tx.Commit()
}

func median(values []float64) float64 {
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	n := len(sorted)
	if n%2 == 1 {
		return sorted[n/2]
	}
	return (sorted[n/2-1] + sorted[n/2]) / 2
}

// Same bucket preference as segment profiles: this hour, the nearest one, then all day.
func loadScheduleDelay(routeID, directionID int, dayType string, bucket int) (float64, bool) {
	rows, err := queryRows(`
		SELECT bucket_start_min, delay_sec
		FROM schedule_delay_profiles
		WHERE route_id = ? AND direction_id = ? AND day_type = ?`,
		[]any{routeID, directionID, dayType},
		func(rows *sql.Rows) ([2]float64, error) {
			var b int
			var d float64
			err := rows.Scan(&b, &d)
			return [2]float64{float64(b), d}, err
		})
	if err != nil {
		log.Printf("schedule delay: route=%d direction=%d: %v", routeID, directionID, err)
		return 0, false
	}
	bestPriority, delay, found := 0, 0.0, false
	for _, r := range rows {
		priority, ok := segmentProfilePriority(int(r[0]), bucket)
		if ok && (!found || priority < bestPriority) {
			bestPriority, delay, found = priority, r[1], true
		}
	}
	return delay, found
}
