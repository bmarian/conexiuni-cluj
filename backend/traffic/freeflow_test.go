package traffic

import "testing"

func stats(cells map[cell][2]float64) *paceStats {
	ps := &paceStats{pace: map[cell]float64{}, samples: map[cell]int{}}
	for k, c := range cells {
		ps.pace[k] = c[0] * c[1]
		ps.samples[k] = int(c[1])
	}
	ps.typical = ps.medianPace(func(cell) bool { return true })
	return ps
}

func TestMedianPaceIsSampleWeighted(t *testing.T) {
	ps := stats(map[cell][2]float64{{"weekday", 5}: {0.10, 2}, {"weekday", 8}: {0.14, 5}, {"weekday", 17}: {0.30, 1}})
	if ps.typical != 0.14 {
		t.Fatalf("typical pace %v, want 0.14", ps.typical)
	}
	if got := ps.medianPace(func(k cell) bool { return k.hour == 5 }); got != 0.10 {
		t.Fatalf("quiet pace %v, want 0.10", got)
	}
}

func TestFreeFlowComesFromTheQuietHoursOrIsBorrowed(t *testing.T) {
	var all []*paceStats
	for range 6 {
		all = append(all, stats(map[cell][2]float64{
			{"weekday", 5}: {0.10, 5}, {"weekday", 8}: {0.15, 5}, {"weekday", 12}: {0.13, 5}, {"weekday", 17}: {0.16, 5},
		}))
	}
	daytimeOnly := stats(map[cell][2]float64{{"weekday", 8}: {0.30, 5}, {"weekday", 12}: {0.26, 5}})
	all = append(all, daytimeOnly)
	atQuiet, atBusy := rateFreeFlow(all)
	if got := all[0].free; got != 0.10 {
		t.Fatalf("free flow %v, want the 05:00 pace 0.10", got)
	}
	if got, want := daytimeOnly.free, 0.26/(0.13/0.10); got < want-1e-9 || got > want+1e-9 {
		t.Fatalf("borrowed free flow %v, want %v", got, want)
	}
	if len(atQuiet) != 6 || len(atBusy) != 20 {
		t.Fatalf("quiet %d busy %d slowdowns, want 6 and 20", len(atQuiet), len(atBusy))
	}
}

func TestSplitsKeepTheirOrder(t *testing.T) {
	busy := make([]float64, 0, 100)
	for i := 1; i <= 100; i++ {
		busy = append(busy, 1+float64(i)/100)
	}
	if got := splits([]float64{1.05, 1.08, 1.12}, busy); got[0] != 112 || got[1] != 175 || got[2] != 190 {
		t.Fatalf("splits %v, want [112 175 190]", got)
	}
	if got := splits([]float64{1.9}, busy); got[0] != 190 || got[1] != 190 {
		t.Fatalf("splits %v, want non-decreasing", got)
	}
}
