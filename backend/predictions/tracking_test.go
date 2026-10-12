package predictions

import (
	"math"
	"reflect"
	"testing"
	"time"
)

// Route 7: direction 0 runs north along a 1.1 km line, direction 1 runs it back and ends
// where direction 0 starts. 12:00, departures of direction 0 every 15 minutes.
var (
	nowMs7      = time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC).UnixMilli() // 12:00 in Cluj
	nowMinutes7 = 12 * 60.0
)

func line(reverse bool) *ShapeIndex {
	pts := make([]Point, 100)
	for i := range pts {
		k := i
		if reverse {
			k = 99 - i
		}
		pts[i] = Point{Lat: 46.77 + float64(k)*0.0001, Lon: 23.6}
	}
	return NewShapeIndex(pts)
}

var forward, backward = line(false), line(true)

func stopAt(shape *ShapeIndex, id, idx int, offset float64) Stop {
	return Stop{ID: id, Lat: shape.Points[idx].Lat, Lon: shape.Points[idx].Lon, OffsetSec: offset}
}

func dir0(through *Turnaround) *Trip {
	return NewTrip("7_0", forward, []Stop{stopAt(forward, 1, 0, 45), stopAt(forward, 2, 40, 120), stopAt(forward, 3, 90, 180)}, through)
}

func dir1() *Trip {
	return NewTrip("7_1", backward, []Stop{stopAt(backward, 4, 0, 0), stopAt(backward, 5, 50, 100), stopAt(backward, 6, 99, 100)}, nil)
}

func vehicle(id int, tripID string, shape *ShapeIndex, idx int) RawVehicle {
	return RawVehicle{ID: id, RouteID: 7, TripID: tripID, Lat: shape.Points[idx].Lat, Lon: shape.Points[idx].Lon, TimestampMs: nowMs7 - 10_000, PollIntervalMs: 15_000}
}

func arrived(v RawVehicle, secondsAgo int64) RawVehicle {
	v.PrevTripID = "7_1"
	v.TripChangedAtMs = nowMs7 - secondsAgo*1000
	return v
}

func through(departures []int, feeder ...RawVehicle) *ThroughRun {
	t := dir1()
	return &ThroughRun{
		Turnaround: Turnaround{Through: true, StandSec: 90}, Departures: departures, NowMinutes: nowMinutes7,
		TimetableLayoverMin: 5, Feeder: &Feeder{Trip: t, Vehicles: IndexVehicles(t, feeder, nowMs7)},
	}
}

func etasAt(stopPos int, own []RawVehicle, tr *ThroughRun) []int {
	trip := dir0(&Turnaround{Through: true, StandSec: 90})
	var out []int
	for _, e := range LiveEtasForStop(trip, stopPos, IndexVehicles(trip, own, nowMs7), nowMs7, tr) {
		out = append(out, int(math.Round(e.Seconds)))
	}
	return out
}

func TestProjectDeparture(t *testing.T) {
	now := float64(nowMs7)
	opts := func(d ...int) *ThroughRun {
		return &ThroughRun{Turnaround: Turnaround{Through: true, StandSec: 90}, Departures: d, NowMinutes: nowMinutes7}
	}
	check := func(name string, got float64, ok bool, want float64) {
		t.Helper()
		if !ok || math.Abs(got-want) > 1 {
			t.Fatalf("%s: got %v (%v), want %v", name, got-now, ok, want-now)
		}
	}
	d, ok := ProjectDeparture(now-30_000, opts(705, 720, 735), nowMs7, false, 0, false)
	check("slot passed", d, ok, now+60_000)
	d, ok = ProjectDeparture(now-30_000, opts(725, 740), nowMs7, false, 0, false)
	check("waits a little", d, ok, now+5*60_000)
	d, ok = ProjectDeparture(now-30_000, opts(732, 745), nowMs7, false, 0, false)
	check("too far to wait", d, ok, now+60_000)
	long := opts(732, 745)
	long.TimetableLayoverMin = 9
	d, ok = ProjectDeparture(now-30_000, long, nowMs7, false, 0, false)
	check("waits where the timetable plans a long stand", d, ok, now+12*60_000)
	learned := opts(732, 745)
	learned.Turnaround.MaxWaitMin = 8
	d, ok = ProjectDeparture(now-30_000, learned, nowMs7, false, 0, false)
	check("nobody waits that long here", d, ok, now+60_000)
	d, ok = ProjectDeparture(now-10*60_000, opts(705, 720, 735), nowMs7, true, 0, false)
	check("outstayed, next slot now", d, ok, now)
	d, ok = ProjectDeparture(now-10*60_000, opts(705, 724), nowMs7, true, 0, false)
	check("outstayed, next slot later", d, ok, now+4*60_000)
	if _, ok := ProjectDeparture(now-10*60_000, opts(705, 800), nowMs7, true, 0, false); ok {
		t.Fatal("no slot in sight should give up")
	}
	late := &ThroughRun{Turnaround: Turnaround{Through: true, StandSec: 90}, Departures: []int{2}, NowMinutes: 23*60 + 58}
	d, ok = ProjectDeparture(now-30_000, late, nowMs7, false, 0, false)
	check("past midnight", d, ok, now+4*60_000)
}

