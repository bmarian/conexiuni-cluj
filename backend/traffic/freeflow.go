package traffic

import (
	"conexiuni-cluj/live"
	"math"
	"sort"
)

type secStats struct {
	sum     map[cell]float64
	samples map[cell]int
}

func newSecStats() *secStats {
	return &secStats{sum: map[cell]float64{}, samples: map[cell]int{}}
}

func (ss *secStats) add(k cell, sec float64, samples int) {
	ss.sum[k] += sec * float64(samples)
	ss.samples[k] += samples
}

func (ss *secStats) usual(k cell) float64 { return ss.sum[k] / float64(ss.samples[k]) }

func (ss *secStats) typical() float64 {
	type cellSec struct {
		sec     float64
		samples int
	}
	cells := make([]cellSec, 0, len(ss.sum))
	total := 0
	for k := range ss.sum {
		cells = append(cells, cellSec{ss.usual(k), ss.samples[k]})
		total += ss.samples[k]
	}
	sort.Slice(cells, func(i, j int) bool { return cells[i].sec < cells[j].sec })
	seen := 0
	for _, c := range cells {
		seen += c.samples
		if seen*2 >= total {
			return c.sec
		}
	}
	return 0
}

// Quiet hours run nearer the city's fastest than its usual, rush hours nearer its slowest.
func freeFlow(pairs map[pair]*secStats) (free map[pair]float64, quiet, rush map[cell]bool) {
	ratios := make(map[cell][]float64)
	for _, ss := range pairs {
		typical := ss.typical()
		if typical <= 0 {
			continue
		}
		for k := range ss.sum {
			ratios[k] = append(ratios[k], ss.usual(k)/typical)
		}
	}
	curve := make(map[cell]float64)
	fastest, slowest := math.Inf(1), math.Inf(-1)
	for k, r := range ratios {
		if len(r) < live.MinProfileSamples {
			continue
		}
		curve[k] = live.Median(r)
		fastest, slowest = min(fastest, curve[k]), max(slowest, curve[k])
	}
	quiet, rush = make(map[cell]bool), make(map[cell]bool)
	for k, m := range curve {
		if m <= (fastest+1)/2 {
			quiet[k] = true
		}
		if m >= (slowest+1)/2 {
			rush[k] = true
		}
	}
	free = make(map[pair]float64)
	for p, ss := range pairs {
		for k := range ss.sum {
			if quiet[k] {
				continue
			}
			if sec := ss.usual(k); free[p] == 0 || sec < free[p] {
				free[p] = sec
			}
		}
	}
	return free, quiet, rush
}
