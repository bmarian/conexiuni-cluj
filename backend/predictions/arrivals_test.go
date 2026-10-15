package predictions

import (
	"math"
	"reflect"
	"testing"
)

// Route 45 at 14:42, every 25 min, the bus ~3 min ahead between Trifoiului Sud and Pinului.
const now45 = 14*60 + 42

var departures45 = func() []int {
	out := make([]int, 12)
	for i := range out {
		out[i] = 12*60 + 2 + i*25
	}
	return out
}()

func at45(offset int) []float64 {
	return ScheduledArrivals(departures45, float64(offset*60), now45*60, 480*60, 3*60)
}

func shown(seconds float64) int { return int(math.Round(seconds / 60)) }

func labels(arrivals []Arrival) []string {
	out := make([]string, len(arrivals))
	for i, a := range arrivals {
		out[i] = itoa(shown(a.Seconds))
		if a.Live {
			out[i] += "L"
		}
	}
	return out
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var b []byte
	for v > 0 {
		b = append([]byte{byte('0' + v%10)}, b...)
		v /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}

func live(minutes ...int) []LiveSeconds {
	out := make([]LiveSeconds, len(minutes))
	for i, m := range minutes {
		out[i] = LiveSeconds{Seconds: float64(m * 60), VehicleID: i + 1}
	}
	return out
}

func mins(minutes ...int) []float64 {
	out := make([]float64, len(minutes))
	for i, m := range minutes {
		out[i] = float64(m * 60)
	}
	return out
}

func TestMinutesUntil(t *testing.T) {
	if MinutesUntil(now45-1, now45) != -1 || MinutesUntil(now45, now45) != 0 || MinutesUntil(now45+18, now45) != 18 {
		t.Fatal("plain differences")
	}
	if MinutesUntil(5, 23*60+50) != 15 || MinutesUntil(23*60+50, 5) != -15 {
		t.Fatal("across midnight")
	}
}

func TestPassedStopIsNotAdvertisedOnATrackedTrip(t *testing.T) {
	if got := at45(160); got[0] != 0 {
		t.Fatalf("trifoiului first = %v", got[0])
	}
	if got := labels(MergeArrivals(nil, at45(160), true, 3, 180)); !reflect.DeepEqual(got, []string{"25", "50", "75"}) {
		t.Fatalf("tracked: %v", got)
	}
	if got := labels(MergeArrivals(nil, at45(160), false, 3, 180)); !reflect.DeepEqual(got, []string{"0", "25", "50"}) {
		t.Fatalf("untracked: %v", got)
	}
}

func TestColumnReadsForwardsDownTheList(t *testing.T) {
	column := []string{
		labels(MergeArrivals(nil, at45(154), true, 3, 180))[0],
		labels(MergeArrivals(nil, at45(159), true, 3, 180))[0],
		labels(MergeArrivals(nil, at45(160), true, 3, 180))[0],
		labels(MergeArrivals(live(0), at45(162), true, 3, 180))[0],
		labels(MergeArrivals(live(2), at45(164), true, 3, 180))[0],
		labels(MergeArrivals(live(7), at45(170), true, 3, 180))[0],
	}
	if want := []string{"19", "24", "25", "0L", "2L", "7L"}; !reflect.DeepEqual(column, want) {
		t.Fatalf("column = %v", column)
	}
}

func TestLiveTakesTheSlotOfItsRun(t *testing.T) {
	if got := at45(162)[:3]; !reflect.DeepEqual(got, mins(2, 27, 52)) {
		t.Fatalf("pinului = %v", got)
	}
	if got := labels(MergeArrivals(live(0), at45(162), true, 3, 180)); !reflect.DeepEqual(got, []string{"0L", "27", "52"}) {
		t.Fatalf("merged = %v", got)
	}
	if got := labels(MergeArrivals(live(2), mins(20, 45, 70), false, 3, 180)); !reflect.DeepEqual(got, []string{"2L", "20", "45"}) {
		t.Fatalf("badly late = %v", got)
	}
	if got := labels(MergeArrivals(live(6), mins(1, 26, 51), false, 3, 180)); !reflect.DeepEqual(got, []string{"6L", "26", "51"}) {
		t.Fatalf("overtaken slot = %v", got)
	}
	if got := MergeArrivals(live(4), nil, false, 3, 180); len(got) != 1 || shown(got[0].Seconds) != 4 || !got[0].Live {
		t.Fatalf("live alone = %v", got)
	}
	if got := MergeArrivals(nil, nil, true, 3, 180); len(got) != 0 {
		t.Fatalf("nothing = %v", got)
	}
}

func TestSeveralLiveEstimates(t *testing.T) {
	if got := labels(MergeArrivals(live(18, 3), mins(4, 16, 28, 40), false, 3, 180)); !reflect.DeepEqual(got, []string{"3L", "18L", "28"}) {
		t.Fatalf("two buses = %v", got)
	}
	if got := labels(MergeArrivals(nil, mins(0, 12), true, 3, 180)); !reflect.DeepEqual(got, []string{"12"}) {
		t.Fatalf("empty tracked = %v", got)
	}
}

func TestLateRunStaysListedWithinGrace(t *testing.T) {
	late := ScheduledArrivals([]int{now45 - 2, now45 + 23}, 0, now45*60, 480*60, 180)
	if !reflect.DeepEqual(late, mins(-2, 23)) {
		t.Fatalf("late = %v", late)
	}
	if got := labels(MergeArrivals(nil, late, false, 3, 180)); !reflect.DeepEqual(got, []string{"0", "23"}) {
		t.Fatalf("merged = %v", got)
	}
	if got := ScheduledArrivals([]int{now45 - 4}, 0, now45*60, 0, 180); len(got) != 0 {
		t.Fatalf("past grace = %v", got)
	}
}

func TestRepeatedNowReadsSoon(t *testing.T) {
	// Bus 32 towards P-ța Mihai Viteazul at 09:17 with nothing tracked.
	departures := []int{9*60 + 3, 9*60 + 12, 9*60 + 22, 9*60 + 31}
	offsets := []int{0, 1, 2, 4, 7, 9, 14}
	rows := make([][]Arrival, len(offsets))
	for i, off := range offsets {
		rows[i] = MergeArrivals(nil, ScheduledArrivals(departures, float64(off*60), (9*60+17)*60, 0, 180), false, 3, 180)
	}
	marked := MarkRepeatedNow(rows)
	var first []string
	for _, row := range marked {
		if row[0].Soon {
			first = append(first, "soon")
		} else {
			first = append(first, itoa(shown(row[0].Seconds)))
		}
	}
	if want := []string{"5", "6", "0", "soon", "2", "4", "0"}; !reflect.DeepEqual(first, want) {
		t.Fatalf("first column = %v", first)
	}
	row := MarkRepeatedNow([][]Arrival{{{Seconds: 0, Live: true}, {Seconds: 20}, {Seconds: 9 * 60}}})[0]
	if row[0].Soon || !row[1].Soon || row[2].Soon {
		t.Fatalf("second zero in a row = %+v", row)
	}
}
