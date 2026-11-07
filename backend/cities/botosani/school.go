package botosani

import (
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/feeds/gtfs"
	"time"
)

// Most days of each kind in the sampled weeks decide.
func termRuns(school feeds.School, now time.Time) map[string]bool {
	type tally struct{ term, days int }
	counts := map[string]tally{}
	first := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	for d := range gtfs.SampledWeeks * 7 {
		day := first.AddDate(0, 0, d)
		c := counts[gtfs.DayKind(day)]
		c.days++
		if school.SchoolDay(day) {
			c.term++
		}
		counts[gtfs.DayKind(day)] = c
	}
	out := map[string]bool{}
	for kind, c := range counts {
		out[kind] = 2*c.term > c.days
	}
	return out
}
