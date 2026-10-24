package router

import (
	"bufio"
	"container/heap"
	"fmt"
	"io"
	"math"
	"sync"
)

type Point struct{ Lat, Lon float64 }

type BBox struct{ MinLat, MinLon, MaxLat, MaxLon float64 }

func (b BBox) contains(lat, lon float64) bool {
	return lat >= b.MinLat && lat <= b.MaxLat && lon >= b.MinLon && lon <= b.MaxLon
}

func (b BBox) Grow(meters float64) BBox {
	dLat := meters / metersPerDegree
	dLon := meters / (metersPerDegree * math.Cos((b.MinLat+b.MaxLat)/2*math.Pi/180))
	return BBox{b.MinLat - dLat, b.MinLon - dLon, b.MaxLat + dLat, b.MaxLon + dLon}
}

const (
	earthRadiusM    = 6371000.0
	metersPerDegree = earthRadiusM * math.Pi / 180
	// Only decides how many segments one snap looks at.
	gridCellDeg = 0.002
)

type segments struct {
	lat, lon []float64
	segU     []int32
	segV     []int32
	grid     map[[2]int32][]int32
	box      BBox
}

func (g *segments) addSegment(u, v int32) {
	s := int32(len(g.segU))
	g.segU, g.segV = append(g.segU, u), append(g.segV, v)
	for _, cell := range g.segmentCells(s) {
		g.grid[cell] = append(g.grid[cell], s)
	}
}

// WalkGraph is the walkable OSM network around the stops: undirected, lengths in meters.
type WalkGraph struct {
	segments
	first  []int32
	to     []int32
	meters []float32
	pool   sync.Pool
}

func (g *WalkGraph) Nodes() int { return len(g.lat) }

// The walkable ways inside box, largest connected piece only, and the places there.
func Load(r io.Reader, box BBox) (*WalkGraph, *Places, error) { return load(r, box, walkable) }

// LoadRoads is Load over the ways a bus can drive.
func LoadRoads(r io.Reader, box BBox) (*WalkGraph, *Places, error) { return load(r, box, drivable) }

func load(r io.Reader, box BBox, usable func(tag func(string) string) bool) (*WalkGraph, *Places, error) {
	defer holdExtract()()
	index := make(map[int64]int32)
	var lat, lon []float64
	var edges [][2]int32
	var raw []pbfPlace
	keep := pbfFilter{
		node: func(_ int64, lat, lon float64) (bool, bool) {
			in := box.contains(lat, lon)
			return in, in
		},
		way:   func(_ int64, tag func(string) string) (string, bool) { return "", usable(tag) },
		place: placeOf,
	}
	err := readPBF(bufio.NewReaderSize(r, 1<<20), keep, func(b pbfBlock) {
		for _, n := range b.nodes {
			index[n.id] = int32(len(lat))
			lat, lon = append(lat, n.lat), append(lon, n.lon)
		}
		for _, w := range b.ways {
			refs := w.refs
			for i := 1; i < len(refs); i++ {
				a, okA := index[refs[i-1]]
				b, okB := index[refs[i]]
				if okA && okB && a != b {
					edges = append(edges, [2]int32{a, b})
				}
			}
		}
		for _, p := range b.places {
			if i, ok := index[p.node]; !p.placed && ok {
				p.lat, p.lon, p.placed = lat[i], lon[i], true
			}
			if p.placed {
				raw = append(raw, p)
			}
		}
	})
	if err != nil {
		return nil, nil, err
	}
	if len(edges) == 0 {
		return nil, nil, fmt.Errorf("graph: no usable ways inside %+v", box)
	}
	g := buildWalkGraph(lat, lon, edges)
	g.box = box
	return g, newPlaces(raw, box), nil
}

