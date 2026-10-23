package botosani

import (
	"cmp"
	"conexiuni-cluj/feeds/roads"
	"conexiuni-cluj/geo"
	"conexiuni-cluj/router"
	"math"
	"slices"
)

type placedLine struct {
	runs    [][]marker
	missing []string
	unused  []marker
}

type streets struct{ g *router.WalkGraph }

func (s streets) path(a, b router.Point) []router.Point { return roads.Path(s.g, a, b) }

func placeRuns(line pageLine, st streets) placedLine {
	var out placedLine
	byKey := map[string][]marker{}
	for _, m := range line.markers {
		byKey[nameKey(m.name)] = append(byKey[nameKey(m.name)], m)
	}
	centre := func(key string) router.Point {
		var c router.Point
		for _, m := range byKey[key] {
			c.Lat += m.at.Lat / float64(len(byKey[key]))
			c.Lon += m.at.Lon / float64(len(byKey[key]))
		}
		return c
	}
	used := map[string]bool{}
	out.runs = make([][]marker, len(line.runs))
	for i, r := range line.runs {
		var keys []string
		for _, name := range r.names {
			if key := nameKey(name); len(byKey[key]) > 0 {
				keys = append(keys, key)
			} else {
				out.missing = append(out.missing, name)
			}
		}
		keys = onceEachWay(keys, centre)
		if len(keys) < 2 {
			continue
		}
		for j, key := range keys {
			var stops []marker
			switch candidates := byKey[key]; {
			case j == 0:
				stops = []marker{furthest(candidates, centre(keys[1]))}
			case j == len(keys)-1:
				stops = st.arriving(candidates, centre(keys[j-1]), centre(key))
			default:
				stops = []marker{st.rightmost(candidates, centre(keys[j-1]), centre(keys[j+1]))}
			}
			for _, m := range stops {
				out.runs[i] = append(out.runs[i], m)
				used[m.id] = true
			}
		}
	}
	for _, m := range line.markers {
		if !used[m.id] && !oppositePartner(out.runs, m) {
			out.unused = append(out.unused, m)
		}
	}
	return out
}

func onceEachWay(keys []string, centre func(string) router.Point) []string {
	turn := len(keys)
	if len(keys) > 2 && keys[0] == keys[len(keys)-1] {
		start, far := centre(keys[0]), 0.0
		for i, key := range keys {
			if d := distance(start, centre(key)); d > far {
				far, turn = d, i+1
			}
		}
	}
	var out []string
	for _, way := range [][]string{keys[:turn], keys[turn:]} {
		seen := map[string]bool{}
		for _, key := range way {
			if !seen[key] {
				seen[key] = true
				out = append(out, key)
			}
		}
	}
	return out
}

// Buses keep right.
func (s streets) rightmost(candidates []marker, from, to router.Point) marker {
	best := candidates[0]
	for _, c := range candidates[1:] {
		if s.side(c.at, from, to) < s.side(best.at, from, to) {
			best = c
		}
	}
	return best
}

func (s streets) arriving(candidates []marker, from, to router.Point) []marker {
	var out []marker
	for _, c := range candidates {
		if s.side(c.at, from, to) < 0 {
			out = append(out, c)
		}
	}
	if len(out) == 0 {
		return []marker{s.rightmost(candidates, from, to)}
	}
	slices.SortFunc(out, func(a, b marker) int { return cmp.Compare(distance(from, a.at), distance(from, b.at)) })
	return out
}

// Negative right of p's road, driving from one station towards the next.
func (s streets) side(p, from, to router.Point) float64 {
	scale := math.Cos(p.Lat * math.Pi / 180)
	planar := func(q router.Point) (float64, float64) { return q.Lon * scale, q.Lat }
	fx, fy := planar(from)
	tx, ty := planar(to)
	a, b, ok := roads.Way(s.g, p)
	if !ok {
		a, b = from, to
	}
	ax, ay := planar(a)
	bx, by := planar(b)
	dx, dy := bx-ax, by-ay
	if dx*(tx-fx)+dy*(ty-fy) < 0 {
		ax, ay, dx, dy = bx, by, -dx, -dy
	}
	px, py := planar(p)
	return (dx*(py-ay) - dy*(px-ax)) / math.Hypot(dx, dy)
}

// Buses leave a terminus from where they turned.
func furthest(candidates []marker, from router.Point) marker {
	best := candidates[0]
	for _, c := range candidates[1:] {
		if distance(c.at, from) > distance(best.at, from) {
			best = c
		}
	}
	return best
}

func oppositePartner(runs [][]marker, m marker) bool {
	key := nameKey(m.name)
	var ways [][2]string
	for _, r := range runs {
		for j := 1; j+1 < len(r); j++ {
			if nameKey(r[j].name) == key {
				ways = append(ways, [2]string{nameKey(r[j+1].name), nameKey(r[j-1].name)})
			}
		}
	}
	placed := false
	for i, r := range runs {
		for j := 1; j < len(r); j++ {
			if slices.Contains(ways, [2]string{nameKey(r[j-1].name), nameKey(r[j].name)}) {
				runs[i] = slices.Insert(r, j, m)
				placed = true
				break
			}
		}
	}
	return placed
}

func distance(a, b router.Point) float64 { return geo.Haversine(a.Lat, a.Lon, b.Lat, b.Lon) }
