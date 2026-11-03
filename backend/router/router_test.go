package router

import (
	"testing"
	"time"
)

// A straight street with four stops about 2.3 km apart, too far to walk between.
func testNetwork(t *testing.T, slack time.Duration) (*Network, time.Time) {
	t.Helper()
	streets, day := testStreets(slack)
	at := func(h, m int) int64 { return day.Add(time.Duration(h)*time.Hour + time.Duration(m)*time.Minute).Unix() }
	return streets.Network([]PatternInput{
		{RouteID: 1, Key: "1_0", Stops: []int{1, 2, 3, 4}, Trips: [][]int64{
			{at(8, 0), at(8, 10), at(8, 20), at(8, 30)},
			{at(8, 30), at(8, 40), at(8, 50), at(9, 0)},
		}},
		{RouteID: 2, Key: "2_0", Stops: []int{1, 2}, Trips: [][]int64{{at(7, 55), at(8, 0)}}},
		{RouteID: 3, Key: "3_0", Stops: []int{2, 4}, Trips: [][]int64{{at(8, 2), at(8, 12)}}},
	}), day
}

func testStreets(slack time.Duration, stops ...Stop) (*Streets, time.Time) {
	var lat, lon []float64
	var edges [][2]int32
	for i := 0; i <= 100; i++ {
		lat, lon = append(lat, 46.77), append(lon, 23.50+float64(i)*0.001)
		if i > 0 {
			edges = append(edges, [2]int32{int32(i - 1), int32(i)})
		}
	}
	g := buildWalkGraph(lat, lon, edges)
	g.box = BBox{46.76, 23.49, 46.78, 23.61}
	if len(stops) == 0 {
		stops = []Stop{{1, 46.77, 23.50}, {2, 46.77, 23.53}, {3, 46.77, 23.56}, {4, 46.77, 23.59}}
	}
	streets := NewStreets(g, stops, Options{WalkSpeed: 80.0 / 60, MaxWalk: 15 * time.Minute, MaxTransfer: 30 * time.Minute, TransferSlack: slack, TicketCost: 15 * time.Minute})
	return streets, time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)
}

// A city line and a line on a ticket of its own, both from stop 1 to stop 4.
func ticketNetwork(ownTicketArrives int) (*Network, time.Time) {
	streets, day := testStreets(time.Minute)
	at := func(h, m int) int64 { return day.Add(time.Duration(h)*time.Hour + time.Duration(m)*time.Minute).Unix() }
	return streets.Network([]PatternInput{
		{RouteID: 1, Key: "city", Stops: []int{1, 2, 3, 4}, Trips: [][]int64{{at(8, 0), at(8, 10), at(8, 20), at(8, 30)}}},
		{RouteID: 2, Key: "metro", Stops: []int{1, 4}, Trips: [][]int64{{at(8, 0), at(8, ownTicketArrives)}}, OwnTicket: true},
	}), day
}

func TestATicketSavingMoreThanItIsWorthComesFirst(t *testing.T) {
	n, day := ticketNetwork(10)
	its := transit(n.Plan(Query{From: Point{46.7701, 23.5002}, To: Point{46.7701, 23.5898}, Time: day.Add(7*time.Hour + 50*time.Minute)}))
	if got := keys(its); len(got) != 2 || got[0] != "metro " || got[1] != "city " {
		t.Fatalf("got %q", got)
	}
	if its[0].Tickets != 1 || its[1].Tickets != 0 {
		t.Fatalf("tickets %d, %d", its[0].Tickets, its[1].Tickets)
	}
}

func TestATicketSavingLessThanItIsWorthGoes(t *testing.T) {
	n, day := ticketNetwork(20)
	got := keys(n.Plan(Query{From: Point{46.7701, 23.5002}, To: Point{46.7701, 23.5898}, Time: day.Add(7*time.Hour + 50*time.Minute)}))
	if len(got) != 1 || got[0] != "city " {
		t.Fatalf("got %q", got)
	}
}

func TestASecondTicketThatSavesTimeComesSecond(t *testing.T) {
	n, day := ticketNetwork(20)
	got := keys(n.Plan(Query{From: Point{46.7701, 23.5002}, To: Point{46.7701, 23.5898}, Time: day.Add(7*time.Hour + 50*time.Minute), Runs: 3}))
	if len(got) != 2 || got[0] != "city " || got[1] != "metro " {
		t.Fatalf("got %q", got)
	}
}

