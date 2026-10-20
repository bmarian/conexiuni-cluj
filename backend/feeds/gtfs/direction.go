package gtfs

import (
	"conexiuni-cluj/geo"
	"math"
	"strings"
)

const unknownDirection = -1

type point struct{ lat, lon float64 }

// A route none of whose trips names a direction takes 0 from its usual run and 1 for the runs heading back; it returns how many routes that was.
func inferDirections(trips map[string]tripInfo, stops tripStops, at map[string]point) int {
	unnamed, named := map[string][]string{}, map[string]bool{}
	for tripID, info := range trips {
		if info.direction == unknownDirection {
			unnamed[info.routeID] = append(unnamed[info.routeID], tripID)
		} else {
			named[info.routeID] = true
		}
	}
	inferred := 0
	for routeID, tripIDs := range unnamed {
		from, to, ok := usualEnds(tripIDs, stops)
		if !named[routeID] && ok {
			inferred++
		}
		for _, tripID := range tripIDs {
			info := trips[tripID]
			info.direction = 0
			if !named[routeID] && ok && headsBack(stops[tripID], from, to, at) {
				info.direction = 1
			}
			trips[tripID] = info
		}
	}
	return inferred
}

// The first and last stop of the stop sequence most of these trips follow.
func usualEnds(tripIDs []string, stops tripStops) (from, to string, ok bool) {
	count := map[string]int{}
	best, n := "", 0
	for _, tripID := range tripIDs {
		run := stops[tripID]
		if len(run) < 2 {
			continue
		}
		seq := make([]string, len(run))
		for i, s := range run {
			seq[i] = s.stopID
		}
		sig := strings.Join(seq, "\x00")
		count[sig]++
		if c := count[sig]; c > n || (c == n && sig < best) {
			best, n = sig, c
		}
	}
	if n == 0 {
		return "", "", false
	}
	seq := strings.Split(best, "\x00")
	return seq[0], seq[len(seq)-1], true
}

func headsBack(run []stopTime, from, to string, at map[string]point) bool {
	if len(run) < 2 {
		return false
	}
	distance := func(a, b string) float64 {
		p, okP := at[a]
		q, okQ := at[b]
		if !okP || !okQ {
			return math.NaN()
		}
		return geo.Haversine(p.lat, p.lon, q.lat, q.lon)
	}
	start, end := run[0].stopID, run[len(run)-1].stopID
	return distance(start, to)+distance(end, from) < distance(start, from)+distance(end, to)
}
