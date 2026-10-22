package traces

import (
	"conexiuni-cluj/database"
	"conexiuni-cluj/geo"
	"conexiuni-cluj/predictions"
	"conexiuni-cluj/router"
	"math"
	"slices"
	"sort"
	"time"
)

// Line is one traced line's recordings, each bus's fixes in the order it made them.
type Line struct {
	buses [][]database.LineFix
	paths map[[2]router.Point][]router.Point
}

// Fixes come ordered by bus, then time.
func newLine(fixes []database.LineFix) *Line {
	l := &Line{paths: map[[2]router.Point][]router.Point{}}
	for i, f := range fixes {
		if i == 0 || f.Vehicle != fixes[i-1].Vehicle {
			l.buses = append(l.buses, nil)
		}
		l.buses[len(l.buses)-1] = append(l.buses[len(l.buses)-1], f)
	}
	return l
}

// Buses is how many buses were recorded on the line; zero for a nil line.
func (l *Line) Buses() int {
	if l == nil {
		return 0
	}
	return len(l.buses)
}

// Road is the road between two points, from the city's road network.
type Road func(a, b router.Point) []router.Point

func at(f database.LineFix) router.Point { return router.Point{Lat: f.Lat, Lon: f.Lon} }

func distance(a, b router.Point) float64 { return geo.Haversine(a.Lat, a.Lon, b.Lat, b.Lon) }

// A bus within this of a place passed it: off its route otherwise, and never nearer the next place than this one.
func passRadius(a, b router.Point) float64 {
	return min(predictions.MaxOffRouteMeters, distance(a, b)/2)
}

// Path is the road between two places as the line's buses drove from one to the other: their median pass (an odd one, through a gap in what was seen, is not), alone snapped to the road; nil when no bus was seen passing them in that order.
func (l *Line) Path(a, b router.Point, road Road) []router.Point {
	if l == nil {
		return nil
	}
	key := [2]router.Point{a, b}
	if path, ok := l.paths[key]; ok {
		return path
	}
	r := passRadius(a, b)
	var passes [][]router.Point
	for _, bus := range l.buses {
		lastA := -1
		for i, f := range bus {
			switch p := at(f); {
			case distance(p, a) <= r:
				lastA = i
			case distance(p, b) <= r && lastA >= 0:
				pass := []router.Point{a}
				for _, between := range bus[lastA+1 : i] {
					pass = append(pass, at(between))
				}
				passes = append(passes, append(pass, b))
				lastA = -1
			}
		}
	}
	var out []router.Point
	if len(passes) > 0 {
		sort.Slice(passes, func(i, j int) bool { return Length(passes[i]) < Length(passes[j]) })
		out = along(passes[len(passes)/2], road)
	}
	l.paths[key] = out
	return out
}

// The road through every point in turn.
func along(points []router.Point, road Road) []router.Point {
	var out []router.Point
	for i := 1; i < len(points); i++ {
		leg := road(points[i-1], points[i])
		if len(out) > 0 && len(leg) > 0 {
			leg = leg[1:]
		}
		out = append(out, leg...)
	}
	return out
}

func Length(path []router.Point) float64 {
	total := 0.0
	for i := 1; i < len(path); i++ {
		total += distance(path[i-1], path[i])
	}
	return total
}

// End is where buses that passed before (when given) and then last stood longest before coming near a known place again, no further than reach from last: the stand the others are nearest. Backwards finds where they stood before reaching last instead.
func (l *Line) End(before *router.Point, last router.Point, known []router.Point, reach float64, backwards bool) (router.Point, bool) {
	if l == nil {
		return router.Point{}, false
	}
	near := func(p, place router.Point) bool { return distance(p, place) <= predictions.MaxOffRouteMeters }
	var stands []router.Point
	for _, bus := range l.buses {
		if backwards {
			bus = slices.Clone(bus)
			slices.Reverse(bus)
		}
		passed := before == nil
		for i := 0; i < len(bus); i++ {
			p := at(bus[i])
			if before != nil && distance(p, *before) <= passRadius(*before, last) {
				passed = true
				continue
			}
			if !passed || !near(p, last) {
				continue
			}
			best, longest, left := -1, time.Duration(-1), false
			j := i + 1
			for ; j < len(bus); j++ {
				q := at(bus[j])
				left = left || !near(q, last)
				if left && slices.ContainsFunc(known, func(k router.Point) bool { return near(q, k) }) {
					break
				}
				if stood := bus[j].Last.Sub(bus[j].First); stood > longest && distance(q, last) <= reach {
					best, longest = j, stood
				}
			}
			if best >= 0 {
				stands = append(stands, at(bus[best]))
			}
			i, passed = j, before == nil
		}
	}
	if len(stands) == 0 {
		return router.Point{}, false
	}
	return medoid(stands), true
}

// The point nearest, in sum, to all the others.
func medoid(points []router.Point) router.Point {
	best, bestSum := points[0], math.Inf(1)
	for _, a := range points {
		sum := 0.0
		for _, b := range points {
			sum += distance(a, b)
		}
		if sum < bestSum {
			best, bestSum = a, sum
		}
	}
	return best
}

// DistanceToPath is the metres from a point to the nearest point of a path.
func DistanceToPath(p router.Point, path []router.Point) float64 {
	best := math.Inf(1)
	for i := range path {
		if i == 0 {
			best = distance(p, path[0])
			continue
		}
		best = min(best, distanceToSegment(p, path[i-1], path[i]))
	}
	return best
}

func distanceToSegment(p, a, b router.Point) float64 {
	// Flat over a few hundred metres: longitude scaled to latitude.
	k := math.Cos(p.Lat * math.Pi / 180)
	dx, dy := (b.Lon-a.Lon)*k, b.Lat-a.Lat
	t := 0.0
	if l := dx*dx + dy*dy; l > 0 {
		t = max(0, min(1, ((p.Lon-a.Lon)*k*dx+(p.Lat-a.Lat)*dy)/l))
	}
	return distance(p, router.Point{Lat: a.Lat + t*(b.Lat-a.Lat), Lon: a.Lon + t*(b.Lon-a.Lon)})
}