var (
	walkableHighways = map[string]bool{
		"footway": true, "pedestrian": true, "path": true, "steps": true, "living_street": true,
		"residential": true, "service": true, "unclassified": true, "road": true, "track": true,
		"cycleway": true, "bridleway": true, "corridor": true, "platform": true,
		"tertiary": true, "tertiary_link": true, "secondary": true, "secondary_link": true,
		"primary": true, "primary_link": true, "trunk": true, "trunk_link": true,
	}
	footAllowed  = map[string]bool{"yes": true, "designated": true, "permissive": true, "destination": true}
	accessDenied = map[string]bool{"no": true, "private": true}

	drivableHighways = map[string]bool{
		"motorway": true, "motorway_link": true, "trunk": true, "trunk_link": true, "primary": true, "primary_link": true,
		"secondary": true, "secondary_link": true, "tertiary": true, "tertiary_link": true, "unclassified": true,
		"residential": true, "living_street": true, "service": true, "road": true, "busway": true,
	}
	busAllowed = map[string]bool{"yes": true, "designated": true, "permissive": true}
)

func drivable(tag func(string) string) bool {
	if !drivableHighways[tag("highway")] {
		return false
	}
	if busAllowed[tag("bus")] || busAllowed[tag("psv")] {
		return true
	}
	return !accessDenied[tag("access")] && !accessDenied[tag("motor_vehicle")]
}

func walkable(tag func(string) string) bool {
	foot := tag("foot")
	if foot == "no" || foot == "use_sidepath" {
		return false
	}
	platform := tag("railway") == "platform" || tag("public_transport") == "platform"
	if !walkableHighways[tag("highway")] && !platform && !footAllowed[foot] {
		return false
	}
	if accessDenied[tag("access")] && !footAllowed[foot] {
		return false
	}
	return true
}

func buildWalkGraph(lat, lon []float64, edges [][2]int32) *WalkGraph {
	parent := make([]int32, len(lat))
	for i := range parent {
		parent[i] = int32(i)
	}
	var find func(int32) int32
	find = func(x int32) int32 {
		for parent[x] != x {
			parent[x] = parent[parent[x]]
			x = parent[x]
		}
		return x
	}
	for _, e := range edges {
		a, b := find(e[0]), find(e[1])
		if a != b {
			parent[a] = b
		}
	}
	size := make(map[int32]int)
	for _, e := range edges {
		size[find(e[0])]++
	}
	var main int32
	for root, n := range size {
		if n > size[main] || (n == size[main] && root < main) {
			main = root
		}
	}

	renumber := make([]int32, len(lat))
	for i := range renumber {
		renumber[i] = -1
	}
	g := &WalkGraph{segments: segments{grid: make(map[[2]int32][]int32)}}
	seen := make(map[[2]int32]bool, len(edges))
	degree := []int32{}
	for _, e := range edges {
		if find(e[0]) != main {
			continue
		}
		u, v := e[0], e[1]
		if u > v {
			u, v = v, u
		}
		if seen[[2]int32{u, v}] {
			continue
		}
		seen[[2]int32{u, v}] = true
		for _, x := range [2]int32{u, v} {
			if renumber[x] < 0 {
				renumber[x] = int32(len(g.lat))
				g.lat, g.lon = append(g.lat, lat[x]), append(g.lon, lon[x])
				degree = append(degree, 0)
			}
		}
		nu, nv := renumber[u], renumber[v]
		g.addSegment(nu, nv)
		degree[nu]++
		degree[nv]++
	}

	g.first = make([]int32, len(g.lat)+1)
	for i, d := range degree {
		g.first[i+1] = g.first[i] + d
	}
	g.to = make([]int32, g.first[len(g.lat)])
	g.meters = make([]float32, len(g.to))
	fill := append([]int32(nil), g.first[:len(g.lat)]...)
	for s := range g.segU {
		u, v := g.segU[s], g.segV[s]
		m := float32(haversine(g.lat[u], g.lon[u], g.lat[v], g.lon[v]))
		g.to[fill[u]], g.meters[fill[u]] = v, m
		fill[u]++
		g.to[fill[v]], g.meters[fill[v]] = u, m
		fill[v]++
	}
	return g
}

func cellOf(lat, lon float64) [2]int32 {
	return [2]int32{int32(math.Floor(lat / gridCellDeg)), int32(math.Floor(lon / gridCellDeg))}
}