func TestASecondTicketThatSavesNoTimeGoes(t *testing.T) {
	n, day := ticketNetwork(40)
	got := keys(n.Plan(Query{From: Point{46.7701, 23.5002}, To: Point{46.7701, 23.5898}, Time: day.Add(7*time.Hour + 50*time.Minute), Runs: 3}))
	if len(got) != 1 || got[0] != "city " {
		t.Fatalf("got %q", got)
	}
}

// Line 1 runs every ten minutes from stop 1 to stop 4; the other way changes at stop 2 and reaches it at the given minute past eight.
func secondWayNetwork(arrives int) (*Network, time.Time) {
	streets, day := testStreets(time.Minute)
	at := func(h, m int) int64 { return day.Add(time.Duration(h)*time.Hour + time.Duration(m)*time.Minute).Unix() }
	return streets.Network([]PatternInput{
		{RouteID: 1, Key: "direct", Stops: []int{1, 4}, Trips: [][]int64{{at(8, 0), at(8, 20)}, {at(8, 10), at(8, 30)}, {at(8, 20), at(8, 40)}}},
		{RouteID: 2, Key: "feeder", Stops: []int{1, 2}, Trips: [][]int64{{at(8, 0), at(8, 5)}}},
		{RouteID: 3, Key: "main", Stops: []int{2, 4}, Trips: [][]int64{{at(8, 8), at(8, arrives)}}},
	}), day
}

func TestABeatenWayOnOtherLinesComesSecond(t *testing.T) {
	n, day := secondWayNetwork(30)
	got := keys(n.Plan(Query{From: Point{46.7701, 23.5002}, To: Point{46.7701, 23.5898}, Time: day.Add(7*time.Hour + 58*time.Minute), Runs: 3}))
	if len(got) != 2 || got[0] != "direct " || got[1] != "feeder main " {
		t.Fatalf("got %q", got)
	}
}

func TestArrivingByKeepsASecondWay(t *testing.T) {
	n, day := secondWayNetwork(30)
	got := keys(n.Plan(Query{From: Point{46.7701, 23.5002}, To: Point{46.7701, 23.5898}, Time: day.Add(8*time.Hour + 35*time.Minute), ArriveBy: true, Runs: 3}))
	if len(got) != 2 || got[0] != "direct " || got[1] != "feeder main " {
		t.Fatalf("got %q", got)
	}
}

func TestASecondWaySlowerThanWaitingForTheBusGoes(t *testing.T) {
	n, day := secondWayNetwork(55)
	got := keys(n.Plan(Query{From: Point{46.7701, 23.5002}, To: Point{46.7701, 23.5898}, Time: day.Add(7*time.Hour + 58*time.Minute), Runs: 3}))
	if len(got) != 1 || got[0] != "direct " {
		t.Fatalf("got %q", got)
	}
}

func TestASecondWayCanKeepOneRide(t *testing.T) {
	streets, day := testStreets(time.Minute)
	at := func(h, m int) int64 { return day.Add(time.Duration(h)*time.Hour + time.Duration(m)*time.Minute).Unix() }
	n := streets.Network([]PatternInput{
		{RouteID: 1, Key: "first", Stops: []int{1, 2}, Trips: [][]int64{{at(8, 0), at(8, 5)}}},
		{RouteID: 2, Key: "fast", Stops: []int{2, 4}, Trips: [][]int64{{at(8, 8), at(8, 20)}}},
		{RouteID: 3, Key: "slow", Stops: []int{2, 3}, Trips: [][]int64{{at(8, 9), at(8, 15)}}},
		{RouteID: 4, Key: "on", Stops: []int{3, 4}, Trips: [][]int64{{at(8, 17), at(8, 24)}}},
	})
	got := keys(n.Plan(Query{From: Point{46.7701, 23.5002}, To: Point{46.7701, 23.5898}, Time: day.Add(7*time.Hour + 58*time.Minute), Runs: 1}))
	if len(got) != 1 || got[0] != "first fast " {
		t.Fatalf("slower than the only listed bus: got %q", got)
	}
	got = keys(n.Plan(Query{From: Point{46.7701, 23.5002}, To: Point{46.7701, 23.5898}, Time: day.Add(7*time.Hour + 58*time.Minute), Runs: 3}))
	if len(got) != 2 || got[0] != "first fast " || got[1] != "first slow on " {
		t.Fatalf("got %q", got)
	}
}

