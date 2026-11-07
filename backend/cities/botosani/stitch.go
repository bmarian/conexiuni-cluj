package botosani

import (
	"conexiuni-cluj/destinations"
	"conexiuni-cluj/router"
	"log"
	"slices"
)

// Through the places in order, changing piece where the next village's stop is not ahead.
func stitch(base mapLine, route []string, pieces []mapLine, villages []router.Area) (mapLine, bool) {
	var out mapLine
	ride := func(p mapLine, from, to int) {
		start := p.at[from]
		for k := from; k <= to; k++ {
			if k == from && len(out.stops) > 0 && out.stops[len(out.stops)-1].id == p.stops[k].id {
				start = p.at[k] + 1
				continue
			}
			out.points = append(out.points, p.points[start:p.at[k]+1]...)
			out.stops = append(out.stops, p.stops[k])
			out.at = append(out.at, len(out.points)-1)
			start = p.at[k] + 1
		}
	}
	var all []marker
	for _, p := range pieces {
		all = append(all, p.stops...)
	}
	markerAt := func(m marker) router.Point { return m.at }
	current, at := base, 0
	ride(base, 0, 0)
	for _, place := range route {
		here := out.stops[len(out.stops)-1].at
		var named []router.Area
		for _, v := range villages {
			if nameKey(v.Name) == nameKey(place) {
				named = append(named, v)
			}
		}
		village, _ := destinations.Nearest(named, here.Lat, here.Lon)
		if village == nil || len(all) == 0 {
			log.Printf("botosani: no village %q in the OSM extract", place)
			return mapLine{}, false
		}
		target := all[nearestTo(all, router.Point{Lat: village.Lat, Lon: village.Lon}, markerAt)]
		if k := slices.IndexFunc(current.stops, func(m marker) bool { return m.id == target.id }); k > at {
			ride(current, at, k)
			at = k
			continue
		}
		var next mapLine
		board, stop := -1, -1
		for _, p := range pieces {
			k := slices.IndexFunc(p.stops, func(m marker) bool { return m.id == target.id })
			if k < 0 || p.name == current.name {
				continue
			}
			if b := nearestTo(p.stops, here, markerAt); board < 0 || distance(p.stops[b].at, here) < distance(next.stops[board].at, here) {
				next, board, stop = p, b, k
			}
		}
		if board < 0 {
			return mapLine{}, false
		}
		if stop < board {
			next, board, stop = backwards(next), len(next.stops)-1-board, len(next.stops)-1-stop
		}
		ride(next, board, stop)
		current, at = next, stop
	}
	return out, len(out.stops) >= 2
}
