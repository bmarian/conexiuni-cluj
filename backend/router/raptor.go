// Package router plans walk + transit journeys: RAPTOR over the timetable, walks on the OSM
// street graph. Pure: no database and no clock of its own.
package router

import (
	"math"
	"slices"
	"sort"
	"time"
)

type Stop struct {
	ID       int
	Lat, Lon float64
}

// PatternInput is one stop sequence and its runs: unix seconds at every stop, one row per run.
type PatternInput struct {
	RouteID int
	Key     string
	Stops   []int
	Trips   [][]int64
	// Riding it takes a ticket of its own.
	OwnTicket bool
}

type Options struct {
	WalkSpeed float64
	// Longest walk to the first stop or from the last one.
	MaxWalk time.Duration
	// Longest walk between two stops.
	MaxTransfer time.Duration
	// Margin kept between alighting and the next boarding.
	TransferSlack time.Duration
	// What buying a ticket of its own is worth in time.
	TicketCost time.Duration
}

// Streets is everything that depends only on the stops and the walk graph.
type Streets struct {
	walk      *WalkGraph
	stops     []Stop
	index     map[int]int32
	snaps     []Snap
	transfers [][]transfer
	opt       Options
}

type transfer struct {
	to     int32
	meters float64
	secs   int64
}

func NewStreets(walk *WalkGraph, stops []Stop, opt Options) *Streets {
	s := &Streets{walk: walk, stops: stops, index: make(map[int]int32, len(stops)), opt: opt}
	s.snaps = make([]Snap, len(stops))
	for i, st := range stops {
		s.index[st.ID] = int32(i)
		s.snaps[i] = walk.Snap(Point{st.Lat, st.Lon})
	}
	s.transfers = make([][]transfer, len(stops))
	for i := range stops {
		for j, m := range walk.Reach(s.snaps[i], s.snaps, s.opt.WalkSpeed*s.opt.MaxTransfer.Seconds()) {
			if j != i {
				s.transfers[i] = append(s.transfers[i], transfer{int32(j), m, s.walkSecs(m)})
			}
		}
		sort.Slice(s.transfers[i], func(a, b int) bool { return s.transfers[i][a].to < s.transfers[i][b].to })
	}
	return s
}

func (s *Streets) maxWalkMeters() float64 { return s.opt.WalkSpeed * s.opt.MaxWalk.Seconds() }

func (s *Streets) walkSecs(meters float64) int64 { return int64(math.Round(meters / s.opt.WalkSpeed)) }

type reach struct {
	stop   int32
	meters float64
	secs   int64
}

func (s *Streets) reach(p Point) []reach {
	var out []reach
	for i, m := range s.walk.Reach(s.walk.Snap(p), s.snaps, s.maxWalkMeters()) {
		out = append(out, reach{int32(i), m, s.walkSecs(m)})
	}
	sort.Slice(out, func(a, b int) bool { return out[a].stop < out[b].stop })
	return out
}

// Walk is the walk-only alternative between two points.
func (s *Streets) Walk(from, to Point) (meters float64, secs int64, path []Point) {
	meters, path = s.walk.Path(s.walk.Snap(from), s.walk.Snap(to))
	return meters, s.walkSecs(meters), path
}

type pattern struct {
	routeID   int
	key       string
	ownTicket bool
	stops     []int32
	// times[pos][run], runs ordered by their time at the first stop.
	times [][]int64
}

func (p *pattern) tickets() int32 {
	if p.ownTicket {
		return 1
	}
	return 0
}

type stopPos struct{ pattern, pos int32 }

type timetable struct {
	patterns []pattern
	byStop   [][]stopPos
}

// Network is one service window of the timetable on top of the streets.
type Network struct {
	*Streets
	forward, backward timetable
}