func TestDroppingARideIsNotASecondWay(t *testing.T) {
	n, day := feederNetwork(23.506)
	got := keys(n.Plan(Query{From: Point{46.7701, 23.5002}, To: Point{46.7701, 23.5798}, Time: day.Add(7*time.Hour + 55*time.Minute), Runs: 3}))
	if len(got) != 1 || got[0] != "feeder main " {
		t.Fatalf("got %q", got)
	}
}

// The second stop is a walk away, or one stop on the feeder; changes cost three minutes.
func feederNetwork(secondStopLon float64) (*Network, time.Time) {
	streets, day := testStreets(3*time.Minute, Stop{1, 46.77, 23.500}, Stop{2, 46.77, secondStopLon}, Stop{3, 46.77, 23.580})
	at := func(h, m int) int64 { return day.Add(time.Duration(h)*time.Hour + time.Duration(m)*time.Minute).Unix() }
	return streets.Network([]PatternInput{
		{RouteID: 1, Key: "feeder", Stops: []int{1, 2}, Trips: [][]int64{{at(8, 0), at(8, 2)}}},
		{RouteID: 2, Key: "main", Stops: []int{2, 3}, Trips: [][]int64{{at(8, 6), at(8, 16)}}},
	}), day
}

func TestAddingARideIsTheSameOptionKeptWhereItCostsLeast(t *testing.T) {
	for _, c := range []struct {
		lon  float64
		want string
	}{{23.506, "feeder main "}, {23.502, "main "}} {
		n, day := feederNetwork(c.lon)
		got := keys(n.Plan(Query{From: Point{46.7701, 23.5002}, To: Point{46.7701, 23.5798}, Time: day.Add(7*time.Hour + 55*time.Minute)}))
		if len(got) != 1 || got[0] != c.want {
			t.Fatalf("second stop at %v: got %q", c.lon, got)
		}
	}
}

func TestWalkingBeatsALateBus(t *testing.T) {
	streets, day := testStreets(time.Minute)
	at := func(h, m int) int64 { return day.Add(time.Duration(h)*time.Hour + time.Duration(m)*time.Minute).Unix() }
	// Walking to stop 2 takes about half an hour; the bus there is an hour and a half away.
	n := streets.Network([]PatternInput{{RouteID: 1, Key: "late", Stops: []int{1, 2}, Trips: [][]int64{{at(9, 30), at(9, 40)}}}})
	if its := transit(n.Plan(Query{From: Point{46.7701, 23.5002}, To: Point{46.7701, 23.5298}, Time: day.Add(8 * time.Hour)})); len(its) != 0 {
		t.Fatalf("got %q", keys(its))
	}
}

func transit(options []Option) []Option {
	var out []Option
	for _, o := range options {
		if o.Rides > 0 {
			out = append(out, o)
		}
	}
	return out
}

func keys(options []Option) []string {
	var out []string
	for _, o := range transit(options) {
		k := ""
		for _, r := range o.Runs[0].Rides {
			k += r.Key + " "
		}
		out = append(out, k)
	}
	return out
}

func TestTransferAndDirectAreBothKept(t *testing.T) {
	n, day := testNetwork(t, time.Minute)
	its := transit(n.Plan(Query{From: Point{46.7701, 23.5002}, To: Point{46.7701, 23.5898}, Time: day.Add(7*time.Hour + 40*time.Minute)}))
	got := keys(its)
	if len(got) != 2 || got[0] != "2_0 3_0 " || got[1] != "1_0 " {
		t.Fatalf("got %q", got)
	}
	if arrive := its[0].Runs[0].Rides[1].Arrive; arrive != day.Add(8*time.Hour+12*time.Minute).Unix() {
		t.Fatalf("transfer ride arrives %d", arrive)
	}
	if its[0].Legs[0].Ride || its[0].Legs[len(its[0].Legs)-1].Ride || len(its[0].Legs[0].Path) < 2 {
		t.Fatalf("walks missing: %+v", its[0].Legs)
	}
}

