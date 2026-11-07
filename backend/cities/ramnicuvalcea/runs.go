package ramnicuvalcea

import (
	"conexiuni-cluj/feeds/sitegtfs"
	"log"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
)

const dayMinutes = 24 * 60

// ETA tells a road's two sides apart by a closing "Tur" or "Retur", as in "Gara Tur" and "Str. Republicii Bl.R24.Retur".
var sideMark = regexp.MustCompile(`(?i)[\s.]*\b(?:re)?tur\s*$`)

// The name the app shows, so both sides of a road make one station.
func stationName(name string) string {
	if bare := strings.TrimSpace(sideMark.ReplaceAllString(name, "")); bare != "" {
		return bare
	}
	return strings.TrimSpace(name)
}

// A run as ETA's trip id names it: the minutes it passes each stop.
type run struct {
	trip  string
	line  string
	way   int
	days  []string
	only  sitegtfs.Limit
	at    map[string][]int
	stops []sitegtfs.Stop
	times []int
}

// lines makes the app's lines from the runs the stop pages show: runs of a line that stop at the same stops the same minutes apart are one route.
func lines(index []link, pages map[string]ways, passings []passing) []sitegtfs.Line {
	runs := map[string]*run{}
	for _, p := range passings {
		r := runs[p.trip]
		if r == nil {
			r = &run{trip: p.trip, line: p.line, way: p.way, only: p.only, at: map[string][]int{}}
			runs[p.trip] = r
		}
		for _, d := range p.days {
			if !slices.Contains(r.days, d) {
				r.days = append(r.days, d)
			}
		}
		r.at[p.stop] = append(r.at[p.stop], p.minute)
	}
	byLine := map[string][]*run{}
	var starts []int
	unknown := 0
	for _, r := range runs {
		page, ok := pages[r.line]
		if !ok {
			unknown++
			continue
		}
		r.follow(page[r.way])
		if len(r.stops) < 2 {
			continue
		}
		byLine[r.line] = append(byLine[r.line], r)
		starts = append(starts, r.times[0])
	}
	if unknown > 0 {
		log.Printf("ramnicuvalcea: %d runs of lines without a page are left out", unknown)
	}
	first := serviceStart(starts)
	var out []sitegtfs.Line
	for _, l := range index {
		rs := byLine[l.name]
		for _, r := range rs {
			if r.times[0] < first {
				for i := range r.times {
					r.times[i] += dayMinutes
				}
			}
		}
		sort.Slice(rs, func(i, j int) bool {
			if rs[i].way != rs[j].way {
				return rs[i].way < rs[j].way
			}
			if rs[i].times[0] != rs[j].times[0] {
				return rs[i].times[0] < rs[j].times[0]
			}
			return rs[i].trip < rs[j].trip
		})
		routes := map[string]*sitegtfs.Route{}
		for _, r := range rs {
			key := r.key()
			route := routes[key]
			if route == nil {
				route = &sitegtfs.Route{ID: l.name + "-" + strconv.Itoa(len(routes)), Stops: r.stops}
				for _, t := range r.times {
					route.Minutes = append(route.Minutes, t-r.times[0])
				}
				routes[key] = route
				var w [2]*sitegtfs.Route
				w[r.way] = route
				out = append(out, sitegtfs.Line{Name: l.name, Ways: w})
			}
			for _, d := range r.days {
				route.Departures = append(route.Departures, sitegtfs.Departure{Day: d, Minute: r.times[0], Only: r.only})
			}
		}
	}
	return out
}

// The run's stops in the order its line's page lists them; a run never goes back in time, so a time earlier than the stop before is past midnight.
func (r *run) follow(stops []sitegtfs.Stop) {
	used := map[string]int{}
	for _, s := range stops {
		times := r.at[s.ID]
		k := used[s.ID]
		if k >= len(times) {
			continue
		}
		used[s.ID]++
		sort.Ints(times)
		t := times[k]
		for len(r.times) > 0 && t < r.times[len(r.times)-1] {
			t += dayMinutes
		}
		r.stops, r.times = append(r.stops, s), append(r.times, t)
	}
}

func (r *run) key() string {
	parts := []string{strconv.Itoa(r.way)}
	for i, s := range r.stops {
		parts = append(parts, s.ID+"@"+strconv.Itoa(r.times[i]-r.times[0]))
	}
	return strings.Join(parts, " ")
}

// The pages show times of day, so a run leaving after midnight reads like a morning one; the service day starts after the longest pause between departures.
func serviceStart(starts []int) int {
	sort.Ints(starts)
	start, longest := 0, -1
	for i, m := range starts {
		if pause := (m - starts[(i+len(starts)-1)%len(starts)] + dayMinutes) % dayMinutes; pause > longest {
			start, longest = m, pause
		}
	}
	return start
}
