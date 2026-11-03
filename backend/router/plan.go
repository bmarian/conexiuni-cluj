package router

import (
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

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
	// The day's last buses: walking the whole way only counts when there are none.
	Last bool
}

// Leg is a walk or a ride; stop IDs are 0 at the origin and the destination.
type Leg struct {
	Ride      bool
	FromStop  int
	ToStop    int
	Passed    []int
	Meters    float64
	Secs      int64
	Path      []Point
	OwnTicket bool
}

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
	// Rides that need a ticket of their own.
	Tickets      int
	ridePatterns []map[int32]bool
}

type line struct{ pattern, board, alight int32 }

type skeleton struct {
	steps []step
	lines [][]line
	// The first ride's step, and the walk before it.
	first  int
	access int64
	// Walked in all, and tickets of its own.
	walk    float64
	tickets int
}

// One option per stop sequence, kept unless another beats it on arrival, walking, rides and tickets.
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
	find := func(skip map[int32]bool) []Option {
		var journeys []journey
		if q.ArriveBy {
			for _, j := range n.search(&n.backward, egress, access, -start, walk.Legs[0].Secs, skip) {
				journeys = append(journeys, n.unreverse(j))
			}
		} else {
			journeys = n.search(&n.forward, access, egress, start, walk.Legs[0].Secs, skip)
		}
		seen := make(map[string]bool)
		var options []Option
		for _, j := range journeys {
			key := n.stopsKey(j)
			if seen[key] {
				continue
			}
			seen[key] = true
			sk := n.skeleton(j, skip)
			runs := n.runs(sk, start, q, walk.Legs[0].Secs)
			if len(runs) == 0 {
				continue
			}
			options = append(options, n.option(sk, runs))
		}
		return options
	}
	options := n.worthTaking(find(nil), walk, q)
	if !q.Last && len(options) == 1 {
		if second, ok := n.secondWay(options[0], walk, q, find); ok {
			options = append(options, second)
		}
	}

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
	if q.Last && len(options) > 0 {
		return options
	}
	return append(options, walk)
}

func (n *Network) stopsKey(j journey) string {
	var b strings.Builder
	for _, s := range j.steps {
		if s.kind == byRide {
			b.WriteString(strconv.Itoa(int(s.from)) + ">" + strconv.Itoa(int(s.to)) + ":" + strconv.Itoa(int(n.forward.patterns[s.pattern].tickets())) + "|")
		}
	}
	return b.String()
}

// skeleton keeps the journey's stops and every line outside skip that rides between each pair of them on the same ticket.
func (n *Network) skeleton(j journey, skip map[int32]bool) skeleton {
	sk := skeleton{steps: j.steps}
	for i, s := range j.steps {
		if s.kind != byRide {
			sk.walk += s.meters
			if len(sk.lines) == 0 {
				sk.access += s.arrive - s.depart
			}
			continue
		}
		if len(sk.lines) == 0 {
			sk.first = i
		}
		var lines []line
		own := n.forward.patterns[s.pattern].ownTicket
		sk.tickets += int(n.forward.patterns[s.pattern].tickets())
		for _, sp := range n.forward.byStop[s.from] {
			if n.forward.patterns[sp.pattern].ownTicket != own || skip[sp.pattern] {
				continue
			}
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
	for _, lines := range sk.lines {
		ride := make(map[int32]bool, len(lines))
		for _, l := range lines {
			ride[l.pattern] = true
		}
		o.ridePatterns = append(o.ridePatterns, ride)
	}
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
			leg.OwnTicket = p.ownTicket
			o.Rides++
			o.Tickets += int(p.tickets())
		} else {
			o.WalkMeters += s.meters
		}
		o.Legs = append(o.Legs, leg)
	}
	return o
}

// q.Runs departures, each further from the last than runs stray; after the first, only those beating a walk.
func (n *Network) runs(sk skeleton, start int64, q Query, walkSecs int64) []Run {
	var out []Run
	slack := int64(n.opt.TransferSlack.Seconds())
	bound := int64(math.MinInt64)
	if q.ArriveBy {
		bound = math.MaxInt64
	}
	for len(out) < max(1, q.Runs) {
		var run Run
		var ok bool
		if q.ArriveBy {
			run, ok = n.timeBack(sk, start, bound, walkSecs)
		} else {
			run, ok = n.timeForward(sk, start, bound, q.Live)
		}
		if !ok || (len(out) > 0 && !q.Last && !beatsWalk(run, start, q.ArriveBy, walkSecs)) {
			break
		}
		out = append(out, run)
		if q.ArriveBy {
			bound = run.Rides[0].Depart - slack
		} else {
			bound = run.Rides[0].Depart + slack
		}
	}
	return out
}

// Arrives no later than walking from start would, or (arriving by) leaves no earlier.
func beatsWalk(run Run, start int64, arriveBy bool, walkSecs int64) bool {
	if arriveBy {
		return run.Depart >= start-walkSecs
	}
	return run.Arrive <= start+walkSecs
}