func TestSlackRulesOutTightTransfer(t *testing.T) {
	n, day := testNetwork(t, 3*time.Minute)
	got := keys(n.Plan(Query{From: Point{46.7701, 23.5002}, To: Point{46.7701, 23.5898}, Time: day.Add(7*time.Hour + 40*time.Minute)}))
	if len(got) != 1 || got[0] != "1_0 " {
		t.Fatalf("got %q", got)
	}
}

func TestArriveByFindsLatestDeparture(t *testing.T) {
	n, day := testNetwork(t, time.Minute)
	its := transit(n.Plan(Query{From: Point{46.7701, 23.5002}, To: Point{46.7701, 23.5898}, Time: day.Add(8*time.Hour + 20*time.Minute), ArriveBy: true}))
	got := keys(its)
	if len(got) != 1 || got[0] != "2_0 3_0 " {
		t.Fatalf("got %q", got)
	}
	if ride := its[0].Runs[0].Rides[0]; ride.FromStop != 1 || ride.ToStop != 2 || ride.Depart != day.Add(7*time.Hour+55*time.Minute).Unix() {
		t.Fatalf("first ride %+v", ride)
	}
	its = transit(n.Plan(Query{From: Point{46.7701, 23.5002}, To: Point{46.7701, 23.5898}, Time: day.Add(9*time.Hour + 30*time.Minute), ArriveBy: true}))
	if got := keys(its); len(got) != 1 || got[0] != "1_0 " || its[0].Runs[0].Rides[0].Depart != day.Add(8*time.Hour+30*time.Minute).Unix() {
		t.Fatalf("got %q %+v", got, its)
	}
}

func TestPassedStops(t *testing.T) {
	n, day := testNetwork(t, time.Minute)
	its := transit(n.Plan(Query{From: Point{46.7701, 23.5002}, To: Point{46.7701, 23.5898}, Time: day.Add(8*time.Hour + 5*time.Minute)}))
	if len(its) != 1 || len(its[0].Legs[1].Passed) != 2 || its[0].Legs[1].Passed[0] != 2 {
		t.Fatalf("got %+v", its)
	}
}

func TestLessWalkingKeepsASlowerOption(t *testing.T) {
	var lat, lon []float64
	var edges [][2]int32
	for i := 0; i <= 100; i++ {
		lat, lon = append(lat, 46.77), append(lon, 23.50+float64(i)*0.001)
		if i > 0 {
			edges = append(edges, [2]int32{int32(i - 1), int32(i)})
		}
	}
	g := buildWalkGraph(lat, lon, edges)
	g.box = BBox{46.76, 23.49, 46.78, 23.61}
	stops := []Stop{{1, 46.77, 23.50}, {5, 46.77, 23.51}, {4, 46.77, 23.59}}
	streets := NewStreets(g, stops, Options{WalkSpeed: 80.0 / 60, MaxWalk: 15 * time.Minute, MaxTransfer: 30 * time.Minute, TransferSlack: time.Minute})
	day := time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)
	at := func(h, m int) int64 { return day.Add(time.Duration(h)*time.Hour + time.Duration(m)*time.Minute).Unix() }
	n := streets.Network([]PatternInput{
		{RouteID: 7, Key: "7_0", Stops: []int{5, 4}, Trips: [][]int64{{at(7, 52), at(8, 10)}}},
		{RouteID: 8, Key: "8_0", Stops: []int{1, 4}, Trips: [][]int64{{at(7, 41), at(8, 15)}}},
		{RouteID: 9, Key: "9_0", Stops: []int{1, 4}, Trips: [][]int64{{at(7, 42), at(8, 25)}}},
	})
	// 9_0 walks as little as 8_0 and arrives later; 7_0 arrives first but walks 10 min more than 8_0, which is 5 min later.
	got := keys(n.Plan(Query{From: Point{46.7701, 23.5002}, To: Point{46.7701, 23.5898}, Time: day.Add(7*time.Hour + 40*time.Minute)}))
	if len(got) != 2 || got[0] != "7_0 " || got[1] != "8_0 " {
		t.Fatalf("got %q", got)
	}
}

