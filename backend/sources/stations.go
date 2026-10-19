package sources

import (
	"conexiuni-cluj/models"
	"fmt"
	"math"
	"sort"
	"strings"
)

type Point struct{ Lat, Lon float64 }

// Locate names the town and the street at each point, "" where it does not know.
type Locate func(points []Point) (towns, streets []string, err error)

// Stations groups each name's stops into stations, the platforms of one stop, and names apart stations sharing a name:
// the town where towns differ, unless it is the city's own; the street where one town has two served by the same lines.
// Namesakes nearer than the next stop down their routes are one station, and so are a line's two directions when their next stops overlap.
// Returns how many stops it renamed.
func Stations(snap *Snapshot, locate Locate, home string) (int, error) {
	key := make([]string, len(snap.Stops))
	at := make(map[int]int, len(snap.Stops))
	for i, s := range snap.Stops {
		key[i], at[s.StopID] = normalizeStopLabelForMatch(s.StopName), i
	}
	runs := patternStops(snap, at)

	spacing := make([]float64, len(snap.Stops))
	for _, run := range runs {
		for i, s := range run {
			for _, j := range []int{i - 1, i + 1} {
				if j < 0 || j >= len(run) || key[run[j]] == key[s] {
					continue
				}
				if m := meters(stopPoint(snap, s), stopPoint(snap, run[j])); spacing[s] == 0 || m < spacing[s] {
					spacing[s] = m
				}
			}
		}
	}

	byName := map[string][]int{}
	for i, k := range key {
		byName[k] = append(byName[k], i)
	}
	calls := directionsAt(snap, runs)
	var shared [][][]int
	var points []int
	for _, group := range byName {
		places := placesOf(snap, group, spacing, calls)
		for _, place := range places {
			station := snap.Stops[place[0]].StopID
			for _, s := range place {
				station = min(station, snap.Stops[s].StopID)
			}
			for _, s := range place {
				snap.Stops[s].StationID = station
			}
		}
		if len(places) > 1 {
			shared = append(shared, places)
			for _, place := range places {
				points = append(points, place...)
			}
		}
	}
	if len(points) == 0 {
		return 0, nil
	}

	onRoute := onRoutePoints(snap, runs, at)
	query := make([]Point, len(points))
	for i, s := range points {
		query[i] = onRoute[s]
	}
	towns, streets, err := locate(query)
	if err != nil {
		return 0, err
	}
	town, street := make(map[int]string, len(points)), make(map[int]string, len(points))
	for i, s := range points {
		town[s], street[s] = towns[i], streets[i]
	}
	lines := linesAt(snap, runs)

	original := make([]string, len(snap.Stops))
	for i, s := range snap.Stops {
		original[i] = s.StopName
	}
	renamed := 0
	ownTown := normalizeStopLabelForMatch(home)
	for _, places := range shared {
		type label struct{ town, street, lines string }
		labels := make([]label, len(places))
		townsSeen := map[string]bool{}
		for i, place := range places {
			labels[i] = label{mostCommon(place, town), mostCommon(place, street), placeLines(place, lines)}
			townsSeen[labels[i].town] = true
		}
		for i, place := range places {
			l := labels[i]
			var parts []string
			for j, other := range labels {
				if j != i && other.town == l.town && other.lines == l.lines && l.street != "" {
					parts = append(parts, l.street)
					break
				}
			}
			if len(townsSeen) > 1 && l.town != "" && normalizeStopLabelForMatch(l.town) != ownTown {
				parts = append(parts, l.town)
			}
			if len(parts) == 0 {
				continue
			}
			for _, s := range place {
				snap.Stops[s].StopName += " (" + strings.Join(parts, ", ") + ")"
				renamed++
			}
		}
	}
	if renamed > 0 {
		followRenames(snap, runs, original)
	}
	return renamed, nil
}

// The routes calling at each stop, by stop index.
func linesAt(snap *Snapshot, runs map[string][]int) map[int]map[int]bool {
	routeOf := make(map[string]int, len(snap.Trips))
	for _, t := range snap.Trips {
		routeOf[t.TripID] = t.RouteID
	}
	out := map[int]map[int]bool{}
	for trip, run := range runs {
		for _, s := range run {
			if out[s] == nil {
				out[s] = map[int]bool{}
			}
			out[s][routeOf[trip]] = true
		}
	}
	return out
}

