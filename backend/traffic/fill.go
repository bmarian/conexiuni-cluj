package traffic

// fillBetween rates a line's unrated stretches along the slope between the rated ones either side, and an unrated
// terminus stretch like its neighbour: a bus waiting at the terminus is what keeps that one from being measured.
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