func TestRunsListNextDepartures(t *testing.T) {
	n, day := testNetwork(t, time.Minute)
	its := transit(n.Plan(Query{From: Point{46.7701, 23.5002}, To: Point{46.7701, 23.5898}, Time: day.Add(7*time.Hour + 58*time.Minute), Runs: 3}))
	if len(its) != 1 || len(its[0].Runs) != 2 || its[0].Runs[1].Rides[0].Depart != day.Add(8*time.Hour+30*time.Minute).Unix() {
		t.Fatalf("got %+v", its)
	}
}

func TestLiveBusIsCaught(t *testing.T) {
	n, day := testNetwork(t, time.Minute)
	late := day.Add(8*time.Hour + 4*time.Minute).Unix()
	live := func(key string, stop int) []Arrival {
		if key == "1_0" && stop == 1 {
			return []Arrival{{At: late, Live: true, VehicleID: 42}}
		}
		return nil
	}
	its := transit(n.Plan(Query{From: Point{46.7701, 23.5002}, To: Point{46.7701, 23.5898}, Time: day.Add(8*time.Hour + 1*time.Minute), Live: live, Runs: 2}))
	if len(its) != 1 {
		t.Fatalf("got %+v", its)
	}
	first := its[0].Runs[0].Rides[0]
	if !first.Live || first.VehicleID != 42 || first.Depart != late || first.Arrive != late+30*60 {
		t.Fatalf("first %+v", first)
	}
	if next := its[0].Runs[1].Rides[0]; next.Live || next.Depart != day.Add(8*time.Hour+30*time.Minute).Unix() {
		t.Fatalf("next %+v", next)
	}
}

func TestLaterRunsStopOnceWalkingIsFaster(t *testing.T) {
	n, day := testNetwork(t, time.Minute)
	at := func(h, m int) int64 { return day.Add(time.Duration(h)*time.Hour + time.Duration(m)*time.Minute).Unix() }
	n = n.Streets.Network([]PatternInput{{RouteID: 1, Key: "1_0", Stops: []int{1, 2, 3, 4}, Trips: [][]int64{
		{at(8, 0), at(8, 10), at(8, 20), at(8, 30)},
		{at(8, 30), at(8, 40), at(8, 50), at(9, 0)},
		{at(10, 0), at(10, 10), at(10, 20), at(10, 30)},
	}}})
	// Walking the 6.9 km takes until about 09:24.
	its := transit(n.Plan(Query{From: Point{46.7701, 23.5002}, To: Point{46.7701, 23.5898}, Time: day.Add(7*time.Hour + 58*time.Minute), Runs: 3}))
	if len(its) != 1 || len(its[0].Runs) != 2 {
		t.Fatalf("got %+v", its)
	}
}

func TestQuietEndIsWhenServiceResumes(t *testing.T) {
	n, day := testNetwork(t, time.Minute)
	// Runs leave at 07:55, 08:00, 08:02 and 08:30 only: quiet from 08:30 until the day looked at ends.
	if end := n.QuietEnd(day.Add(7 * time.Hour).Unix()); end != day.Add(31*time.Hour).Unix() {
		t.Fatalf("got %s", time.Unix(end, 0).UTC())
	}
}

func TestInServiceIsFalseInTheLongestGap(t *testing.T) {
	n, day := testNetwork(t, time.Minute)
	if !n.InService(day.Add(8*time.Hour + 10*time.Minute).Unix()) {
		t.Error("between runs counts as quiet")
	}
	if n.InService(day.Add(12 * time.Hour).Unix()) {
		t.Error("after the last run counts as running")
	}
}

func TestSameLinesAreOneOption(t *testing.T) {
	n, day := testNetwork(t, time.Minute)
	// Halfway between stops 3 and 4: line 1 could drop you at either.
	its := transit(n.Plan(Query{From: Point{46.7701, 23.5002}, To: Point{46.7701, 23.575}, Time: day.Add(7*time.Hour + 58*time.Minute)}))
	if len(its) != 1 || its[0].Runs[0].Rides[0].ToStop != 3 {
		t.Fatalf("got %+v", its)
	}
}

