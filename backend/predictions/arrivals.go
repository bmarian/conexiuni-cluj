// Package predictions turns vehicle positions, learned stop offsets and the timetable
// into the arrival rows every client shows. Pure: no database, no clock of its own, and
// nothing rounded: times stay in seconds until a screen shows them.
package predictions

import (
	"math"
	"sort"
)

// MinutesUntil is the signed distance from nowMinutes to absMinutes, both past midnight,
// read across midnight the short way round.
func MinutesUntil(absMinutes, nowMinutes float64) float64 {
	diff := absMinutes - nowMinutes
	if diff < -720 {
		return diff + 1440
	}
	if diff > 720 {
		return diff - 1440
	}
	return diff
}

// ScheduledArrivals lists the timetable arrivals at one stop, soonest first, as seconds
// from now (nowSec past midnight); horizon <= 0 means no limit. A run up to graceSec
// late, what runs here usually are, is still listed.
func ScheduledArrivals(departureMinutes []int, offsetSec, nowSec, horizonSec, graceSec float64) []float64 {
	out := make([]float64, 0, len(departureMinutes))
	for _, d := range departureMinutes {
		s := MinutesUntil(float64(d)+offsetSec/60, nowSec/60) * 60
		if s >= -graceSec && (horizonSec <= 0 || s < horizonSec) {
			out = append(out, s)
		}
	}
	sort.Float64s(out)
	return out
}

type Arrival struct {
	Seconds float64
	Live    bool
	// The bus behind a live entry; 0 for a timetable run.
	VehicleID int
	Kind      EtaKind
}

type LiveSeconds struct {
	Seconds   float64
	VehicleID int
	Kind      EtaKind
}

// Screens show whole minutes; what rounds to zero reads "now".
func readsNow(seconds float64) bool { return math.Round(seconds/60) == 0 }

// MergeArrivals lists each tracked bus, then only the timetable runs after the last of
// them: everything at or before a live estimate is a run being tracked or one already
// gone. With nothing live, a slot still reading "now" on a tracked trip is a run gone past.
func MergeArrivals(live []LiveSeconds, scheduled []float64, tracked bool, limit int, graceSec float64) []Arrival {
	if limit <= 0 {
		limit = 3
	}
	lives := make([]LiveSeconds, 0, len(live))
	for _, l := range live {
		l.Seconds = math.Max(0, l.Seconds)
		lives = append(lives, l)
	}
	sort.SliceStable(lives, func(i, j int) bool { return lives[i].Seconds < lives[j].Seconds })

	out := make([]Arrival, 0, limit)
	if len(lives) == 0 {
		for _, s := range scheduled {
			// Due within the minute on a trip the feed can see, with no bus here: gone past.
			if tracked && s < 60 {
				continue
			}
			if len(out) == limit {
				break
			}
			out = append(out, Arrival{Seconds: math.Max(0, s)})
		}
		return out
	}
	last := lives[len(lives)-1].Seconds
	for _, l := range lives {
		out = append(out, Arrival{Seconds: l.Seconds, Live: true, VehicleID: l.VehicleID, Kind: l.Kind})
	}
	for _, s := range scheduled {
		if s > last+graceSec {
			out = append(out, Arrival{Seconds: s})
		}
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

type Marked struct {
	Arrival
	Soon bool
}

// MarkRepeatedNow keeps "now" on the first of consecutive zeros down a column and on the
// first zero in a row; the rest read "soon".
func MarkRepeatedNow(rows [][]Arrival) [][]Marked {
	out := make([][]Marked, len(rows))
	for stop, row := range rows {
		out[stop] = make([]Marked, len(row))
		for col := range row {
			a := &row[col]
			soon := false
			if readsNow(a.Seconds) {
				if stop > 0 && col < len(rows[stop-1]) && readsNow(rows[stop-1][col].Seconds) {
					soon = true
				}
				for k := 0; k < col && !soon; k++ {
					if readsNow(row[k].Seconds) {
						soon = true
					}
				}
			}
			out[stop][col] = Marked{Arrival: *a, Soon: soon}
		}
	}
	return out
}