func (g *segments) segmentCells(s int32) [][2]int32 {
	a := cellOf(g.lat[g.segU[s]], g.lon[g.segU[s]])
	b := cellOf(g.lat[g.segV[s]], g.lon[g.segV[s]])
	var out [][2]int32
	for i := min(a[0], b[0]); i <= max(a[0], b[0]); i++ {
		for j := min(a[1], b[1]); j <= max(a[1], b[1]); j++ {
			out = append(out, [2]int32{i, j})
		}
	}
	return out
}

// Snap is a point attached to the nearest segment; Off is the straight walk onto it.
type Snap struct {
	Point  Point
	Proj   Point
	Off    float64
	Seg    int32
	U, V   int32
	DU, DV float64
}

func (g *segments) Snap(p Point) Snap {
	center := cellOf(p.Lat, p.Lon)
	best := Snap{Point: p, Off: math.Inf(1), Seg: -1, U: -1, V: -1}
	cellMeters := gridCellDeg * metersPerDegree * math.Cos(p.Lat*math.Pi/180)
	for ring := int32(0); ; ring++ {
		for i := center[0] - ring; i <= center[0]+ring; i++ {
			for j := center[1] - ring; j <= center[1]+ring; j++ {
				if max(abs32(i-center[0]), abs32(j-center[1])) != ring {
					continue
				}
				for _, s := range g.grid[[2]int32{i, j}] {
					if c := g.project(p, s); c.Off < best.Off {
						best = c
					}
				}
			}
		}
		if best.Off <= float64(ring)*cellMeters || ring > g.maxRing(center) {
			return best
		}
	}
}

func (g *segments) Ends(s Snap) (Point, Point) {
	return Point{g.lat[s.U], g.lon[s.U]}, Point{g.lat[s.V], g.lon[s.V]}
}

func (g *segments) maxRing(center [2]int32) int32 {
	lo, hi := cellOf(g.box.MinLat, g.box.MinLon), cellOf(g.box.MaxLat, g.box.MaxLon)
	return max(abs32(center[0]-lo[0]), abs32(center[0]-hi[0]), abs32(center[1]-lo[1]), abs32(center[1]-hi[1]))
}

func abs32(x int32) int32 {
	if x < 0 {
		return -x
	}
	return x
}

func (g *segments) project(p Point, s int32) Snap {
	u, v := g.segU[s], g.segV[s]
	kx := math.Cos(p.Lat * math.Pi / 180)
	ax, ay := (g.lon[u]-p.Lon)*kx, g.lat[u]-p.Lat
	bx, by := (g.lon[v]-p.Lon)*kx, g.lat[v]-p.Lat
	dx, dy := bx-ax, by-ay
	t := 0.0
	if l := dx*dx + dy*dy; l > 0 {
		t = math.Max(0, math.Min(1, -(ax*dx+ay*dy)/l))
	}
	proj := Point{g.lat[u] + t*(g.lat[v]-g.lat[u]), g.lon[u] + t*(g.lon[v]-g.lon[u])}
	seg := haversine(g.lat[u], g.lon[u], g.lat[v], g.lon[v])
	return Snap{
		Point: p, Proj: proj, Off: haversine(p.Lat, p.Lon, proj.Lat, proj.Lon),
		Seg: s, U: u, V: v, DU: t * seg, DV: (1 - t) * seg,
	}
}

type search struct {
	dist    []float64
	prev    []int32
	touched []int32
	queue   nodeQueue
}

func (g *WalkGraph) acquire() *search {
	if s, ok := g.pool.Get().(*search); ok {
		return s
	}
	s := &search{dist: make([]float64, len(g.lat)), prev: make([]int32, len(g.lat))}
	for i := range s.dist {
		s.dist[i] = math.Inf(1)
	}
	return s
}

func (g *WalkGraph) release(s *search) {
	for _, n := range s.touched {
		s.dist[n] = math.Inf(1)
	}
	s.touched, s.queue = s.touched[:0], s.queue[:0]
	g.pool.Put(s)
}

func (s *search) relax(n int32, d float64, from int32, h float64) {
	if d >= s.dist[n] {
		return
	}
	if math.IsInf(s.dist[n], 1) {
		s.touched = append(s.touched, n)
	}
	s.dist[n], s.prev[n] = d, from
	heap.Push(&s.queue, queued{n, d, d + h})
}

