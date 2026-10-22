package suceava

import (
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/geo"
	"conexiuni-cluj/models"
	"conexiuni-cluj/router"
	"math"
	"slices"
	"strings"
)

// Where a station of a page stands: on a zip stop when the names agree, else at a point of its own.
type station struct {
	name string
	stop *models.Stop
	at   router.Point
	from source
}

type source int

const (
	unplaced source = iota
	fromZip
	fromName
	spaced
)

func (s station) placed() bool { return s.from != unplaced }

// placer puts the stations of the lines the zip lacks on the map.
type placer struct {
	stops []models.Stop
	words [][]word
	// No two consecutive stops of the zip are further apart.
	maxLeg float64
	// Nil without the OSM extract.
	roads  *router.WalkGraph
	places *router.Places
}

func newPlacer(snap *feeds.Snapshot, roads *router.WalkGraph, places *router.Places) *placer {
	p := &placer{stops: snap.Stops, roads: roads, places: places}
	at := make(map[int]models.Stop, len(snap.Stops))
	for _, s := range snap.Stops {
		at[s.StopID] = s
		p.words = append(p.words, nameWords(s.StopName))
	}
	prev := map[string]models.Stop{}
	for _, ts := range snap.TripStops {
		s := at[ts.StopID]
		if before, ok := prev[ts.TripID]; ok && ts.StopSequence > 0 {
			p.maxLeg = max(p.maxLeg, geo.Haversine(before.StopLat, before.StopLon, s.StopLat, s.StopLon))
		}
		prev[ts.TripID] = s
	}
	return p
}

// Each line's directions with the stations it could place, and how many came from where; a line cut short would end where its buses do not, so one missing an end is left out.
func (p *placer) place(lines []pageLine) ([][][]station, map[source]int) {
	out := make([][][]station, len(lines))
	legs := make([]float64, len(lines))
	for i, line := range lines {
		out[i] = make([][]station, len(line.directions))
		for d, names := range line.directions {
			out[i][d] = p.matchStops(names)
		}
		legs[i] = p.lineLeg(out[i])
	}
	for p.placeNamed(out, legs) {
	}
	counts := map[source]int{}
	for i := range out {
		whole := true
		for d, dir := range out[i] {
			p.interpolate(dir)
			for _, s := range dir {
				counts[s.from]++
			}
			whole = whole && dir[0].placed() && dir[len(dir)-1].placed()
			out[i][d] = slices.DeleteFunc(dir, func(s station) bool { return !s.placed() })
		}
		if !whole {
			out[i] = nil
		}
	}
	return out, counts
}

// The line's own longest leg between stations on the zip's stops, the zip's longest when it has none: town lines have short legs, village lines long ones.
func (p *placer) lineLeg(dirs [][]station) float64 {
	leg := 0.0
	for _, dir := range dirs {
		for i := 1; i < len(dir); i++ {
			if dir[i-1].from == fromZip && dir[i].from == fromZip {
				leg = max(leg, geo.Haversine(dir[i-1].at.Lat, dir[i-1].at.Lon, dir[i].at.Lat, dir[i].at.Lon))
			}
		}
	}
	if leg == 0 {
		return p.maxLeg
	}
	return leg
}

// matchStops puts stations on the zip stops named like them, the chain of same-named stops the bus drives least, again without any stop too far from the rest.
func (p *placer) matchStops(names []string) []station {
	cand := make([][]int, len(names))
	for i, name := range names {
		cand[i] = p.candidates(name)
	}
	for {
		out := p.chain(names, cand)
		wrong := p.farFetched(out)
		if len(wrong) == 0 {
			return out
		}
		for _, i := range wrong {
			cand[i] = nil
		}
	}
}