func TestStandingBusIsAboutToLeave(t *testing.T) {
	own := []RawVehicle{arrived(vehicle(1, "7_0", forward, 0), 30)}
	if got := etasAt(0, own, through([]int{705, 720, 735})); !reflect.DeepEqual(got, []int{60}) {
		t.Fatalf("first stop = %v", got)
	}
	if got := etasAt(2, own, through([]int{705, 720, 735})); !reflect.DeepEqual(got, []int{360}) {
		t.Fatalf("last stop = %v", got)
	}
	tr := through([]int{705, 720, 735})
	tr.Turnaround.Through = false
	if got := etasAt(2, own, tr); len(got) != 0 {
		t.Fatalf("not through = %v", got)
	}
	other := vehicle(1, "7_0", forward, 0)
	other.PrevTripID, other.TripChangedAtMs = "8_1", nowMs7-30_000
	if got := etasAt(2, []RawVehicle{other}, through([]int{705, 720, 735})); len(got) != 0 {
		t.Fatalf("other route = %v", got)
	}
	if got := etasAt(2, []RawVehicle{arrived(vehicle(1, "7_0", forward, 0), 26*60)}, through([]int{705, 720, 735})); len(got) != 0 {
		t.Fatalf("old flip = %v", got)
	}
}

func TestFeederBusIsListedAfterItsRideInAndTheStand(t *testing.T) {
	feeder := vehicle(2, "7_1", backward, 50)
	if got := etasAt(0, nil, through([]int{705, 720, 735}, feeder)); !reflect.DeepEqual(got, []int{180}) {
		t.Fatalf("first stop = %v", got)
	}
	if got := etasAt(2, nil, through([]int{705, 720, 735}, feeder)); !reflect.DeepEqual(got, []int{480}) {
		t.Fatalf("last stop = %v", got)
	}
	got := etasAt(2, []RawVehicle{vehicle(1, "7_0", forward, 50)}, through([]int{705, 720, 735}, feeder))
	if len(got) != 2 || got[0] >= got[1] || got[1] != 480 {
		t.Fatalf("own then feeder = %v", got)
	}
	if got := etasAt(2, nil, through([]int{705, 720, 735}, vehicle(2, "7_1", backward, 0))); len(got) != 0 {
		t.Fatalf("waiting at its own terminus = %v", got)
	}
}

func TestFeederBusIsTiedToItsRun(t *testing.T) {
	feeder := vehicle(2, "7_1", backward, 50)
	withSchedule := func(own, feederDeps []int) *ThroughRun {
		tr := through(own, feeder)
		tr.FeederDepartures = feederDeps
		return tr
	}
	if got := etasAt(0, nil, withSchedule([]int{705, 732}, []int{700, 715, 730})); !reflect.DeepEqual(got, []int{720}) {
		t.Fatalf("waits for its paired slot = %v", got)
	}
	if got := etasAt(0, nil, through([]int{705, 735}, feeder)); !reflect.DeepEqual(got, []int{180}) {
		t.Fatalf("unpaired, nearest too far = %v", got)
	}
	if got := etasAt(0, nil, withSchedule([]int{705, 743}, []int{700, 715, 730})); !reflect.DeepEqual(got, []int{180}) {
		t.Fatalf("paired slot far beyond the cap = %v", got)
	}
	if got := etasAt(0, nil, withSchedule([]int{705, 718, 731}, []int{700, 715, 730})); !reflect.DeepEqual(got, []int{180}) {
		t.Fatalf("paired slot passed = %v", got)
	}
	if got := etasAt(0, nil, withSchedule([]int{705, 725, 745}, []int{702, 722, 742})); !reflect.DeepEqual(got, []int{300}) {
		t.Fatalf("between two runs = %v", got)
	}
	if got := etasAt(0, nil, withSchedule([]int{705, 733}, []int{700, 715, 730})); len(got) != 0 {
		t.Fatalf("beyond the horizon = %v", got)
	}
}

func TestHeadingFollowsTheShape(t *testing.T) {
	trip := dir0(nil)
	v := IndexVehicles(trip, []RawVehicle{vehicle(1, "7_0", forward, 10)}, nowMs7)
	if len(v) != 1 || math.Abs(v[0].Heading) > 1 {
		t.Fatalf("northbound heading = %+v", v)
	}
	back := IndexVehicles(dir1(), []RawVehicle{vehicle(2, "7_1", backward, 10)}, nowMs7)
	if len(back) != 1 || math.Abs(back[0].Heading-180) > 1 {
		t.Fatalf("southbound heading = %+v", back)
	}
}
