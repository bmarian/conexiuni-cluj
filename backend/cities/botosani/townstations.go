package botosani

import (
	"conexiuni-cluj/router"
	"math"
)

type station struct {
	markers []marker
	centre  router.Point
}

func cityStations(lines []pageLine) []station {
	byKey := map[string]int{}
	var out []station
	seen := map[string]bool{}
	for _, l := range lines {
		for _, m := range l.markers {
			if seen[m.id] {
				continue
			}
			seen[m.id] = true
			i, ok := byKey[nameKey(m.name)]
			if !ok {
				i = len(out)
				byKey[nameKey(m.name)] = i
				out = append(out, station{})
			}
			out[i].markers = append(out[i].markers, m)
		}
	}
	for i := range out {
		out[i].centre = centreOf(out[i].markers)
	}
	return out
}

// Each other's nearest, within half the way to another station's marker.
func sharedStations(stations []station, stops []marker) map[string]int {
	type side struct {
		at      router.Point
		station int
	}
	var sides []side
	for i, s := range stations {
		for _, m := range s.markers {
			sides = append(sides, side{m.at, i})
		}
	}
	out := map[string]int{}
	if len(stops) == 0 {
		return out
	}
	sideAt := func(s side) router.Point { return s.at }
	for k, sd := range sides {
		stop := stops[nearestTo(stops, sd.at, func(m marker) router.Point { return m.at })]
		if nearestTo(sides, stop.at, sideAt) != k {
			continue
		}
		reach := math.Inf(1)
		for _, o := range sides {
			if o.station != sd.station {
				reach = min(reach, distance(o.at, sd.at)/2)
			}
		}
		if distance(sd.at, stop.at) < reach {
			out[stop.id] = sd.station
		}
	}
	return out
}