func TestCoversOnlyTheWalkGraphArea(t *testing.T) {
	streets, _ := testStreets(time.Minute)
	if !streets.Covers(Point{46.77, 23.55}) {
		t.Error("a point on the street is not covered")
	}
	for _, p := range []Point{{0, 0}, {44.43, 26.10}, {46.77, 23.62}} {
		if streets.Covers(p) {
			t.Errorf("%v outside the walk graph is covered", p)
		}
	}
}

func TestTheLastBusCountsEvenWhenWalkingWouldLeaveLater(t *testing.T) {
	n, day := testNetwork(t, time.Minute)
	end := n.QuietEnd(day.Add(12 * time.Hour).Unix())
	options := n.Plan(Query{From: Point{46.7701, 23.5002}, To: Point{46.7701, 23.5898}, Time: time.Unix(end, 0), ArriveBy: true, Last: true})
	its := transit(options)
	if len(its) == 0 || len(its) != len(options) {
		t.Fatalf("got %q", keys(options))
	}
	if dep := its[0].Runs[0].Rides[0].Depart; dep != day.Add(8*time.Hour+30*time.Minute).Unix() {
		t.Fatalf("last bus leaves at %s", time.Unix(dep, 0).UTC())
	}
}

func TestArriveByTakesTheFirstConnection(t *testing.T) {
	streets, day := testStreets(time.Minute)
	at := func(h, m int) int64 { return day.Add(time.Duration(h)*time.Hour + time.Duration(m)*time.Minute).Unix() }
	n := streets.Network([]PatternInput{
		{RouteID: 1, Key: "a", Stops: []int{1, 2}, Trips: [][]int64{{at(8, 0), at(8, 10)}}},
		{RouteID: 2, Key: "b", Stops: []int{2, 4}, Trips: [][]int64{{at(8, 15), at(8, 25)}, {at(9, 0), at(9, 10)}}},
	})
	its := transit(n.Plan(Query{From: Point{46.7701, 23.5002}, To: Point{46.7701, 23.5898}, Time: day.Add(9*time.Hour + 30*time.Minute), ArriveBy: true}))
	if len(its) != 1 || len(its[0].Runs[0].Rides) != 2 {
		t.Fatalf("got %+v", its)
	}
	if second := its[0].Runs[0].Rides[1]; second.Depart != at(8, 15) {
		t.Fatalf("waits for the %s bus", time.Unix(second.Depart, 0).UTC())
	}
}

func TestTheLastBusGoesWhenWalkingIsAsFast(t *testing.T) {
	streets, day := testStreets(time.Minute)
	at := func(h, m int) int64 { return day.Add(time.Duration(h)*time.Hour + time.Duration(m)*time.Minute).Unix() }
	// Stop 1 to stop 2 is a 28-minute walk; the bus takes an hour.
	n := streets.Network([]PatternInput{{RouteID: 1, Key: "1_0", Stops: []int{1, 2}, Trips: [][]int64{{at(8, 0), at(9, 0)}}}})
	end := n.QuietEnd(day.Add(12 * time.Hour).Unix())
	options := n.Plan(Query{From: Point{46.7701, 23.5002}, To: Point{46.7701, 23.5298}, Time: time.Unix(end, 0), ArriveBy: true, Last: true})
	if len(options) != 1 || options[0].Rides != 0 {
		t.Fatalf("got %q", keys(options))
	}
}

func TestEarlierLastBusesLeaveEarlier(t *testing.T) {
	streets, day := testStreets(time.Minute)
	at := func(h, m int) int64 { return day.Add(time.Duration(h)*time.Hour + time.Duration(m)*time.Minute).Unix() }
	n := streets.Network([]PatternInput{
		{RouteID: 1, Key: "a", Stops: []int{1, 2}, Trips: [][]int64{{at(8, 0), at(8, 10)}, {at(8, 30), at(8, 40)}}},
		{RouteID: 2, Key: "b", Stops: []int{2, 4}, Trips: [][]int64{{at(8, 15), at(8, 25)}, {at(8, 45), at(8, 55)}, {at(9, 0), at(9, 10)}, {at(9, 15), at(9, 25)}}},
	})
	end := n.QuietEnd(day.Add(12 * time.Hour).Unix())
	its := transit(n.Plan(Query{From: Point{46.7701, 23.5002}, To: Point{46.7701, 23.5898}, Time: time.Unix(end, 0), ArriveBy: true, Last: true, Runs: 3}))
	if len(its) != 1 || len(its[0].Runs) != 2 {
		t.Fatalf("got %+v", its)
	}
	first, second := its[0].Runs[0], its[0].Runs[1]
	if first.Rides[0].Depart != at(8, 30) || first.Rides[1].Depart != at(8, 45) || second.Rides[0].Depart != at(8, 0) || second.Rides[1].Depart != at(8, 15) {
		t.Fatalf("got %+v", its[0].Runs)
	}
}

