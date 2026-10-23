package destinations

import (
	"conexiuni-cluj/calendar"
	"conexiuni-cluj/geo"
	"conexiuni-cluj/models"
	"conexiuni-cluj/router"
	"conexiuni-cluj/transit"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

// Destination is an area of the city with the stops in it and the direct rides there from the other areas.
type Destination struct {
	Slug     string   `json:"slug"`
	Name     string   `json:"name"`
	District bool     `json:"district"`
	Lat      float64  `json:"lat"`
	Lon      float64  `json:"lon"`
	StopIDs  []int    `json:"stop_ids"`
	RouteIDs []int    `json:"route_ids"`
	From     []Origin `json:"from"`
}

type Origin struct {
	Slug  string `json:"slug"`
	Name  string `json:"name"`
	Rides []Ride `json:"rides"`
}

// Ride is one line from the last stop of an area to the first stop it reaches in another.
type Ride struct {
	RouteID   int `json:"route_id"`
	Direction int `json:"direction"`
	Board     int `json:"board"`
	Alight    int `json:"alight"`
	// Fastest and slowest over the day, learned.
	Minutes [2]int `json:"minutes"`
	// The usual gap between departures; 0 with a single one.
	EveryMin int    `json:"every_min,omitempty"`
	First    string `json:"first"`
	Last     string `json:"last"`
	// The first kind of day the line runs on, normally a weekday.
	DayType string `json:"day_type"`
}

type Summary struct {
	Slug     string `json:"slug"`
	Name     string `json:"name"`
	District bool   `json:"district"`
	Stops    int    `json:"stops"`
	Lines    int    `json:"lines"`
}

type Line struct {
	RouteID   int
	Direction int
}

// Departing is the lines leaving a station for one end; Toward is empty for the lines ending there.
type Departing struct {
	Toward string
	Lines  []Line
}

// Index is the network as the static pages read it. Its areas are empty until the place index is read.
type Index struct {
	routes   map[int]*models.NetworkRoute
	stops    map[int]*models.Stop
	patterns []*pattern
	byRoute  map[[2]int]*pattern
	calls    map[int][]*pattern
	areaOf   map[int]*Destination
	bySlug   map[string]*Destination
	list     []*Destination
}

// pattern is one direction of a line.
type pattern struct {
	route *models.NetworkRoute
	dir   int
	stops []int
	// Seconds from the timetable departure to each stop, by day type and hour.
	reach map[string][][]float64
}

func build(bundle *models.NetworkBundle, areas []router.Area) *Index {
	x := &Index{
		routes:  make(map[int]*models.NetworkRoute, len(bundle.Routes)),
		stops:   make(map[int]*models.Stop, len(bundle.Stops)),
		byRoute: make(map[[2]int]*pattern),
		calls:   make(map[int][]*pattern),
		areaOf:  make(map[int]*Destination),
		bySlug:  make(map[string]*Destination),
	}
	for i := range bundle.Stops {
		x.stops[bundle.Stops[i].StopID] = &bundle.Stops[i]
	}
	for i := range bundle.Routes {
		route := &bundle.Routes[i]
		x.routes[route.RouteID] = route
		x.addPatterns(route)
	}
	x.addAreas(areas)
	return x
}

func (x *Index) addPatterns(route *models.NetworkRoute) {
	at := 0
	for _, trip := range route.Trips {
		p := &pattern{route: route, dir: tripDirection(trip.TripID), stops: trip.StopIDs, reach: map[string][][]float64{}}
		for day, hours := range route.Offsets {
			reach := make([][]float64, len(hours))
			for h, offsets := range hours {
				cum := make([]float64, len(trip.StopIDs))
				total := 0.0
				for k := range trip.StopIDs {
					if at+k < len(offsets) {
						total += float64(offsets[at+k])
					}
					cum[k] = total
				}
				reach[h] = cum
			}
			p.reach[day] = reach
		}
		at += len(trip.StopIDs)
		x.patterns = append(x.patterns, p)
		x.byRoute[[2]int{route.RouteID, p.dir}] = p
		for _, id := range trip.StopIDs {
			if calls := x.calls[id]; len(calls) == 0 || calls[len(calls)-1] != p {
				x.calls[id] = append(calls, p)
			}
		}
	}
}

// Trip ids end in the direction: "35_0", "35_1".
func tripDirection(tripID string) int {
	if i := strings.LastIndexByte(tripID, '_'); i >= 0 {
		if d, err := strconv.Atoi(tripID[i+1:]); err == nil {
			return d
		}
	}
	return 0
}

// IsDistrict tells a part of a town from a town or village, by its OSM place.
func IsDistrict(place string) bool {
	return place == "suburb" || place == "neighbourhood" || place == "quarter"
}

// Each stop is in its nearest district when one is nearer than any town, else in that town, as place labels are.
func (x *Index) addAreas(areas []router.Area) {
	var districts, towns []router.Area
	for _, a := range areas {
		if IsDistrict(a.Place) {
			districts = append(districts, a)
		} else {
			towns = append(towns, a)
		}
	}
	byArea := map[router.Area]*Destination{}
	ids := make([]int, 0, len(x.calls))
	for id := range x.calls {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	for _, id := range ids {
		stop := x.stops[id]
		if stop == nil {
			continue
		}
		d, dm := nearest(districts, stop.StopLat, stop.StopLon)
		t, tm := nearest(towns, stop.StopLat, stop.StopLon)
		area := t
		if d != nil && (t == nil || dm < tm) {
			area = d
		}
		if area == nil {
			continue
		}
		dest := byArea[*area]
		if dest == nil {
			dest = &Destination{Name: area.Name, District: IsDistrict(area.Place), Lat: area.Lat, Lon: area.Lon}
			byArea[*area] = dest
			x.list = append(x.list, dest)
		}
		dest.StopIDs = append(dest.StopIDs, id)
		x.areaOf[id] = dest
	}
	// The area with the most stops keeps a shared name's slug; the others add the nearest town's.
	sort.SliceStable(x.list, func(i, j int) bool { return len(x.list[i].StopIDs) > len(x.list[j].StopIDs) })
	kept := x.list[:0]
	for _, dest := range x.list {
		dest.Slug = router.Slug(dest.Name)
		if x.bySlug[dest.Slug] != nil {
			others := make([]router.Area, 0, len(towns))
			for _, t := range towns {
				if t.Name != dest.Name {
					others = append(others, t)
				}
			}
			if t, _ := nearest(others, dest.Lat, dest.Lon); t != nil {
				dest.Slug += "-" + router.Slug(t.Name)
			}
		}
		if dest.Slug == "" || x.bySlug[dest.Slug] != nil {
			for _, id := range dest.StopIDs {
				delete(x.areaOf, id)
			}
			continue
		}
		x.bySlug[dest.Slug] = dest
		kept = append(kept, dest)
	}
	x.list = kept
	sizes := make(map[*Destination]int, len(x.list))
	for _, dest := range x.list {
		sizes[dest] = len(dest.StopIDs)
	}
	for _, dest := range x.list {
		x.fill(dest, sizes)
	}
	sort.Slice(x.list, func(i, j int) bool { return router.Slug(x.list[i].Name) < router.Slug(x.list[j].Name) })
}

func nearest(among []router.Area, lat, lon float64) (*router.Area, float64) {
	var best *router.Area
	bestM := 0.0
	for i := range among {
		if m := geo.Haversine(lat, lon, among[i].Lat, among[i].Lon); best == nil || m < bestM {
			best, bestM = &among[i], m
		}
	}
	return best, bestM
}

// fill adds the lines at dest and the direct rides there; origins with more stops come first.
func (x *Index) fill(dest *Destination, sizes map[*Destination]int) {
	lines := map[int]bool{}
	for _, id := range dest.StopIDs {
		for _, p := range x.calls[id] {
			lines[p.route.RouteID] = true
		}
	}
	for id := range lines {
		dest.RouteIDs = append(dest.RouteIDs, id)
	}
	x.sortRoutes(dest.RouteIDs)
	sort.SliceStable(dest.StopIDs, func(i, j int) bool {
		a, b := len(x.calls[dest.StopIDs[i]]), len(x.calls[dest.StopIDs[j]])
		if a != b {
			return a > b
		}
		return x.stops[dest.StopIDs[i]].StopName < x.stops[dest.StopIDs[j]].StopName
	})

	byOrigin := map[*Destination]map[int]Ride{}
	for _, p := range x.patterns {
		// The latest stop of each area so far: the shortest ride from it is to the first stop in dest after it.
		last := map[*Destination]int{}
		done := map[*Destination]bool{}
		for j, id := range p.stops {
			if x.areaOf[id] == dest {
				for origin, i := range last {
					if done[origin] {
						continue
					}
					done[origin] = true
					ride, ok := x.ride(p, i, j)
					if !ok {
						continue
					}
					rides := byOrigin[origin]
					if rides == nil {
						rides = map[int]Ride{}
						byOrigin[origin] = rides
					}
					if have, seen := rides[ride.RouteID]; !seen || ride.expected() < have.expected() {
						rides[ride.RouteID] = ride
					}
				}
				continue
			}
			if origin := x.areaOf[id]; origin != nil {
				last[origin] = j
				done[origin] = false
			}
		}
	}
	origins := make([]*Destination, 0, len(byOrigin))
	for origin := range byOrigin {
		origins = append(origins, origin)
	}
	sort.Slice(origins, func(i, j int) bool {
		if sizes[origins[i]] != sizes[origins[j]] {
			return sizes[origins[i]] > sizes[origins[j]]
		}
		return origins[i].Slug < origins[j].Slug
	})
	dest.From = make([]Origin, 0, len(origins))
	for _, origin := range origins {
		o := Origin{Slug: origin.Slug, Name: origin.Name}
		for _, r := range byOrigin[origin] {
			o.Rides = append(o.Rides, r)
		}
		sort.Slice(o.Rides, func(i, j int) bool {
			if a, b := o.Rides[i].expected(), o.Rides[j].expected(); a != b {
				return a < b
			}
			return lineLess(x.routes[o.Rides[i].RouteID].RouteShortName, x.routes[o.Rides[j].RouteID].RouteShortName)
		})
		dest.From = append(dest.From, o)
	}
}

// expected is the average trip: half the gap waited, then the middle of the ride times; a single run of the day comes last.
func (r Ride) expected() float64 {
	if r.EveryMin == 0 {
		return math.Inf(1)
	}
	return float64(r.EveryMin)/2 + float64(r.Minutes[0]+r.Minutes[1])/2
}

// service is a direction's departures from its first stop on a kind of day, in minutes, and their usual gap.
type service struct {
	times []float64
	every float64
}

func serviceOn(t *models.Timetable, dayType string, dir int) (service, bool) {
	if t == nil {
		return service{}, false
	}
	if times := transit.Departures(t, dayType, dir); len(times) > 0 {
		return service{times: times, every: medianGap(times)}, true
	}
	day := t.Weekdays
	switch dayType {
	case calendar.Saturday:
		day = t.Saturday
	case calendar.Sunday:
		day = t.Sunday
	}
	f := day.InFrequency
	if dir == 1 {
		f = day.OutFrequency
	}
	if f == nil {
		return service{}, false
	}
	start, okStart := transit.ParseDeparture(f.Start)
	end, okEnd := transit.ParseDeparture(f.End)
	if !okStart || !okEnd || end < start {
		return service{}, false
	}
	// Headway-only lines: one sample an hour over their span.
	var times []float64
	for m := start; m < end; m += 60 {
		times = append(times, float64(m))
	}
	return service{times: append(times, float64(end)), every: float64(f.MaxMinutes)}, true
}

func medianGap(times []float64) float64 {
	var gaps []float64
	for i := 1; i < len(times); i++ {
		if g := times[i] - times[i-1]; g > 0 {
			gaps = append(gaps, g)
		}
	}
	if len(gaps) == 0 {
		return 0
	}
	sort.Float64s(gaps)
	return gaps[len(gaps)/2]
}

func (x *Index) ride(p *pattern, i, j int) (Ride, bool) {
	for _, day := range calendar.ScheduleDays {
		svc, ok := serviceOn(p.route.Timetable, day, p.dir)
		if !ok {
			continue
		}
		reach := func(at float64, k int) float64 {
			hours := p.reach[day]
			h := int(at/60) % 24
			if h >= len(hours) || k >= len(hours[h]) {
				return 0
			}
			return hours[h][k]
		}
		lo, hi := math.Inf(1), math.Inf(-1)
		for _, at := range svc.times {
			secs := reach(at, j) - reach(at, i)
			lo, hi = math.Min(lo, secs), math.Max(hi, secs)
		}
		first, last := svc.times[0], svc.times[len(svc.times)-1]
		return Ride{
			RouteID: p.route.RouteID, Direction: p.dir, Board: p.stops[i], Alight: p.stops[j],
			Minutes:  [2]int{rideMinutes(lo), rideMinutes(hi)},
			EveryMin: int(math.Round(svc.every)),
			First:    clock(first + reach(first, i)/60),
			Last:     clock(last + reach(last, i)/60),
			DayType:  day,
		}, true
	}
	return Ride{}, false
}

func rideMinutes(secs float64) int { return max(1, int(math.Round(secs/60))) }

func clock(minutes float64) string {
	m := int(math.Round(minutes)) % (24 * 60)
	return fmt.Sprintf("%02d:%02d", m/60, m%60)
}

// Line names with their numbers compared as numbers, as the clients sort them: 5, 5B, 24, M11, TE3, TE10.
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

// chunk splits off the leading run of digits, or of anything else.
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

func (x *Index) Route(id int) *models.NetworkRoute { return x.routes[id] }

func (x *Index) Stop(id int) *models.Stop { return x.stops[id] }

// RouteStops are the stops of one direction of a line, in order.
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

// Toward is the end a direction of a line heads for, from its long name "A - B".
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

// StationLines are the lines at a stop's station, grouped by the end they leave for, as the stop page lists them.
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

// appendLine keeps one direction of each line.
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

// AreaOf is the destination a stop is in, nil before the place index is read.
func (x *Index) AreaOf(stopID int) *Destination { return x.areaOf[stopID] }

func (x *Index) Destination(slug string) *Destination { return x.bySlug[slug] }

// Destinations are every area with a stop, by name.
func (x *Index) Destinations() []*Destination { return x.list }

func (x *Index) Summaries() []Summary {
	out := make([]Summary, 0, len(x.list))
	for _, d := range x.list {
		out = append(out, Summary{Slug: d.Slug, Name: d.Name, District: d.District, Stops: len(d.StopIDs), Lines: len(d.RouteIDs)})
	}
	return out
}
