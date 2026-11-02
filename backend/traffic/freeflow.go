package traffic

import (
	"conexiuni-cluj/live"
	"math"
	"sort"
)

type paceStats struct {
	// Sample-weighted seconds per metre per cell.
	pace    map[cell]float64
	samples map[cell]int
	// typical is the sample-weighted median pace over every measured hour; free is the pace at the city's quiet hours.
	typical float64
	free    float64
}

func (ps *paceStats) usual(k cell) (float64, bool) {
	sum, ok := ps.pace[k]
	if !ok {
		return 0, false
	}
	return sum / float64(ps.samples[k]), true
}

func (ps *paceStats) each(fn func(k cell, pace float64)) {
	for k := range ps.pace {
		pace, _ := ps.usual(k)
		fn(k, pace)
	}
}

func (ps *paceStats) medianPace(counts func(cell) bool) float64 {
	type cellPace struct {
		pace    float64
		samples int
	}
	cells := make([]cellPace, 0, len(ps.pace))
	total := 0
	ps.each(func(k cell, pace float64) {
		if counts(k) {
			cells = append(cells, cellPace{pace, ps.samples[k]})
			total += ps.samples[k]
		}
	})
	sort.Slice(cells, func(i, j int) bool { return cells[i].pace < cells[j].pace })
	seen := 0
	for _, c := range cells {
		seen += c.samples
		if seen*2 >= total {
			return c.pace
		}
	}
	return 0
}

// The hours of each kind of day when the city's buses run closer to their fastest than to their normal: free flow.
func quietCells(stats []*paceStats) map[cell]bool {
	ratios := make(map[cell][]float64)
	for _, ps := range stats {
		ps.each(func(k cell, pace float64) { ratios[k] = append(ratios[k], pace/ps.typical) })
	}
	curve := make(map[cell]float64)
	fastest := math.Inf(1)
	for k, r := range ratios {
		if len(r) < live.MinProfileSamples {
			continue
		}
		sort.Float64s(r)
		curve[k] = quantile(r, 0.5)
		fastest = min(fastest, curve[k])
	}
	quiet := make(map[cell]bool)
	for k, m := range curve {
		if m <= (fastest+1)/2 {
			quiet[k] = true
		}
	}
	return quiet
}

func rateFreeFlow(stats []*paceStats) (atQuiet, atBusy []float64) {
	quiet := quietCells(stats)
	var typicalOverFree []float64
	for _, ps := range stats {
		if free := ps.medianPace(func(k cell) bool { return quiet[k] }); free > 0 {
			ps.free = free
			typicalOverFree = append(typicalOverFree, ps.typical/free)
		}
	}
	// Stretches never measured at a quiet hour borrow how much slower than free flow the city typically runs.
	borrow := 1.0
	if len(typicalOverFree) > 0 {
		sort.Float64s(typicalOverFree)
		borrow = quantile(typicalOverFree, 0.5)
	}
	for _, ps := range stats {
		if ps.free == 0 {
			ps.free = ps.typical / borrow
		}
		ps.each(func(k cell, pace float64) {
			if quiet[k] {
				atQuiet = append(atQuiet, pace/ps.free)
			} else {
				atBusy = append(atBusy, pace/ps.free)
			}
		})
	}
	return atQuiet, atBusy
}

// Busy, slow and jammed, in percent of free flow: the spread of the quiet hours, then the slowest quarter and tenth of the busy ones.
func splits(atQuiet, atBusy []float64) []int {
	if len(atBusy) == 0 {
		return []int{}
	}
	quiet := append([]float64(nil), atQuiet...)
	busy := append([]float64(nil), atBusy...)
	sort.Float64s(quiet)
	sort.Float64s(busy)
	free := quantile(busy, 0.5)
	if len(quiet) > 0 {
		free = quantile(quiet, 0.9)
	}
	out := []int{percent(free), percent(quantile(busy, 0.75)), percent(quantile(busy, 0.9))}
	for i := 1; i < len(out); i++ {
		out[i] = max(out[i], out[i-1])
	}
	return out
}

func quantile(sorted []float64, p float64) float64 {
	i := min(int(math.Ceil(p*float64(len(sorted))))-1, len(sorted)-1)
	return sorted[max(i, 0)]
}