// Walking takes no more effort than the journey, by worthTaking's costs.
func (n *Network) walkBeats(trip int64, walkMeters float64, rides, tickets int, walkSecs int64) bool {
	slack := int64(n.opt.TransferSlack.Seconds())
	x := int64(max(rides-1, 0))*slack + int64(tickets)*int64(n.opt.TicketCost.Seconds())
	return trip >= walkSecs-x-slack && trip >= 2*walkSecs-x-n.walkSecs(walkMeters)
}

// timeForward catches the soonest bus on the first ride leaving after `after`, then on every ride after it.
func (n *Network) timeForward(sk skeleton, start, after int64, live LiveSource) (Run, bool) {
	first, ok := n.catchAny(sk.lines[0], max(start+sk.access, after+1), live)
	if !ok {
		return Run{}, false
	}
	return n.ride(sk, first, live)
}

// The latest run that arrives by deadline, its first bus leaving before `before`.
func (n *Network) timeBack(sk skeleton, deadline, before, walkSecs int64) (Run, bool) {
	var runs []Run
	for _, l := range sk.lines[0] {
		board := n.forward.patterns[l.pattern].times[l.board]
		end := sort.Search(len(board), func(i int) bool { return board[i] >= before })
		// Runs reach the destination in time up to one of them and late from the next on.
		r := sort.Search(end, func(i int) bool {
			run, found := n.ride(sk, n.rideOn(l, i), nil)
			return !found || run.Arrive > deadline
		}) - 1
		for ; r >= 0; r-- {
			if run, _ := n.ride(sk, n.rideOn(l, r), nil); !n.walkBeats(run.Arrive-run.Depart, sk.walk, len(run.Rides), sk.tickets, walkSecs) {
				runs = append(runs, run)
				break
			}
		}
	}
	// The last to leave; of the lines leaving within the margin of it, the first to arrive.
	last := int64(math.MinInt64)
	for _, r := range runs {
		last = max(last, r.Depart)
	}
	var best Run
	ok := false
	for _, r := range runs {
		if r.Depart >= last-int64(n.opt.TransferSlack.Seconds()) && (!ok || r.Arrive < best.Arrive) {
			best, ok = r, true
		}
	}
	return best, ok
}

// ride takes `first` on the first ride and the soonest bus on every ride after it.
func (n *Network) ride(sk skeleton, first Ride, live LiveSource) (Run, bool) {
	run := Run{Depart: first.Depart - sk.access, Rides: []Ride{first}}
	t := first.Arrive
	for _, s := range sk.steps[sk.first+1:] {
		if s.kind != byRide {
			t += s.arrive - s.depart
			continue
		}
		next, ok := n.catchAny(sk.lines[len(run.Rides)], t+int64(n.opt.TransferSlack.Seconds()), live)
		if !ok {
			return Run{}, false
		}
		run.Rides = append(run.Rides, next)
		t = next.Arrive
	}
	run.Arrive = t
	return run, true
}

// catchAny is the first to arrive of the lines' buses from `ready` on, a live one over a timetabled one due the same time.
func (n *Network) catchAny(lines []line, ready int64, live LiveSource) (Ride, bool) {
	best, ok := Ride{}, false
	for _, l := range lines {
		if r, found := n.catch(l, ready, live); found && (!ok || r.Arrive < best.Arrive || (r.Arrive == best.Arrive && r.Live && !best.Live)) {
			best, ok = r, true
		}
	}
	return best, ok
}

// catch is the first bus of a line at its boarding stop from ready on: what the stop page lists, then the timetable.
func (n *Network) catch(l line, ready int64, live LiveSource) (Ride, bool) {
	p := &n.forward.patterns[l.pattern]
	board, alight := p.times[l.board], p.times[l.alight]
	listed := int64(math.MinInt64)
	if live != nil {
		for _, a := range live(p.key, n.stops[p.stops[l.board]].ID) {
			listed = max(listed, a.At)
			if a.At < ready {
				continue
			}
			r := nearestRun(board, a.At)
			ride := n.rideOn(l, r)
			ride.Depart, ride.Arrive, ride.Live, ride.VehicleID = a.At, a.At+alight[r]-board[r], a.Live, a.VehicleID
			return ride, true
		}
	}
	from := max(ready, listed+1)
	r := sort.Search(len(board), func(i int) bool { return board[i] >= from })
	if r == len(board) {
		return Ride{}, false
	}
	return n.rideOn(l, r), true
}

func (n *Network) rideOn(l line, r int) Ride {
	p := &n.forward.patterns[l.pattern]
	return Ride{RouteID: p.routeID, Key: p.key, FromStop: n.stops[p.stops[l.board]].ID, ToStop: n.stops[p.stops[l.alight]].ID, Depart: p.times[l.board][r], Arrive: p.times[l.alight][r]}
}

func nearestRun(col []int64, at int64) int {
	i := sort.Search(len(col), func(i int) bool { return col[i] >= at })
	if i == len(col) || (i > 0 && at-col[i-1] < col[i]-at) {
		return i - 1
	}
	return i
}