func (s *Streets) Network(inputs []PatternInput) *Network {
	n := &Network{Streets: s}
	for _, in := range inputs {
		p := pattern{routeID: in.RouteID, key: in.Key, ownTicket: in.OwnTicket}
		known := make([]int, 0, len(in.Stops))
		for pos, id := range in.Stops {
			if idx, ok := s.index[id]; ok {
				p.stops = append(p.stops, idx)
				known = append(known, pos)
			}
		}
		if len(p.stops) < 2 {
			continue
		}
		runs := make([][]int64, 0, len(in.Trips))
		for _, t := range in.Trips {
			if len(t) == len(in.Stops) {
				runs = append(runs, t)
			}
		}
		if len(runs) == 0 {
			continue
		}
		sort.SliceStable(runs, func(a, b int) bool { return runs[a][known[0]] < runs[b][known[0]] })
		p.times = make([][]int64, len(known))
		for i, pos := range known {
			p.times[i] = make([]int64, len(runs))
			for r, run := range runs {
				p.times[i][r] = run[pos]
			}
		}
		n.forward.patterns = append(n.forward.patterns, p)
	}
	n.backward.patterns = make([]pattern, len(n.forward.patterns))
	for i, p := range n.forward.patterns {
		last, runs := len(p.stops)-1, len(p.times[0])
		r := pattern{routeID: p.routeID, key: p.key, ownTicket: p.ownTicket, stops: make([]int32, len(p.stops)), times: make([][]int64, len(p.stops))}
		for pos := range p.stops {
			r.stops[pos] = p.stops[last-pos]
			r.times[pos] = make([]int64, runs)
			for run := range runs {
				r.times[pos][run] = -p.times[last-pos][runs-1-run]
			}
		}
		n.backward.patterns[i] = r
	}
	for _, tt := range []*timetable{&n.forward, &n.backward} {
		tt.byStop = make([][]stopPos, len(s.stops))
		for pi, p := range tt.patterns {
			for pos, st := range p.stops {
				tt.byStop[st] = append(tt.byStop[st], stopPos{int32(pi), int32(pos)})
			}
		}
	}
	return n
}

type how uint8

const (
	byAccess how = iota
	byRide
	byWalk
	byEgress
)

// label is one way of reaching a stop, or the destination: when, after how much walking and how many tickets, and how.
type label struct {
	arr     int64
	walk    float64
	tickets int32
	how     how
	stop    int32
	rides   int32
	parent  *label
	secs    int64
	meters  float64
	// Rides only.
	pattern, run, board, alight int32
}

// bag keeps the labels no other one beats on arriving, walking and tickets.
type bag []*label

func (b bag) beats(arr int64, walk float64, tickets int32) bool {
	for _, l := range b {
		if l.arr <= arr && l.walk <= walk && l.tickets <= tickets {
			return true
		}
	}
	return false
}

func (b *bag) add(l *label) {
	kept := (*b)[:0]
	for _, o := range *b {
		if l.arr > o.arr || l.walk > o.walk || l.tickets > o.tickets {
			kept = append(kept, o)
		}
	}
	*b = append(kept, l)
}

type raptor struct {
	n     *Network
	tt    *timetable
	slack int64
	// arr[k][stop]: with at most k rides, by ride or ride and walk; the next round boards from these.
	arr [][]bag
	// ride[k][stop]: straight off a ride, at most k of them; walks start from these.
	ride   [][]bag
	target []bag
	egress map[int32]reach
	marked []bool
}

func newRaptor(n *Network, tt *timetable, egress []reach) *raptor {
	r := &raptor{n: n, tt: tt, slack: int64(n.opt.TransferSlack.Seconds()), egress: make(map[int32]reach, len(egress)), marked: make([]bool, len(n.stops))}
	for _, e := range egress {
		r.egress[e.stop] = e
	}
	r.round(0)
	return r
}

func (r *raptor) round(k int) {
	for len(r.arr) <= k {
		arr, ride := make([]bag, len(r.n.stops)), make([]bag, len(r.n.stops))
		var target bag
		if prev := len(r.arr) - 1; prev >= 0 {
			for p := range arr {
				arr[p], ride[p] = append(bag(nil), r.arr[prev][p]...), append(bag(nil), r.ride[prev][p]...)
			}
			target = append(bag(nil), r.target[prev]...)
		}
		r.arr, r.ride, r.target = append(r.arr, arr), append(r.ride, ride), append(r.target, target)
	}
}

// beaten: with at most k rides something already gets there as early with as little walking and as few tickets,
// or already reaches the destination so.
func (r *raptor) beaten(bags [][]bag, k int, p int32, arr int64, walk float64, tickets int32) bool {
	return r.target[k].beats(arr, walk, tickets) || (p >= 0 && bags[k][p].beats(arr, walk, tickets))
}

func (r *raptor) insert(bags [][]bag, p int32, l *label) {
	for j := int(l.rides); j < len(bags); j++ {
		bags[j][p].add(l)
	}
}

type step struct {
	kind           how
	from, to       int32
	depart, arrive int64
	meters         float64
	pattern        int32
	board, alight  int32
}

type journey struct {
	depart, arrive int64
	rides          int
	walk           float64
	steps          []step
}

type onboard struct {
	from       *label
	run, board int32
}

