package slobozia

import (
	"conexiuni-cluj/calendar"
	"conexiuni-cluj/feeds/sitegtfs"
	"strings"
)

// SPTL's kinds of day as the calendar's; the site runs its weekend times on public holidays too.
var kinds = map[string][]string{weekdays: {calendar.Weekday}, weekends: {calendar.Saturday, calendar.Sunday}}

// Each route is a way of its line, a "…_RETUR" the way back; a loop is split where it waits into a way out and a way back, as the site labels them.
func lines(stops map[string]sitegtfs.Stop, routes []route) []sitegtfs.Line {
	var out []sitegtfs.Line
	for _, r := range routes {
		var ways [2]*sitegtfs.Route
		switch {
		case r.dwell > 0:
			ways = r.halves(stops)
		case strings.HasSuffix(r.id, backSuffix):
			ways[1] = way(r.id, r.stops, r.minutes, r.leaves, stops)
		default:
			ways[0] = way(r.id, r.stops, r.minutes, r.leaves, stops)
		}
		out = append(out, sitegtfs.Line{Name: r.line, Ways: ways})
	}
	return out
}

// The way out ends where the loop waits; the way back leaves it when the rows say and keeps the bundle's minutes between its stops, its first leg taking what the rows' median ride back leaves over.
func (r route) halves(stops map[string]sitegtfs.Stop) [2]*sitegtfs.Route {
	var out, back []int
	if r.minutes != nil {
		out = r.minutes[:r.dwell+1]
		next, last := r.minutes[r.dwell+1], r.minutes[len(r.minutes)-1]
		leg := max(0, r.rideBack-(last-next))
		back = []int{0}
		for _, m := range r.minutes[r.dwell+1:] {
			back = append(back, m-next+leg)
		}
	}
	return [2]*sitegtfs.Route{
		way(r.id, r.stops[:r.dwell+1], out, r.leaves, stops),
		way(strings.TrimSuffix(r.id, "_TUR")+backSuffix, r.stops[r.dwell:], back, r.turns, stops),
	}
}

func way(id string, keys []string, minutes []int, runs map[string][]int, stops map[string]sitegtfs.Stop) *sitegtfs.Route {
	w := &sitegtfs.Route{ID: id, Minutes: minutes}
	for _, key := range keys {
		s, ok := stops[key]
		if !ok {
			s = sitegtfs.Stop{ID: key, Name: key}
		}
		w.Stops = append(w.Stops, s)
	}
	for _, day := range []string{weekdays, weekends} {
		for _, kind := range kinds[day] {
			for _, minute := range runs[day] {
				w.Departures = append(w.Departures, sitegtfs.Departure{Day: kind, Minute: minute})
			}
		}
	}
	return w
}
