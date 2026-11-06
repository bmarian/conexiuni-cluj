package traffic

import "testing"

func TestTypicalIsSampleWeighted(t *testing.T) {
	ss := newSecStats()
	ss.add(cell{"weekday", 5}, 50, 2)
	ss.add(cell{"weekday", 8}, 70, 5)
	ss.add(cell{"weekday", 17}, 150, 1)
	if got := ss.typical(); got != 70 {
		t.Fatalf("typical %v, want 70", got)
	}
}

func TestFreeFlowIsTheFastestHourOutsideTheQuietOnes(t *testing.T) {
	pairs := map[pair]*secStats{}
	for i := range 6 {
		ss := newSecStats()
		ss.add(cell{"weekday", 4}, 50, 5)
		ss.add(cell{"weekday", 10}, 70, 5)
		ss.add(cell{"weekday", 12}, 75, 5)
		ss.add(cell{"weekday", 17}, 110, 5)
		pairs[pair{i, i + 1}] = ss
	}
	free, quiet, rush := freeFlow(pairs)
	if !quiet[cell{"weekday", 4}] || quiet[cell{"weekday", 10}] {
		t.Fatalf("quiet cells %v, want only 04:00", quiet)
	}
	if !rush[cell{"weekday", 17}] || rush[cell{"weekday", 12}] {
		t.Fatalf("rush cells %v, want only 17:00", rush)
	}
	if got := free[pair{0, 1}]; got != 70 {
		t.Fatalf("free flow %v, want the 10:00 time 70", got)
	}
}

func TestReadingPoolsNeighboursAndNeedsItsOwnBus(t *testing.T) {
	line := []*measuredStretch{{pair: pair{1, 2}}, {pair: pair{2, 3}}, {pair: pair{3, 4}}, {pair: pair{4, 5}}}
	m := &measured{free: map[pair]float64{{1, 2}: 60, {2, 3}: 60, {3, 4}: 60, {4, 5}: 60}}
	runs := map[pair][]float64{{1, 2}: {60}, {2, 3}: {180}, {3, 4}: {66}}
	if r, ok := m.reading(line, 1, runs); !ok || r != 1.1 {
		t.Fatalf("reading %v %v, want the median 1.1 of the stretch and its neighbours", r, ok)
	}
	if _, ok := m.reading(line, 3, runs); ok {
		t.Fatal("a stretch no bus drove was rated from its neighbour")
	}
}

func TestSplitsKeepTheirOrder(t *testing.T) {
	rush := make([]float64, 0, 100)
	for i := 1; i <= 100; i++ {
		rush = append(rush, 1+float64(i)/100)
	}
	if got := splits([]float64{1.05, 1.08, 1.12}, rush); got[0] != 112 || got[1] != 175 || got[2] != 190 {
		t.Fatalf("splits %v, want [112 175 190]", got)
	}
	if got := splits([]float64{1.9}, rush); got[0] != 190 || got[1] != 190 {
		t.Fatalf("splits %v, want non-decreasing", got)
	}
	if got := splits(nil, rush); got != nil {
		t.Fatalf("splits %v without quiet readings, want none", got)
	}
}