func when(o Option, q Query) int64 {
	switch {
	case q.Last:
		return o.Runs[0].Arrive - o.Runs[0].Depart
	case q.ArriveBy:
		return -o.Runs[0].Depart
	}
	return o.Runs[0].Arrive
}

func (n *Network) score(o Option, q Query) (t, effort int64) {
	t = when(o, q) + int64(max(o.Rides-1, 0))*int64(n.opt.TransferSlack.Seconds()) + int64(o.Tickets)*int64(n.opt.TicketCost.Seconds())
	return t, t + n.walkSecs(o.WalkMeters)
}

// Merges options on the same lines, then drops any beaten on effort, rides and tickets by more than runs stray.
// Among the day's last buses, leaving later counts too.
func (n *Network) worthTaking(options []Option, walk Option, q Query) []Option {
	slack := int64(n.opt.TransferSlack.Seconds())
	ticket := int64(n.opt.TicketCost.Seconds())
	score := func(o Option) (int64, int64) { return n.score(o, q) }
	journey := make([]int, len(options))
	var root func(i int) int
	root = func(i int) int {
		if journey[i] != i {
			journey[i] = root(journey[i])
		}
		return journey[i]
	}
	for i := range options {
		journey[i] = i
		for j := range i {
			if sameLines(options[i], options[j]) || adds(options[i], options[j]) || adds(options[j], options[i]) {
				journey[root(j)] = root(i)
			}
		}
	}
	cheapest := make(map[int]int)
	for i, o := range options {
		r := root(i)
		c, seen := cheapest[r]
		if !seen {
			cheapest[r] = i
			continue
		}
		ot, ow := score(o)
		ct, cw := score(options[c])
		if ow < cw || (ow == cw && ot < ct) {
			cheapest[r] = i
		}
	}
	var kept []Option
	for i, o := range options {
		if cheapest[root(i)] == i {
			kept = append(kept, o)
		}
	}
	beats := func(b, a Option, tie bool) bool {
		at, aw := score(a)
		bt, bw := score(b)
		noWorse := bt <= at+slack && bw <= aw && b.Rides <= a.Rides && b.Tickets <= a.Tickets
		better := b.Rides < a.Rides || b.Tickets < a.Tickets || bw < aw || bt < at
		if q.Last {
			noWorse = noWorse && b.Runs[0].Depart >= a.Runs[0].Depart-slack
			better = better || b.Runs[0].Depart > a.Runs[0].Depart
		}
		return noWorse && (better || tie)
	}
	// Among the day's last buses, walking is measured against each option on its own, from when it leaves.
	walkBeats := func(a Option) bool {
		if q.Last {
			r := a.Runs[0]
			return n.walkBeats(r.Arrive-r.Depart, a.WalkMeters, a.Rides, a.Tickets, walk.Legs[0].Secs)
		}
		return beats(walk, a, false)
	}
	var worth []Option
	for _, a := range kept {
		if !walkBeats(a) {
			worth = append(worth, a)
		}
	}
	var out []Option
	for i, a := range worth {
		beaten := false
		for j, b := range worth {
			if !beaten && i != j {
				beaten = beats(b, a, j < i)
			}
		}
		if !beaten {
			out = append(out, a)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if q.Last {
			return out[i].Runs[0].Depart > out[j].Runs[0].Depart
		}
		return when(out[i], q)+int64(out[i].Tickets)*ticket < when(out[j], q)+int64(out[j].Tickets)*ticket
	})
	return out
}

// sameLines: as many rides, and every ride shares a line with the other option's.
func sameLines(a, b Option) bool {
	if a.Rides != b.Rides {
		return false
	}
	for leg := range a.Rides {
		if !sharesLine(a, leg, b, leg) {
			return false
		}
	}
	return true
}

// adds: a rides b's lines in b's order, with more rides of its own around them.
func adds(a, b Option) bool {
	if a.Rides <= b.Rides {
		return false
	}
	j := 0
	for i := 0; i < a.Rides && j < b.Rides; i++ {
		if sharesLine(a, i, b, j) {
			j++
		}
	}
	return j == b.Rides
}

func sharesLine(a Option, i int, b Option, j int) bool {
	for _, ra := range a.Runs {
		for _, rb := range b.Runs {
			if ra.Rides[i].RouteID == rb.Rides[j].RouteID {
				return true
			}
		}
	}
	return false
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

// InService is false inside the day's longest gap between departures.
func (n *Network) InService(at int64) bool {
	prev, next := int64(math.MinInt64), int64(math.MaxInt64)
	for _, p := range n.forward.patterns {
		for _, t := range p.times[0] {
			if t <= at && t > prev {
				prev = t
			}
			if t >= at && t < next {
				next = t
			}
		}
	}
	switch {
	case next == math.MaxInt64:
		return false
	case prev == math.MinInt64 || prev == next:
		return true
	}
	return n.QuietEnd(prev) != next
}

// When service resumes after the day's longest gap; arriving by it means the last bus.
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