func (r *raptor) iterate(depart int64, access []reach) []journey {
	var marked []int32
	mark := func(p int32) {
		if !r.marked[p] {
			r.marked[p] = true
			marked = append(marked, p)
		}
	}
	for _, a := range access {
		if x := depart + a.secs; !r.beaten(r.arr, 0, a.stop, x, a.meters, 0) {
			r.insert(r.arr, a.stop, &label{arr: x, walk: a.meters, how: byAccess, stop: a.stop, secs: a.secs, meters: a.meters})
			mark(a.stop)
		}
	}
	var found []journey
	for k := 1; len(marked) > 0; k++ {
		r.round(k)
		queue := make(map[int32]int32)
		for _, p := range marked {
			r.marked[p] = false
			for _, sp := range r.tt.byStop[p] {
				if pos, ok := queue[sp.pattern]; !ok || sp.pos < pos {
					queue[sp.pattern] = sp.pos
				}
			}
		}
		marked = marked[:0]
		var rode []*label
		for pi, start := range queue {
			p := &r.tt.patterns[pi]
			var route []onboard
			for pos := start; pos < int32(len(p.stops)); pos++ {
				st := p.stops[pos]
				for _, o := range route {
					x, tickets := p.times[pos][o.run], o.from.tickets+p.tickets()
					if r.beaten(r.ride, k, st, x, o.from.walk, tickets) {
						continue
					}
					l := &label{arr: x, walk: o.from.walk, tickets: tickets, how: byRide, stop: st, rides: int32(k), parent: o.from, pattern: pi, run: o.run, board: o.board, alight: pos}
					r.insert(r.ride, st, l)
					rode = append(rode, l)
					if !r.beaten(r.arr, k, st, x, l.walk, l.tickets) {
						r.insert(r.arr, st, l)
						mark(st)
					}
				}
				col := p.times[pos]
				for _, from := range r.arr[k-1][st] {
					if from.rides != int32(k-1) {
						continue
					}
					ready := from.arr
					if k > 1 {
						ready += r.slack
					}
					if j := int32(sort.Search(len(col), func(i int) bool { return col[i] >= ready })); j < int32(len(col)) {
						route = board(route, onboard{from, j, pos})
					}
				}
			}
		}
		for _, l := range rode {
			for _, t := range r.n.transfers[l.stop] {
				x, w := l.arr+t.secs, l.walk+t.meters
				if !r.beaten(r.arr, k, t.to, x, w, l.tickets) {
					r.insert(r.arr, t.to, &label{arr: x, walk: w, tickets: l.tickets, how: byWalk, stop: t.to, rides: int32(k), parent: l, secs: t.secs, meters: t.meters})
					mark(t.to)
				}
			}
		}
		// Only straight off a ride: the walk to the destination already is the shortest from that stop.
		for _, l := range rode {
			e, ok := r.egress[l.stop]
			if !ok {
				continue
			}
			if x, w := l.arr+e.secs, l.walk+e.meters; !r.beaten(nil, k, -1, x, w, l.tickets) {
				t := &label{arr: x, walk: w, tickets: l.tickets, how: byEgress, stop: -1, rides: int32(k), parent: l, secs: e.secs, meters: e.meters}
				for j := k; j < len(r.target); j++ {
					r.target[j].add(t)
				}
				found = append(found, r.backtrack(t))
			}
		}
	}
	return found
}

// board adds a way onto the route unless one on a run no later, with no more walking and tickets, beats it.
// The same run caught further along replaces the earlier boarding: less walking to it.
func board(route []onboard, c onboard) []onboard {
	for _, o := range route {
		noWorse := o.run <= c.run && o.from.walk <= c.from.walk && o.from.tickets <= c.from.tickets
		if noWorse && (o.run < c.run || o.from.walk < c.from.walk || o.from.tickets < c.from.tickets) {
			return route
		}
	}
	kept := route[:0]
	for _, o := range route {
		if c.run > o.run || c.from.walk > o.from.walk || c.from.tickets > o.from.tickets {
			kept = append(kept, o)
		}
	}
	return append(kept, c)
}

func (r *raptor) backtrack(t *label) journey {
	j := journey{arrive: t.arr, walk: t.walk}
	var steps []step
	for l := t; l != nil; l = l.parent {
		switch l.how {
		case byEgress, byWalk:
			to := l.stop
			steps = append(steps, step{kind: byWalk, from: l.parent.stop, to: to, depart: l.parent.arr, arrive: l.arr, meters: l.meters})
		case byRide:
			p := &r.tt.patterns[l.pattern]
			steps = append(steps, step{kind: byRide, from: p.stops[l.board], to: l.stop, depart: p.times[l.board][l.run], arrive: l.arr, pattern: l.pattern, board: l.board, alight: l.alight})
			j.rides++
		case byAccess:
			first := steps[len(steps)-1].depart
			steps = append(steps, step{kind: byAccess, from: -1, to: l.stop, depart: first - l.secs, arrive: first, meters: l.meters})
		}
	}
	slices.Reverse(steps)
	j.depart, j.steps = steps[0].depart, steps
	return j
}

// search runs the rounds from one departure time and returns every journey that reached the destination better.
func (n *Network) search(tt *timetable, access, egress []reach, start int64) []journey {
	return newRaptor(n, tt, egress).iterate(start, access)
}
