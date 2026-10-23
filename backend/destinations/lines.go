package destinations

import (
	"conexiuni-cluj/models"
	"sort"
	"strconv"
	"strings"
)

// Digit runs compare as numbers, as the clients sort lines.
func lineLess(a, b string) bool {
	for a != "" && b != "" {
		ca, ra := chunk(a)
		cb, rb := chunk(b)
		if ca != cb {
			na, errA := strconv.Atoi(ca)
			nb, errB := strconv.Atoi(cb)
			switch {
			case errA == nil && errB == nil && na != nb:
				return na < nb
			case (errA == nil) != (errB == nil):
				return errA == nil
			}
			return ca < cb
		}
		a, b = ra, rb
	}
	return len(a) < len(b)
}

func chunk(s string) (string, string) {
	digit := func(c byte) bool { return c >= '0' && c <= '9' }
	end := 1
	for end < len(s) && digit(s[end]) == digit(s[0]) {
		end++
	}
	return s[:end], s[end:]
}

func (x *Index) sortRoutes(ids []int) {
	sort.Slice(ids, func(i, j int) bool {
		return lineLess(x.routes[ids[i]].RouteShortName, x.routes[ids[j]].RouteShortName)
	})
}

func (x *Index) Lines() []*models.NetworkRoute {
	ids := make([]int, 0, len(x.routes))
	for id := range x.routes {
		ids = append(ids, id)
	}
	x.sortRoutes(ids)
	out := make([]*models.NetworkRoute, len(ids))
	for i, id := range ids {
		out[i] = x.routes[id]
	}
	return out
}

func (x *Index) RouteStops(routeID, dir int) []*models.Stop {
	p := x.byRoute[[2]int{routeID, dir}]
	if p == nil {
		return nil
	}
	out := make([]*models.Stop, 0, len(p.stops))
	for _, id := range p.stops {
		if s := x.stops[id]; s != nil {
			out = append(out, s)
		}
	}
	return out
}

func (x *Index) Toward(routeID, dir int) string {
	route := x.routes[routeID]
	if route == nil {
		return ""
	}
	name := route.RouteLongName
	if route.Timetable != nil && route.Timetable.RouteLongName != "" {
		name = route.Timetable.RouteLongName
	}
	parts := strings.Split(name, " - ")
	if dir == 1 {
		return parts[0]
	}
	return parts[len(parts)-1]
}

// Same grouping as stationDestinations in frontend/src/utils/destinations.ts.
func (x *Index) StationLines(stopID int) []Departing {
	stop := x.stops[stopID]
	if stop == nil {
		return nil
	}
	station := stationOf(stop)
	var out []Departing
	at := map[string]int{}
	leaving := map[int]bool{}
	var ending []Line
	for _, p := range x.patterns {
		for k, id := range p.stops {
			s := x.stops[id]
			if s == nil || stationOf(s) != station {
				continue
			}
			line := Line{p.route.RouteID, p.dir}
			if k == len(p.stops)-1 {
				ending = append(ending, line)
				continue
			}
			leaving[line.RouteID] = true
			toward := x.Toward(line.RouteID, line.Direction)
			i, ok := at[toward]
			if !ok {
				i = len(out)
				at[toward] = i
				out = append(out, Departing{Toward: toward})
			}
			out[i].Lines = appendLine(out[i].Lines, line)
			break
		}
	}
	var ends []Line
	for _, line := range ending {
		if !leaving[line.RouteID] {
			ends = appendLine(ends, line)
		}
	}
	if len(ends) > 0 {
		out = append(out, Departing{Lines: ends})
	}
	for i := range out {
		lines := out[i].Lines
		sort.Slice(lines, func(a, b int) bool {
			return lineLess(x.routes[lines[a].RouteID].RouteShortName, x.routes[lines[b].RouteID].RouteShortName)
		})
	}
	return out
}

func appendLine(lines []Line, line Line) []Line {
	for _, l := range lines {
		if l.RouteID == line.RouteID {
			return lines
		}
	}
	return append(lines, line)
}

func stationOf(s *models.Stop) int {
	if s.StationID != 0 {
		return s.StationID
	}
	return s.StopID
}
