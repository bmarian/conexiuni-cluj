package traffic

import (
	"math"
	"testing"
)

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestFillBetweenInterpolatesOnlyBetweenMeasuredStretches(t *testing.T) {
	var line []*measuredStretch
	for i := range 6 {
		line = append(line, &measuredStretch{key: stretchKey{1, 0, i, i + 1}})
	}
	ratio := map[stretchKey]float64{line[1].key: 1.0, line[4].key: 1.6}
	fillBetween(line, ratio)
	if ratio[line[0].key] != 1.0 || ratio[line[5].key] != 1.6 {
		t.Fatal("the terminus stretches did not take their neighbour's slowdown")
	}
	if !near(ratio[line[2].key], 1.2) || !near(ratio[line[3].key], 1.4) {
		t.Fatalf("stretches 2 and 3 rated %v and %v, want 1.2 and 1.4", ratio[line[2].key], ratio[line[3].key])
	}
	one := map[stretchKey]float64{line[3].key: 1.3}
	fillBetween(line, one)
	if len(one) != 1 {
		t.Fatalf("%d stretches rated from one measured in the middle, want it alone", len(one))
	}
}