// run settles nodes from the snap outward until limit meters, or until target (A*) is settled.
func (g *WalkGraph) run(s *search, from Snap, limit float64, target *Snap) {
	h := func(n int32) float64 {
		if target == nil {
			return 0
		}
		return haversine(g.lat[n], g.lon[n], target.Proj.Lat, target.Proj.Lon)
	}
	s.relax(from.U, from.DU, -1, h(from.U))
	s.relax(from.V, from.DV, -1, h(from.V))
	for s.queue.Len() > 0 {
		q := heap.Pop(&s.queue).(queued)
		n := q.node
		if q.dist > s.dist[n] {
			continue
		}
		if s.dist[n] > limit {
			return
		}
		if target != nil && q.key >= math.Min(s.dist[target.U]+target.DU, s.dist[target.V]+target.DV) {
			return
		}
		for e := g.first[n]; e < g.first[n+1]; e++ {
			s.relax(g.to[e], s.dist[n]+float64(g.meters[e]), n, h(g.to[e]))
		}
	}
}

// along is the walk between two snaps on the graph, without their off-graph parts.
func along(s *search, a, b Snap) (float64, int32) {
	viaU, viaV := s.dist[b.U]+b.DU, s.dist[b.V]+b.DV
	best, end := viaU, b.U
	if viaV < best {
		best, end = viaV, b.V
	}
	if sameSegment(a, b) {
		if direct := math.Abs(a.DU - b.DU); direct <= best {
			return direct, -1
		}
	}
	return best, end
}

func sameSegment(a, b Snap) bool {
	return (a.U == b.U && a.V == b.V) || (a.U == b.V && a.V == b.U)
}

// Reach returns the walking meters from p to every target within limit, by index.
func (g *WalkGraph) Reach(from Snap, targets []Snap, limit float64) map[int]float64 {
	s := g.acquire()
	defer g.release(s)
	g.run(s, from, limit, nil)
	out := make(map[int]float64)
	for i, t := range targets {
		m, _ := along(s, from, t)
		if m += from.Off + t.Off; m <= limit {
			out[i] = m
		}
	}
	return out
}

// Path is the walk from a to b: total meters and the points to draw.
func (g *WalkGraph) Path(a, b Snap) (float64, []Point) {
	s := g.acquire()
	defer g.release(s)
	g.run(s, a, math.Inf(1), &b)
	m, end := along(s, a, b)
	if math.IsInf(m, 1) {
		m = haversine(a.Point.Lat, a.Point.Lon, b.Point.Lat, b.Point.Lon)
		return m, []Point{a.Point, b.Point}
	}
	var nodes []Point
	for n := end; n >= 0; n = s.prev[n] {
		nodes = append(nodes, Point{g.lat[n], g.lon[n]})
	}
	points := []Point{a.Point, a.Proj}
	for i := len(nodes) - 1; i >= 0; i-- {
		points = append(points, nodes[i])
	}
	points = append(points, b.Proj, b.Point)
	return m + a.Off + b.Off, dedupe(points)
}

func dedupe(points []Point) []Point {
	out := points[:0:0]
	for _, p := range points {
		if len(out) == 0 || out[len(out)-1] != p {
			out = append(out, p)
		}
	}
	return out
}

type queued struct {
	node      int32
	dist, key float64
}

type nodeQueue []queued

func (q nodeQueue) Len() int           { return len(q) }
func (q nodeQueue) Less(i, j int) bool { return q[i].key < q[j].key }
func (q nodeQueue) Swap(i, j int)      { q[i], q[j] = q[j], q[i] }
func (q *nodeQueue) Push(x any)        { *q = append(*q, x.(queued)) }
func (q *nodeQueue) Pop() any {
	old := *q
	x := old[len(old)-1]
	*q = old[:len(old)-1]
	return x
}

func haversine(lat1, lon1, lat2, lon2 float64) float64 {
	dLat := (lat2 - lat1) * math.Pi / 180
	dLon := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * earthRadiusM * math.Asin(math.Sqrt(a))
}