func placeLines(place []int, lines map[int]map[int]bool) string {
	var ids []int
	seen := map[int]bool{}
	for _, s := range place {
		for id := range lines[s] {
			if !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
	}
	sort.Ints(ids)
	return fmt.Sprint(ids)
}

// Timetable ends and route names made of stop names take their stops' new names.
func followRenames(snap *Snapshot, runs map[string][]int, original []string) {
	ends := map[int][]int{}
	for _, t := range snap.Trips {
		if run := runs[t.TripID]; len(run) > 0 {
			ends[t.RouteID] = append(ends[t.RouteID], run[0])
		}
	}
	for i := range snap.Routes {
		r := &snap.Routes[i]
		rename := func(name string) string {
			for _, s := range ends[r.RouteID] {
				if name == original[s] {
					return snap.Stops[s].StopName
				}
			}
			return name
		}
		parts := strings.Split(r.RouteLongName, " - ")
		for k := range parts {
			parts[k] = rename(parts[k])
		}
		r.RouteLongName = strings.Join(parts, " - ")
		if t := snap.Timetables[r.RouteShortName]; t != nil {
			t.InStopName, t.OutStopName = rename(t.InStopName), rename(t.OutStopName)
		}
	}
}

// patternStops are the stops of each pattern in order, as indexes into snap.Stops.
func patternStops(snap *Snapshot, at map[int]int) map[string][]int {
	type call struct{ stop, seq int }
	calls := map[string][]call{}
	for _, ts := range snap.TripStops {
		if i, ok := at[ts.StopID]; ok {
			calls[ts.TripID] = append(calls[ts.TripID], call{i, ts.StopSequence})
		}
	}
	out := make(map[string][]int, len(calls))
	for trip, c := range calls {
		sort.Slice(c, func(a, b int) bool { return c[a].seq < c[b].seq })
		run := make([]int, len(c))
		for i, r := range c {
			run[i] = r.stop
		}
		out[trip] = run
	}
	return out
}

// The directions of each route calling at each stop, by stop index.
func directionsAt(snap *Snapshot, runs map[string][]int) []map[int][2]bool {
	out := make([]map[int][2]bool, len(snap.Stops))
	for _, t := range snap.Trips {
		dir := int(t.DirectionID)
		if dir < 0 || dir > 1 {
			continue
		}
		for _, s := range runs[t.TripID] {
			if out[s] == nil {
				out[s] = map[int][2]bool{}
			}
			d := out[s][t.RouteID]
			d[dir] = true
			out[s][t.RouteID] = d
		}
	}
	return out
}

// Each side of a street served by a line one way: they share a line, never one of its directions.
func oppositeSides(a, b map[int][2]bool) bool {
	shared := false
	for route, da := range a {
		db, ok := b[route]
		if !ok {
			continue
		}
		if (da[0] && db[0]) || (da[1] && db[1]) {
			return false
		}
		shared = true
	}
	return shared
}

// Stops in one group linked, transitively, when nearer than either's spacing, or than both together on opposite sides of a line.
func placesOf(snap *Snapshot, group []int, spacing []float64, calls []map[int][2]bool) [][]int {
	parent := make([]int, len(group))
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(x int) int {
		for parent[x] != x {
			parent[x] = parent[parent[x]]
			x = parent[x]
		}
		return x
	}
	for i := range group {
		for j := i + 1; j < len(group); j++ {
			a, b := group[i], group[j]
			m := meters(stopPoint(snap, a), stopPoint(snap, b))
			if m < math.Max(spacing[a], spacing[b]) || (m < spacing[a]+spacing[b] && oppositeSides(calls[a], calls[b])) {
				parent[find(i)] = find(j)
			}
		}
	}
	byRoot := map[int][]int{}
	var roots []int
	for i, s := range group {
		r := find(i)
		if byRoot[r] == nil {
			roots = append(roots, r)
		}
		byRoot[r] = append(byRoot[r], s)
	}
	out := make([][]int, len(roots))
	for i, r := range roots {
		out[i] = byRoot[r]
	}
	return out
}

// Where the bus stops: the nearest point of the stop's route shapes, so both platforms meet the same road.
func onRoutePoints(snap *Snapshot, runs map[string][]int, at map[int]int) []Point {
	shapeOf := map[string]string{}
	for _, t := range snap.Trips {
		shapeOf[t.TripID] = t.ShapeID
	}
	byShape := map[string][]models.Shape{}
	for _, s := range snap.Shapes {
		byShape[s.ShapeID] = append(byShape[s.ShapeID], s)
	}
	shapes := make(map[string][]Point, len(byShape))
	for id, pts := range byShape {
		sort.Slice(pts, func(a, b int) bool { return pts[a].ShapePtSequence < pts[b].ShapePtSequence })
		line := make([]Point, len(pts))
		for i, s := range pts {
			line[i] = Point{s.ShapePtLat, s.ShapePtLon}
		}
		shapes[id] = line
	}
	out := make([]Point, len(snap.Stops))
	best := make([]float64, len(snap.Stops))
	for i := range out {
		out[i], best[i] = stopPoint(snap, i), math.Inf(1)
	}
	for trip, run := range runs {
		line := shapes[shapeOf[trip]]
		for _, s := range run {
			p := stopPoint(snap, s)
			for k := 1; k < len(line); k++ {
				if q, m := nearestOnSegment(p, line[k-1], line[k]); m < best[s] {
					out[s], best[s] = q, m
				}
			}
		}
	}
	return out
}

func nearestOnSegment(p, a, b Point) (Point, float64) {
	kx := math.Cos(p.Lat * math.Pi / 180)
	ax, ay := (a.Lon-p.Lon)*kx, a.Lat-p.Lat
	bx, by := (b.Lon-p.Lon)*kx, b.Lat-p.Lat
	dx, dy := bx-ax, by-ay
	t := 0.0
	if l := dx*dx + dy*dy; l > 0 {
		t = math.Max(0, math.Min(1, -(ax*dx+ay*dy)/l))
	}
	q := Point{a.Lat + t*(b.Lat-a.Lat), a.Lon + t*(b.Lon-a.Lon)}
	return q, meters(p, q)
}

// The value most of the stops have; a tie goes to the first stop's.
func mostCommon(stops []int, value map[int]string) string {
	count := map[string]int{}
	best := value[stops[0]]
	for _, s := range stops {
		count[value[s]]++
		if count[value[s]] > count[best] {
			best = value[s]
		}
	}
	return best
}

func stopPoint(snap *Snapshot, i int) Point {
	return Point{snap.Stops[i].StopLat, snap.Stops[i].StopLon}
}

func meters(a, b Point) float64 {
	const earthRadiusM = 6371000.0
	dLat := (b.Lat - a.Lat) * math.Pi / 180
	dLon := (b.Lon - a.Lon) * math.Pi / 180
	h := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(a.Lat*math.Pi/180)*math.Cos(b.Lat*math.Pi/180)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * earthRadiusM * math.Asin(math.Sqrt(h))
}
