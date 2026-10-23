package destinations

import (
	"conexiuni-cluj/models"
	"conexiuni-cluj/router"
	"strconv"
	"strings"
)

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

type Ride struct {
	RouteID   int `json:"route_id"`
	Direction int `json:"direction"`
	Board     int `json:"board"`
	Alight    int `json:"alight"`
	// Fastest and slowest of the day.
	Minutes  [2]int `json:"minutes"`
	EveryMin int    `json:"every_min,omitempty"`
	First    string `json:"first"`
	Last     string `json:"last"`
	DayType  string `json:"day_type"`
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

// Toward is empty for the lines ending there.
type Departing struct {
	Toward string
	Lines  []Line
}

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

func tripDirection(tripID string) int {
	if i := strings.LastIndexByte(tripID, '_'); i >= 0 {
		if d, err := strconv.Atoi(tripID[i+1:]); err == nil {
			return d
		}
	}
	return 0
}

func (x *Index) Route(id int) *models.NetworkRoute { return x.routes[id] }

func (x *Index) Stop(id int) *models.Stop { return x.stops[id] }

func (x *Index) AreaOf(stopID int) *Destination { return x.areaOf[stopID] }

func (x *Index) Destination(slug string) *Destination { return x.bySlug[slug] }

func (x *Index) Destinations() []*Destination { return x.list }

func (x *Index) Summaries() []Summary {
	out := make([]Summary, 0, len(x.list))
	for _, d := range x.list {
		out = append(out, Summary{Slug: d.Slug, Name: d.Name, District: d.District, Stops: len(d.StopIDs), Lines: len(d.RouteIDs)})
	}
	return out
}