// The candidates, one per station that has any, with the least distance along the way.
func (p *placer) chain(names []string, cand [][]int) []station {
	out := make([]station, len(names))
	var at []int
	for i, name := range names {
		out[i].name = name
		if len(cand[i]) > 0 {
			at = append(at, i)
		}
	}
	if len(at) == 0 {
		return out
	}
	cost := make([][]float64, len(at))
	from := make([][]int, len(at))
	for k, i := range at {
		cost[k], from[k] = make([]float64, len(cand[i])), make([]int, len(cand[i]))
		if k == 0 {
			continue
		}
		for c, stop := range cand[i] {
			cost[k][c], from[k][c] = math.Inf(1), -1
			for b, prev := range cand[at[k-1]] {
				if d := cost[k-1][b] + p.stopDistance(prev, stop); d < cost[k][c] {
					cost[k][c], from[k][c] = d, b
				}
			}
		}
	}
	last := len(at) - 1
	best := 0
	for c := range cost[last] {
		if cost[last][c] < cost[last][best] {
			best = c
		}
	}
	for k := last; k >= 0; k-- {
		s := &p.stops[cand[at[k]][best]]
		out[at[k]] = station{name: names[at[k]], stop: s, at: router.Point{Lat: s.StopLat, Lon: s.StopLon}, from: fromZip}
		best = from[k][best]
	}
	return out
}

// The zip stops whose name holds every word of the station's, the closest names only.
func (p *placer) candidates(name string) []int {
	words := nameWords(name)
	var out []int
	bestShare := 0.0
	for i, stopWords := range p.words {
		if len(words) == 0 || wordsShared(words, stopWords) < len(words) {
			continue
		}
		share := float64(len(words)) / float64(len(stopWords))
		switch {
		case share > bestShare:
			out, bestShare = []int{i}, share
		case share == bestShare:
			out = append(out, i)
		}
	}
	return out
}

func (p *placer) stopDistance(a, b int) float64 {
	return geo.Haversine(p.stops[a].StopLat, p.stops[a].StopLon, p.stops[b].StopLat, p.stops[b].StopLon)
}

// Stops further from their neighbours than the zip's longest legs reach: the one far from both, else all far from one, as two that disagree cannot both be trusted.
func (p *placer) farFetched(dir []station) []int {
	worst, worstN := -1, 0
	var single []int
	for i := range dir {
		if !dir[i].placed() {
			continue
		}
		prev, next := neighbours(dir, i)
		n := 0
		if prev >= 0 && !within(dir[prev].at, dir[i].at, p.maxLeg*float64(i-prev)) {
			n++
		}
		if next >= 0 && !within(dir[i].at, dir[next].at, p.maxLeg*float64(next-i)) {
			n++
		}
		if n > worstN {
			worst, worstN = i, n
		}
		if n == 1 {
			single = append(single, i)
		}
	}
	switch worstN {
	case 0:
		return nil
	case 1:
		return single
	}
	return []int{worst}
}

func within(a, b router.Point, meters float64) bool {
	return geo.Haversine(a.Lat, a.Lon, b.Lat, b.Lon) <= meters
}

// The nearest placed stations before and after i, -1 when there is none.
func neighbours(dir []station, i int) (int, int) {
	prev, next := -1, -1
	for j := i - 1; j >= 0 && prev < 0; j-- {
		if dir[j].placed() {
			prev = j
		}
	}
	for j := i + 1; j < len(dir) && next < 0; j++ {
		if dir[j].placed() {
			next = j
		}
	}
	return prev, next
}

// placeNamed puts a station where the line already has its name, else at the OSM place or other line's station of that name nearest the way between its neighbours; false when it placed none.
func (p *placer) placeNamed(lines [][][]station, legs []float64) bool {
	placedAt := map[string][]router.Point{}
	for _, line := range lines {
		for _, dir := range line {
			for _, s := range dir {
				if s.placed() && s.stop == nil {
					key := feeds.NormalizeStopLabel(s.name)
					placedAt[key] = append(placedAt[key], s.at)
				}
			}
		}
	}
	progress := false
	for l, line := range lines {
		for _, dir := range line {
			for i := range dir {
				if dir[i].placed() {
					continue
				}
				key := feeds.NormalizeStopLabel(dir[i].name)
				if at, ok := placedOnLine(line, key); ok {
					dir[i].at, dir[i].from = at, fromName
					progress = true
					continue
				}
				prev, next := neighbours(dir, i)
				if prev < 0 && next < 0 {
					continue
				}
				cand := append([]router.Point(nil), placedAt[key]...)
				if p.places != nil {
					words := nameWords(dir[i].name)
					for _, name := range spellings(dir[i].name) {
						for _, hit := range p.places.Search(name, p.places.Count()) {
							// A label ends with the area the place is in; a station is named after the place itself.
							own, _, _ := strings.Cut(hit.Label, ", ")
							if wordsShared(words, nameWords(own)) == len(words) {
								cand = append(cand, router.Point{Lat: hit.Lat, Lon: hit.Lon})
							}
						}
					}
				}
				best, bestDetour := router.Point{}, math.Inf(1)
				for _, c := range cand {
					detour, ok := detour(dir, prev, i, next, c, legs[l])
					if ok && detour < bestDetour {
						best, bestDetour = c, detour
					}
				}
				if math.IsInf(bestDetour, 1) {
					continue
				}
				dir[i].at, dir[i].from = p.onRoad(best), fromName
				placedAt[key] = append(placedAt[key], dir[i].at)
				progress = true
			}
		}
	}
	return progress
}

