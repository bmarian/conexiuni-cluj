package traffic

// Unrated stretches follow the slope between their rated neighbours; a terminus one, unmeasured because buses wait there, its neighbour.
func fillBetween(line []*measuredStretch, ratio map[stretchKey]float64) {
	last := -1
	for i, st := range line {
		r, ok := ratio[st.key]
		if !ok {
			continue
		}
		if last < 0 && i == 1 {
			ratio[line[0].key] = r
		}
		if last >= 0 && i-last > 1 {
			from := ratio[line[last].key]
			for j := last + 1; j < i; j++ {
				ratio[line[j].key] = from + float64(j-last)/float64(i-last)*(r-from)
			}
		}
		last = i
	}
	if n := len(line); n > 1 && last == n-2 {
		ratio[line[n-1].key] = ratio[line[last].key]
	}
}
