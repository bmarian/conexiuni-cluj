package live

import (
	"conexiuni-cluj/database"
	"conexiuni-cluj/models"
	"conexiuni-cluj/transit"
	"database/sql"
	"math"
	"slices"
	"time"
)

type LineStats struct {
	DayType    string          `json:"day_type"`
	Directions []LineDirection `json:"directions"`
}

type LineDirection struct {
	TripSec  *float64     `json:"trip_sec,omitempty"`
	DelaySec *float64     `json:"delay_sec,omitempty"`
	Hours    []LineHour   `json:"hours"`
	Slowest  *LineStretch `json:"slowest,omitempty"`
}

type LineHour struct {
	Hour     int      `json:"hour"`
	TripSec  *float64 `json:"trip_sec,omitempty"`
	DelaySec *float64 `json:"delay_sec,omitempty"`
}

type LineStretch struct {
	From     string  `json:"from"`
	To       string  `json:"to"`
	Hour     int     `json:"hour"`
	ExtraSec float64 `json:"extra_sec"`
}

type stretchMean struct {
	n   int
	sum float64
}

func (m stretchMean) mean() float64 { return m.sum / float64(m.n) }

func (m stretchMean) measured() bool { return m.n >= minSegmentProfileSamples }

func (eng *Engine) LineStats(routeID int, now time.Time) (LineStats, error) {
	dayType := eng.profileDay(now)
	out := LineStats{DayType: dayType}
	stopTimes, err := eng.StopTimesForTrips([]string{models.PatternID(routeID, 0), models.PatternID(routeID, 1)})
	if err != nil {
		return out, err
	}
	window, args := eng.currentDayWindow(dayType, now).sql("observed_at")
	for directionID, suffix := range []string{models.OutgoingSuffix, models.IncomingSuffix} {
		d, err := eng.lineDirection(routeID, directionID, dayType, transit.SortedBySuffix(stopTimes, suffix), window, args)
		if err != nil {
			return out, err
		}
		out.Directions = append(out.Directions, d)
	}
	return out, nil
}

func (eng *Engine) lineDirection(routeID, directionID int, dayType string, stops []models.StopTime, window string, windowArgs []any) (LineDirection, error) {
	d := LineDirection{Hours: []LineHour{}}
	if len(stops) < 2 {
		return d, nil
	}
	type stretchHour struct{ from, to, bucket int }
	byHour := make(map[stretchHour]stretchMean)
	err := database.QueryEach(eng.DB, `
		SELECT from_stop_id, to_stop_id, bucket_start_min, COUNT(*), SUM(duration_sec)
		FROM segment_travel_time_samples
		WHERE route_id = ? AND direction_id = ? AND day_type = ? AND `+window+`
		GROUP BY from_stop_id, to_stop_id, bucket_start_min`,
		append([]any{routeID, directionID, dayType}, windowArgs...),
		func(rows *sql.Rows) error {
			var k stretchHour
			var m stretchMean
			if err := rows.Scan(&k.from, &k.to, &k.bucket, &m.n, &m.sum); err != nil {
				return err
			}
			byHour[k] = m
			return nil
		})
	if err != nil {
		return d, err
	}

	delays, err := eng.loadScheduleDelayRows(routeID, directionID, dayType)
	if err != nil {
		return d, err
	}
	delayAt := make(map[int]float64, len(delays))
	for _, r := range delays {
		if r.bucket == allDaySegmentBucket {
			d.DelaySec = &r.delay
			continue
		}
		delayAt[r.bucket] = r.delay
	}

	allDay := make([]stretchMean, len(stops)-1)
	first, last := math.MaxInt, math.MinInt
	for i := range allDay {
		for k, m := range byHour {
			if stops[i].StopID == k.from && stops[i+1].StopID == k.to {
				allDay[i].n += m.n
				allDay[i].sum += m.sum
				first, last = min(first, k.bucket), max(last, k.bucket)
			}
		}
	}
	d.TripSec = tripSeconds(allDay, eng.assumedStretches(stops, routeID, directionID, dayType, allDaySegmentBucket))

	var perHour [][]stretchMean
	for b := first; b <= last; b += segmentBucketMinutes {
		stretches := make([]stretchMean, len(stops)-1)
		for i := range stretches {
			stretches[i] = byHour[stretchHour{stops[i].StopID, stops[i+1].StopID, b}]
		}
		perHour = append(perHour, stretches)
		h := LineHour{Hour: b / 60, TripSec: tripSeconds(stretches, eng.assumedStretches(stops, routeID, directionID, dayType, b))}
		if delay, ok := delayAt[b]; ok {
			h.DelaySec = &delay
		}
		d.Hours = append(d.Hours, h)
	}
	fastest, slowest := -1, -1
	for i, h := range d.Hours {
		if h.TripSec == nil {
			continue
		}
		if fastest < 0 || *h.TripSec < *d.Hours[fastest].TripSec {
			fastest = i
		}
		if slowest < 0 || *h.TripSec > *d.Hours[slowest].TripSec {
			slowest = i
		}
	}
	if fastest == slowest {
		return d, nil
	}
	slow, fast := perHour[slowest], perHour[fastest]
	worst := -1
	for i := range slow {
		if !slow[i].measured() || !fast[i].measured() {
			continue
		}
		if worst < 0 || slow[i].mean()-fast[i].mean() > slow[worst].mean()-fast[worst].mean() {
			worst = i
		}
	}
	if worst < 0 {
		return d, nil
	}
	d.Slowest = &LineStretch{
		From:     stops[worst].StopHeadsign,
		To:       stops[worst+1].StopHeadsign,
		Hour:     d.Hours[slowest].Hour,
		ExtraSec: slow[worst].mean() - fast[worst].mean(),
	}
	return d, nil
}

// Nil when no stretch was measured.
func tripSeconds(stretches []stretchMean, assumed []float64) *float64 {
	total, measured := 0.0, false
	for i, m := range stretches {
		if m.measured() {
			total += m.mean()
			measured = true
		} else {
			total += assumed[i]
		}
	}
	if !measured {
		return nil
	}
	return &total
}

func (eng *Engine) assumedStretches(stops []models.StopTime, routeID, directionID int, dayType string, bucket int) []float64 {
	trip := slices.Clone(stops)
	ordered := make([]int, len(trip))
	for i := range ordered {
		ordered[i] = i
	}
	eng.timeTrip(trip, ordered, models.PatternID(routeID, directionID), dayType, bucket, eng.snapshotSegmentProfiles(routeID, directionID, dayType, bucket))
	out := make([]float64, len(trip)-1)
	for i := range out {
		out[i] = trip[i+1].OffsetArrivalTime
	}
	return out
}
