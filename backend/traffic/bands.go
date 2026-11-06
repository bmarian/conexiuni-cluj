package traffic

import (
	"conexiuni-cluj/live"
	"sort"
	"time"
)

type lineRef struct {
	line []*measuredStretch
	i    int
}

func (s *Service) learnBands(m *measured) ([]int, error) {
	started := time.Now()
	at := make(map[pair][]lineRef)
	for _, line := range m.lines {
		for i, st := range line {
			at[st.pair] = append(at[st.pair], lineRef{line, i})
		}
	}
	window := int64(LiveWindow / time.Second)
	var atQuiet, atRush []float64
	runs := make(map[pair][]float64)
	var current int64
	var k cell
	flush := func() {
		if !m.quiet[k] && !m.rush[k] {
			return
		}
		for p := range runs {
			for _, ref := range at[p] {
				r, ok := m.reading(ref.line, ref.i, runs)
				if !ok {
					continue
				}
				if m.quiet[k] {
					atQuiet = append(atQuiet, r)
				} else {
					atRush = append(atRush, r)
				}
			}
		}
	}
	err := s.timing.EachStretchRun(time.Unix(0, 0), func(r live.StretchRun) {
		if w := r.ObservedAt / window; w != current {
			flush()
			clear(runs)
			current, k = w, cell{r.DayType, r.Hour}
		}
		p := pair{r.FromStopID, r.ToStopID}
		runs[p] = append(runs[p], r.DurationSec)
	})
	if err != nil {
		return nil, err
	}
	flush()
	bands := splits(atQuiet, atRush)
	s.store.Logf("traffic: bands %v from %d quiet and %d rush readings, in %s", bands, len(atQuiet), len(atRush), time.Since(started).Round(time.Millisecond))
	return bands, nil
}

// Percent of free flow: the quiet readings' p90, then the rush readings' p75 and p90.
func splits(atQuiet, atRush []float64) []int {
	if len(atQuiet) == 0 || len(atRush) == 0 {
		return nil
	}
	sort.Float64s(atQuiet)
	sort.Float64s(atRush)
	out := []int{percent(live.NearestRank(atQuiet, 0.9)), percent(live.NearestRank(atRush, 0.75)), percent(live.NearestRank(atRush, 0.9))}
	for i := 1; i < len(out); i++ {
		out[i] = max(out[i], out[i-1])
	}
	return out
}
