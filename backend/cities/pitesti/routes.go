package pitesti

import (
	"conexiuni-cluj/calendar"
	"conexiuni-cluj/feeds/sitegtfs"
	"conexiuni-cluj/router"
	"fmt"
	"math"
)

// network writes each page as a line; a way's runs that take different times between stops become routes of their own, so the reader learns each hour's times.
func network(pages []linePage) ([]sitegtfs.Line, map[string][]router.Point) {
	var lines []sitegtfs.Line
	var drawn []sitegtfs.Route
	alike := map[string][]string{}
	for _, p := range pages {
		var dirs [2][]*sitegtfs.Route
		for dir, w := range p.ways {
			dirs[dir] = w.routes(fmt.Sprintf("%s-%d", p.id, dir))
			if len(dirs[dir]) == 0 {
				continue
			}
			first := dirs[dir][0]
			drawn = append(drawn, *first)
			for _, r := range dirs[dir] {
				alike[first.ID] = append(alike[first.ID], r.ID)
			}
		}
		lines = append(lines, pair(p.name, dirs)...)
	}
	shapes := map[string][]router.Point{}
	for id, path := range sitegtfs.Draw(drawn) {
		for _, r := range alike[id] {
			shapes[r] = path
		}
	}
	return lines, shapes
}

// One route per way and set of times between its stops, in the order its runs first leave.
func (w way) routes(id string) []*sitegtfs.Route {
	var out []*sitegtfs.Route
	byTimes := map[string]*sitegtfs.Route{}
	for _, day := range calendar.ScheduleDays {
		for _, run := range w.runs[day] {
			minutes := reach(w.stops, run)
			key := fmt.Sprint(minutes)
			r := byTimes[key]
			if r == nil {
				r = &sitegtfs.Route{ID: fmt.Sprintf("%s-%d", id, len(out)), Stops: w.stops, Minutes: minutes}
				byTimes[key] = r
				out = append(out, r)
			}
			r.Departures = append(r.Departures, sitegtfs.Departure{Day: day, Minute: run[0]})
		}
	}
	return out
}

// Minutes from the first stop to each; the page times no arrival at the way out's end, so the run reaches it at its own pace from its last placed stop, by straight distance.
func reach(stops []sitegtfs.Stop, run []int) []int {
	out := make([]int, len(stops))
	for i, at := range run {
		out[i] = at - run[0]
	}
	first, last, metres := -1, -1, 0.0
	for i := range run {
		if !stops[i].Placed {
			continue
		}
		if last >= 0 {
			metres += stops[last].At.Meters(stops[i].At)
		} else {
			first = i
		}
		last = i
	}
	for i := len(run); i < len(stops); i++ {
		out[i] = out[i-1]
		if stops[i].Placed && metres > 0 {
			pace := float64(out[last]-out[first]) / metres
			out[i] = max(out[i], out[last]+int(math.Round(stops[last].At.Meters(stops[i].At)*pace)))
		}
	}
	return out
}

// Ways with several routes become several lines of the same name.
func pair(name string, dirs [2][]*sitegtfs.Route) []sitegtfs.Line {
	out := make([]sitegtfs.Line, max(len(dirs[0]), len(dirs[1])))
	for i := range out {
		out[i].Name = name
		for dir := range dirs {
			if i < len(dirs[dir]) {
				out[i].Ways[dir] = dirs[dir][i]
			}
		}
	}
	return out
}
