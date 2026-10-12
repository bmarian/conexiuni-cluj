package router

import (
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Arrival is a bus due at a stop, as the stop pages list it.
type Arrival struct {
	At        int64
	Live      bool
	VehicleID int
}

// LiveSource lists what is due at a stop on one direction of a line, soonest first.
type LiveSource func(key string, stop int) []Arrival

type Query struct {
	From, To Point
	Time     time.Time
	ArriveBy bool
	Live     LiveSource
	// How many departures of each option to time.
	Runs int
}

// Leg is a walk or a ride; stop IDs are 0 at the origin and the destination.
type Leg struct {
	Ride     bool
	FromStop int
	ToStop   int
	Passed   []int
	Meters   float64
	Secs     int64
	Path     []Point
}

// Ride is the bus taken on one ride leg of a run.
type Ride struct {
	RouteID   int
	Key       string
	FromStop  int
	ToStop    int
	Depart    int64
	Arrive    int64
	Live      bool
	VehicleID int
}

// Run is one timed departure of an option: leaving the origin, reaching the destination.
type Run struct {
	Depart, Arrive int64
	Rides          []Ride
}

type Option struct {
	Legs       []Leg
	Runs       []Run
	Rides      int
	WalkMeters float64
}

type line struct{ pattern, board, alight int32 }

type skeleton struct {
	steps []step
	lines [][]line
}

// Plan lists one option per sequence of stops, timed from q.Time (or back from it when arriving by),
// keeping only those no other one beats on arriving, on arriving plus walking, and on rides.
func (n *Network) Plan(q Query) []Option {
	access, egress := n.reach(q.From), n.reach(q.To)
	start := q.Time.Unix()
	from, to := n.walk.Snap(q.From), n.walk.Snap(q.To)
	walkMeters, walkPath := n.walk.Path(from, to)
	walk := Option{WalkMeters: walkMeters, Legs: []Leg{{Meters: walkMeters, Secs: n.walkSecs(walkMeters), Path: walkPath}}}
	walk.Runs = []Run{{Depart: start, Arrive: start + walk.Legs[0].Secs}}
	if q.ArriveBy {
		walk.Runs[0] = Run{Depart: start - walk.Legs[0].Secs, Arrive: start}
	}
	var journeys []journey
	if q.ArriveBy {
		for _, j := range n.search(&n.backward, egress, access, -start) {
			journeys = append(journeys, n.unreverse(j))
		}
	} else {
		journeys = n.search(&n.forward, access, egress, start)
	}

	seen := make(map[string]bool)
	var options []Option
	for _, j := range journeys {
		key := n.stopsKey(j)
		if seen[key] {
			continue
		}
		seen[key] = true
		sk := n.skeleton(j)
		runs := n.runs(sk, start, q, walk.Runs[0])
		if len(runs) == 0 {
			continue
		}
		options = append(options, n.option(sk, runs))
	}
	options = n.worthTaking(options, q.ArriveBy)

	for i := range options {
		for l := range options[i].Legs {
			leg := &options[i].Legs[l]
			if leg.Ride {
				continue
			}
			a, b := from, to
			if leg.FromStop != 0 {
				a = n.snaps[n.index[leg.FromStop]]
			}
			if leg.ToStop != 0 {
				b = n.snaps[n.index[leg.ToStop]]
			}
			leg.Meters, leg.Path = n.walk.Path(a, b)
		}
	}
	return append(options, walk)
}

func (n *Network) stopsKey(j journey) string {
	var b strings.Builder
	for _, s := range j.steps {
		if s.kind == byRide {
			b.WriteString(strconv.Itoa(int(s.from)) + ">" + strconv.Itoa(int(s.to)) + "|")
		}
	}
	return b.String()
}

// skeleton keeps the journey's stops and every line that rides between each pair of them.
func (n *Network) skeleton(j journey) skeleton {
	sk := skeleton{steps: j.steps}
	for _, s := range j.steps {
		if s.kind != byRide {
			continue
		}
		var lines []line
		for _, sp := range n.forward.byStop[s.from] {
			stops := n.forward.patterns[sp.pattern].stops
			for pos := sp.pos + 1; pos < int32(len(stops)); pos++ {
				if stops[pos] == s.to {
					lines = append(lines, line{sp.pattern, sp.pos, pos})
					break
				}
			}
		}
		sk.lines = append(sk.lines, lines)
	}
	return sk
}

func (n *Network) option(sk skeleton, runs []Run) Option {
	o := Option{Runs: runs}
	for _, s := range sk.steps {
		leg := Leg{Ride: s.kind == byRide, Secs: s.arrive - s.depart, Meters: s.meters}
		if s.from >= 0 {
			leg.FromStop = n.stops[s.from].ID
		}
		if s.to >= 0 {
			leg.ToStop = n.stops[s.to].ID
		}
		if leg.Ride {
			p := n.forward.patterns[s.pattern]
			for _, st := range p.stops[s.board+1 : s.alight] {
				leg.Passed = append(leg.Passed, n.stops[st].ID)
			}
			o.Rides++
		} else {
			o.WalkMeters += s.meters
		}
		o.Legs = append(o.Legs, leg)
	}
	return o
}

// runs times the option's next departures: q.Runs of them, soonest first (latest first arriving by).
// After the first, a run only counts while it beats walking the whole way, and when it leaves further
// from the one before than runs here stray from their times.
func (n *Network) runs(sk skeleton, start int64, q Query, walk Run) []Run {
	var out []Run
	bound := int64(math.MinInt64)
	if q.ArriveBy {
		bound = math.MaxInt64
	}
	for len(out) < max(1, q.Runs) {
		var run Run
		var ok bool
		if q.ArriveBy {
			run, ok = n.timeBack(sk, start, bound)
		} else {
			run, ok = n.timeForward(sk, start, bound, q.Live)
		}
		if !ok || (len(out) > 0 && (run.Arrive > walk.Arrive || run.Depart < walk.Depart)) {
			break
		}
		out = append(out, run)
		slack := int64(n.opt.TransferSlack.Seconds())
		if q.ArriveBy {
			bound = run.Rides[len(run.Rides)-1].Arrive - slack
		} else {
			bound = run.Rides[0].Depart + slack
		}
	}
	return out
}

// timeForward catches the soonest bus on every ride, the first one leaving after `after`.
func (n *Network) timeForward(sk skeleton, start, after int64, live LiveSource) (Run, bool) {
	t := start
	var run Run
	walked := int64(0)
	leg := 0
	for _, s := range sk.steps {
		if s.kind != byRide {
			t += s.arrive - s.depart
			if leg == 0 {
				walked += s.arrive - s.depart
			}
			continue
		}
		ready := t
		if leg > 0 {
			ready += int64(n.opt.TransferSlack.Seconds())
		} else {
			ready = max(ready, after+1)
		}
		best, ok := Ride{}, false
		for _, l := range sk.lines[leg] {
			if r, found := n.catch(l, ready, live); found && (!ok || r.Arrive < best.Arrive || (r.Arrive == best.Arrive && r.Live && !best.Live)) {
				best, ok = r, true
			}
		}
		if !ok {
			return Run{}, false
		}
		run.Rides = append(run.Rides, best)
		t = best.Arrive
		leg++
	}
	run.Depart = run.Rides[0].Depart - walked
	run.Arrive = t
	return run, true
}

// catch is the first bus of a line at its boarding stop from ready on: what the stop page lists, then the timetable.
func (n *Network) catch(l line, ready int64, live LiveSource) (Ride, bool) {
	p := &n.forward.patterns[l.pattern]
	board, alight := p.times[l.board], p.times[l.alight]
	ride := Ride{RouteID: p.routeID, Key: p.key, FromStop: n.stops[p.stops[l.board]].ID, ToStop: n.stops[p.stops[l.alight]].ID}
	listed := int64(math.MinInt64)
	if live != nil {
		for _, a := range live(p.key, ride.FromStop) {
			listed = max(listed, a.At)
			if a.At < ready {
				continue
			}
			r := nearestRun(board, a.At)
			ride.Depart, ride.Arrive, ride.Live, ride.VehicleID = a.At, a.At+alight[r]-board[r], a.Live, a.VehicleID
			return ride, true
		}
	}
	from := max(ready, listed+1)
	r := sort.Search(len(board), func(i int) bool { return board[i] >= from })
	if r == len(board) {
		return Ride{}, false
	}
	ride.Depart, ride.Arrive = board[r], alight[r]
	return ride, true
}

func nearestRun(col []int64, at int64) int {
	i := sort.Search(len(col), func(i int) bool { return col[i] >= at })
	if i == len(col) || (i > 0 && at-col[i-1] < col[i]-at) {
		return i - 1
	}
	return i
}

// timeBack catches the latest bus on every ride that still makes the next one, arriving before `before`.
func (n *Network) timeBack(sk skeleton, deadline, before int64) (Run, bool) {
	t := deadline
	rides := make([]Ride, len(sk.lines))
	leg := len(sk.lines) - 1
	walked := int64(0)
	for i := len(sk.steps) - 1; i >= 0; i-- {
		s := sk.steps[i]
		if s.kind != byRide {
			t -= s.arrive - s.depart
			if leg == len(sk.lines)-1 {
				walked += s.arrive - s.depart
			}
			continue
		}
		limit := t
		if leg < len(sk.lines)-1 {
			limit -= int64(n.opt.TransferSlack.Seconds())
		} else {
			limit = min(limit, before-1)
		}
		best, ok := Ride{}, false
		for _, l := range sk.lines[leg] {
			p := &n.forward.patterns[l.pattern]
			board, alight := p.times[l.board], p.times[l.alight]
			r := sort.Search(len(alight), func(i int) bool { return alight[i] > limit }) - 1
			if r < 0 {
				continue
			}
			if !ok || board[r] > best.Depart {
				best = Ride{RouteID: p.routeID, Key: p.key, FromStop: n.stops[p.stops[l.board]].ID, ToStop: n.stops[p.stops[l.alight]].ID, Depart: board[r], Arrive: alight[r]}
				ok = true
			}
		}
		if !ok {
			return Run{}, false
		}
		rides[leg] = best
		t = best.Depart
		leg--
	}
	last := rides[len(rides)-1]
	return Run{Depart: t, Arrive: last.Arrive + walked, Rides: rides}, true
}

// worthTaking keeps the options someone would take. Options riding the same lines are one journey,
// kept where it costs least. Then an option goes when another takes no more effort and no more rides
// and is not slower by more than runs here stray from their times. A change costs the margin kept for
// it, and a minute walked is never better than a minute sat.
func (n *Network) worthTaking(options []Option, arriveBy bool) []Option {
	slack := int64(n.opt.TransferSlack.Seconds())
	score := func(o Option) (int64, int64) {
		r := o.Runs[0]
		t := r.Arrive
		if arriveBy {
			t = -r.Depart
		}
		t += int64(o.Rides-1) * slack
		return t, t + n.walkSecs(o.WalkMeters)
	}
	var kept []Option
	for _, o := range options {
		same := -1
		for i, k := range kept {
			if sameLines(o, k) {
				same = i
				break
			}
		}
		if same < 0 {
			kept = append(kept, o)
			continue
		}
		ot, ow := score(o)
		kt, kw := score(kept[same])
		if ow < kw || (ow == kw && ot < kt) {
			kept[same] = o
		}
	}
	var out []Option
	for i, a := range kept {
		at, aw := score(a)
		beaten := false
		for j, b := range kept {
			if i == j {
				continue
			}
			bt, bw := score(b)
			noWorse := bt <= at+slack && bw <= aw && b.Rides <= a.Rides
			better := b.Rides < a.Rides || bw < aw || bt < at
			if noWorse && (better || j < i) {
				beaten = true
				break
			}
		}
		if !beaten {
			out = append(out, a)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if arriveBy {
			return out[i].Runs[0].Depart > out[j].Runs[0].Depart
		}
		return out[i].Runs[0].Arrive < out[j].Runs[0].Arrive
	})
	return out
}

// sameLines: as many rides, and every ride shares a line with the other option's.
func sameLines(a, b Option) bool {
	if a.Rides != b.Rides {
		return false
	}
	for leg := range a.Rides {
		shared := false
		for _, ra := range a.Runs {
			for _, rb := range b.Runs {
				if ra.Rides[leg].RouteID == rb.Rides[leg].RouteID {
					shared = true
				}
			}
		}
		if !shared {
			return false
		}
	}
	return true
}

func (n *Network) unreverse(j journey) journey {
	out := journey{depart: -j.arrive, arrive: -j.depart, rides: j.rides, walk: j.walk, steps: make([]step, len(j.steps))}
	for i, s := range j.steps {
		f := step{kind: s.kind, from: s.to, to: s.from, depart: -s.arrive, arrive: -s.depart, meters: s.meters, pattern: s.pattern}
		if s.kind == byRide {
			last := int32(len(n.forward.patterns[s.pattern].stops) - 1)
			f.board, f.alight = last-s.alight, last-s.board
		}
		out.steps[len(j.steps)-1-i] = f
	}
	return out
}

// QuietEnd is when service resumes after the longest stretch without a departure in the day from at:
// arriving by it is "the last one today".
func (n *Network) QuietEnd(at int64) int64 {
	var deps []int64
	for _, p := range n.forward.patterns {
		for _, t := range p.times[0] {
			if t >= at && t < at+24*3600 {
				deps = append(deps, t)
			}
		}
	}
	sort.Slice(deps, func(i, j int) bool { return deps[i] < deps[j] })
	end, gap := at+24*3600, int64(-1)
	prev := at
	for _, t := range deps {
		if t-prev > gap {
			gap, end = t-prev, t
		}
		prev = t
	}
	if at+24*3600-prev > gap {
		end = at + 24*3600
	}
	return end
}
