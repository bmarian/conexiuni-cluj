package traffic

import "testing"

func TestNeighbouringSlowStretchesFormOneJam(t *testing.T) {
	var line []*measuredStretch
	m := &measured{free: map[pair]float64{}, lines: map[string][]*measuredStretch{}}
	for i := range 6 {
		p := pair{i, i + 1}
		line = append(line, &measuredStretch{pair: p})
		m.free[p] = 60
	}
	m.lines["1_0"] = line
	runs := map[pair][]float64{{0, 1}: {60, 60, 60}, {1, 2}: {120, 120}, {2, 3}: {120, 120}, {3, 4}: {60, 60, 60}, {4, 5}: {60}, {5, 6}: {60}}
	got := m.now(runs, []int{110, 150, 190})["1_0"]
	if len(got.Stretches) != 2 || len(got.Jams) != 1 {
		t.Fatalf("%d stretches and %d jams, want 2 slow stretches in one jam", len(got.Stretches), len(got.Jams))
	}
	if jam := got.Jams[0]; jam.FromStopID != 1 || jam.ToStopID != 3 || jam.Band != 3 || jam.DelaySec != 120 {
		t.Fatalf("jam %+v, want stops 1 to 3, band 3, 120 s", jam)
	}
	if _, ok := m.now(map[pair][]float64{{0, 1}: {60}}, []int{110, 150, 190})["1_0"]; ok {
		t.Fatal("a line with nothing slow got traffic")
	}
}