func placedOnLine(line [][]station, key string) (router.Point, bool) {
	for _, dir := range line {
		for _, s := range dir {
			if s.placed() && s.stop == nil && feeds.NormalizeStopLabel(s.name) == key {
				return s.at, true
			}
		}
	}
	return router.Point{}, false
}

// How much further than straight between its neighbours a bus goes through c; false when a leg is longer than the line's own could be.
func detour(dir []station, prev, i, next int, c router.Point, leg float64) (float64, bool) {
	total, direct := 0.0, 0.0
	if prev >= 0 {
		if !within(dir[prev].at, c, leg*float64(i-prev)) {
			return 0, false
		}
		total += geo.Haversine(dir[prev].at.Lat, dir[prev].at.Lon, c.Lat, c.Lon)
	}
	if next >= 0 {
		if !within(c, dir[next].at, leg*float64(next-i)) {
			return 0, false
		}
		total += geo.Haversine(c.Lat, c.Lon, dir[next].at.Lat, dir[next].at.Lon)
	}
	if prev >= 0 && next >= 0 {
		direct = geo.Haversine(dir[prev].at.Lat, dir[prev].at.Lon, dir[next].at.Lat, dir[next].at.Lon)
	}
	return total - direct, true
}

// Buses stop on the road, not at the shop a station is named after.
func (p *placer) onRoad(at router.Point) router.Point {
	if p.roads == nil {
		return at
	}
	return p.roads.Snap(at).Proj
}

// interpolate spaces the stations nothing names evenly along the road between their placed neighbours.
func (p *placer) interpolate(dir []station) {
	for i := 0; i < len(dir); i++ {
		if dir[i].placed() {
			continue
		}
		prev, next := neighbours(dir, i)
		if prev < 0 || next < 0 {
			continue
		}
		path := p.path(dir[prev].at, dir[next].at)
		total := pathLength(path)
		for j := prev + 1; j < next; j++ {
			dir[j].at, dir[j].from = pointAlong(path, total*float64(j-prev)/float64(next-prev)), spaced
		}
		i = next
	}
}

// The road between two points, straight without the OSM extract.
func (p *placer) path(a, b router.Point) []router.Point {
	if p.roads == nil {
		return []router.Point{a, b}
	}
	_, points := p.roads.Path(p.roads.Snap(a), p.roads.Snap(b))
	return points
}

func pathLength(path []router.Point) float64 {
	total := 0.0
	for i := 1; i < len(path); i++ {
		total += geo.Haversine(path[i-1].Lat, path[i-1].Lon, path[i].Lat, path[i].Lon)
	}
	return total
}

func pointAlong(path []router.Point, meters float64) router.Point {
	for i := 1; i < len(path); i++ {
		leg := geo.Haversine(path[i-1].Lat, path[i-1].Lon, path[i].Lat, path[i].Lon)
		if meters <= leg && leg > 0 {
			f := meters / leg
			return router.Point{Lat: path[i-1].Lat + f*(path[i].Lat-path[i-1].Lat), Lon: path[i-1].Lon + f*(path[i].Lon-path[i-1].Lon)}
		}
		meters -= leg
	}
	return path[len(path)-1]
}

// The road a bus drives through a direction's stations.
func (p *placer) shape(dir []station) []router.Point {
	var out []router.Point
	for i := 1; i < len(dir); i++ {
		leg := p.path(dir[i-1].at, dir[i].at)
		if len(out) > 0 && len(leg) > 0 {
			leg = leg[1:]
		}
		out = append(out, leg...)
	}
	return out
}
