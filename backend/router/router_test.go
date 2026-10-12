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

func testStreets(slack time.Duration) (*Streets, time.Time) {
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
	stops := []Stop{{1, 46.77, 23.50}, {2, 46.77, 23.53}, {3, 46.77, 23.56}, {4, 46.77, 23.59}}
	streets := NewStreets(g, stops, Options{WalkSpeed: 80.0 / 60, MaxWalk: 15 * time.Minute, MaxTransfer: 30 * time.Minute, TransferSlack: slack})
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

func TestAnotherTicketComesAfterTheCityOnes(t *testing.T) {
	n, day := ticketNetwork(20)
	its := transit(n.Plan(Query{From: Point{46.7701, 23.5002}, To: Point{46.7701, 23.5898}, Time: day.Add(7*time.Hour + 50*time.Minute)}))
	if got := keys(its); len(got) != 2 || got[0] != "city " || got[1] != "metro " {
		t.Fatalf("got %q", got)
	}
	if its[0].Tickets != 0 || its[1].Tickets != 1 {
		t.Fatalf("tickets %d, %d", its[0].Tickets, its[1].Tickets)
	}
}

func TestAnotherTicketThatIsNoFasterGoes(t *testing.T) {
	n, day := ticketNetwork(40)
	got := keys(n.Plan(Query{From: Point{46.7701, 23.5002}, To: Point{46.7701, 23.5898}, Time: day.Add(7*time.Hour + 50*time.Minute)}))
	if len(got) != 1 || got[0] != "city " {
		t.Fatalf("got %q", got)
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

func TestSameLinesAreOneOption(t *testing.T) {
	n, day := testNetwork(t, time.Minute)
	// Halfway between stops 3 and 4: line 1 could drop you at either.
	its := transit(n.Plan(Query{From: Point{46.7701, 23.5002}, To: Point{46.7701, 23.575}, Time: day.Add(7*time.Hour + 58*time.Minute)}))
	if len(its) != 1 || its[0].Runs[0].Rides[0].ToStop != 3 {
		t.Fatalf("got %+v", its)
	}
}