func TestABusWalkingBeatsDoesNotHideTheLastBus(t *testing.T) {
	streets, day := testStreets(time.Minute)
	at := func(h, m int) int64 { return day.Add(time.Duration(h)*time.Hour + time.Duration(m)*time.Minute).Unix() }
	// Both lines go to stop 2, a 28-minute walk away. Line 1 takes 10 minutes at 20:00. Line 2 leaves
	// later, at 23:00, and crawls there in an hour: walking beats it.
	n := streets.Network([]PatternInput{
		{RouteID: 1, Key: "1_0", Stops: []int{1, 2}, Trips: [][]int64{{at(20, 0), at(20, 10)}}},
		{RouteID: 2, Key: "2_0", Stops: []int{1, 2}, Trips: [][]int64{{at(23, 0), at(24, 0)}}},
	})
	end := n.QuietEnd(day.Add(12 * time.Hour).Unix())
	its := transit(n.Plan(Query{From: Point{46.7701, 23.5002}, To: Point{46.7701, 23.5298}, Time: time.Unix(end, 0), ArriveBy: true, Last: true}))
	if got := keys(its); len(got) != 1 || got[0] != "1_0 " {
		t.Fatalf("got %q", got)
	}
}

func TestTheLastBusAndAFasterEarlierOneBothStay(t *testing.T) {
	streets, day := testStreets(time.Minute, Stop{1, 46.77, 23.50}, Stop{4, 46.77, 23.59}, Stop{5, 46.77, 23.585})
	at := func(h, m int) int64 { return day.Add(time.Duration(h)*time.Hour + time.Duration(m)*time.Minute).Unix() }
	// Line 1 does the 6.9 km in half an hour at 22:00; the night line leaves at 23:30 and takes an hour to a stop nearby.
	n := streets.Network([]PatternInput{
		{RouteID: 1, Key: "1_0", Stops: []int{1, 4}, Trips: [][]int64{{at(22, 0), at(22, 30)}}},
		{RouteID: 2, Key: "n_0", Stops: []int{1, 5}, Trips: [][]int64{{at(23, 30), at(24, 30)}}},
	})
	end := n.QuietEnd(day.Add(12 * time.Hour).Unix())
	got := keys(n.Plan(Query{From: Point{46.7701, 23.5002}, To: Point{46.7701, 23.5898}, Time: time.Unix(end, 0), ArriveBy: true, Last: true}))
	if len(got) != 2 || got[0] != "n_0 " || got[1] != "1_0 " {
		t.Fatalf("got %q", got)
	}
}

func TestOfLinesLeavingTogetherTheFasterIsTheLastBus(t *testing.T) {
	streets, day := testStreets(time.Minute)
	at := func(h, m int) int64 { return day.Add(time.Duration(h)*time.Hour + time.Duration(m)*time.Minute).Unix() }
	n := streets.Network([]PatternInput{
		{RouteID: 1, Key: "fast", Stops: []int{1, 4}, Trips: [][]int64{{at(22, 0), at(22, 30)}}},
		{RouteID: 2, Key: "slow", Stops: []int{1, 4}, Trips: [][]int64{{at(22, 0) + 30, at(23, 10)}}},
	})
	end := n.QuietEnd(day.Add(12 * time.Hour).Unix())
	its := transit(n.Plan(Query{From: Point{46.7701, 23.5002}, To: Point{46.7701, 23.5898}, Time: time.Unix(end, 0), ArriveBy: true, Last: true}))
	if got := keys(its); len(got) != 1 || got[0] != "fast " {
		t.Fatalf("got %q", got)
	}
}
