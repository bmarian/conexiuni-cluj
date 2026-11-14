package search

import (
	"cmp"
	"math"
	"slices"
	"strings"
	"unicode"

	"conexiuni-cluj/live"
	"conexiuni-cluj/models"
	"conexiuni-cluj/router"
)

// Same as placesLimit in planner/places.go.
const resultsKept = 10

const placeMinLetters = 3

type stopHit struct {
	ID   int `json:"id"`
	Rank int `json:"rank"`
}

type placeHit struct {
	router.Place
	Far bool `json:"far,omitempty"`
}

type otherLine struct {
	City     string `json:"city"`
	CityName string `json:"city_name"`
	RouteID  int    `json:"route_id"`
	Name     string `json:"name"`
	LongName string `json:"long_name"`
	Color    string `json:"color"`
	Slug     string `json:"slug,omitempty"`
}

type cityHit struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

type tripEnd struct {
	Name   string  `json:"name"`
	Lat    float64 `json:"lat"`
	Lon    float64 `json:"lon"`
	StopID int     `json:"stop_id,omitempty"`
}

// From is nil for where the rider is.
type trip struct {
	From *tripEnd `json:"from"`
	To   tripEnd  `json:"to"`
}

type farPlace struct {
	Name    string `json:"name"`
	Town    bool   `json:"town,omitempty"`
	RouteID int    `json:"route_id,omitempty"`
	StopID  int    `json:"stop_id"`
	Meters  int    `json:"meters"`
}

type notice struct {
	Kind     string `json:"kind"`
	Name     string `json:"name,omitempty"`
	Operator string `json:"operator,omitempty"`
	URL      string `json:"url,omitempty"`
}

const (
	noticeCityMissing = "city_missing"
	noticeBusOff      = "bus_off"
	noticeTickets     = "tickets"
)

const (
	bestLine  = "line"
	bestStop  = "stop"
	bestPlace = "place"
	bestBus   = "bus"
	bestOther = "other"
	bestCity  = "city"
	bestTrip  = "trip"
	bestFar   = "far"
)

// Best names the list whose first item is the top result.
type answer struct {
	Best   string      `json:"best,omitempty"`
	Show   string      `json:"show,omitempty"`
	Lines  []int       `json:"lines"`
	Stops  []stopHit   `json:"stops"`
	Places []placeHit  `json:"places"`
	Buses  []live.Bus  `json:"buses"`
	Other  []otherLine `json:"other"`
	Cities []cityHit   `json:"cities"`
	Trip   *trip       `json:"trip,omitempty"`
	Far    *farPlace   `json:"far,omitempty"`
	Notice *notice     `json:"notice,omitempty"`
}

func (a *answer) empty() bool {
	return len(a.Lines)+len(a.Stops)+len(a.Places)+len(a.Buses)+len(a.Other)+len(a.Cities) == 0 && a.Trip == nil
}

func (s *Service) find(n *names, q string) answer {
	a := answer{Lines: []int{}, Stops: []stopHit{}, Places: []placeHit{}, Buses: []live.Bus{}, Other: []otherLine{}, Cities: []cityHit{}}
	r := read(q)
	a.Show = r.show
	words, done := s.elsewhere(&a, r.words)
	if done {
		return a
	}
	text := strings.Join(words, " ")

	exact, rest := lineNamed(n.bundle, words)
	if exact != nil {
		a.Lines = append(a.Lines, exact.RouteID)
		if len(rest) == 0 {
			a.Best = bestLine
		}
	}
	for _, m := range sortedMatches(matching(n.routes, func(r namedRoute) string { return r.name }, text)) {
		if len(a.Lines) < resultsKept && !slices.Contains(a.Lines, m.item.route.RouteID) {
			a.Lines = append(a.Lines, m.item.route.RouteID)
		}
	}
	stops := sortedMatches(matching(n.stops, func(st namedStop) string { return st.name }, text))
	for _, m := range stops[:min(len(stops), resultsKept)] {
		a.Stops = append(a.Stops, stopHit{ID: m.item.stop.StopID, Rank: m.rank})
	}
	if r.only == stopsOnly && len(a.Stops) > 0 {
		a.Lines = a.Lines[:0]
	}
	if r.only == linesOnly && len(a.Lines) > 0 {
		a.Stops = a.Stops[:0]
	}
	s.busesNamed(&a, words)
	s.placesNamed(&a, r, text)
	if !r.tickets && !s.strong(&a, text, stops) && !s.isTown(words) {
		s.trip(&a, n, q, words)
	}
	s.pickBest(&a, text, stops)
	s.notices(&a, r, words)
	return a
}

func sortedMatches[T any](found []ranked[T]) []ranked[T] {
	slices.SortStableFunc(found, func(a, b ranked[T]) int { return cmp.Compare(a.rank, b.rank) })
	return found
}

// A city named alone is listed and the search goes on.
func (s *Service) elsewhere(a *answer, words []string) ([]string, bool) {
	cities := slices.DeleteFunc(s.Cities(), func(c City) bool { return c.Slug == s.Here.Slug })
	if city, rest := namedCity(words, cities); city != nil {
		if s.otherLine(a, city, rest) {
			return nil, true
		}
		a.Cities = append(a.Cities, cityHit{Slug: city.Slug, Name: city.Name})
		if len(rest) == 0 {
			a.Best = bestCity
		}
		return words, false
	}
	if city, rest := namedCity(words, []City{s.Here}); city != nil && len(rest) > 0 {
		words = rest
	}
	ops, rest := namedOperator(words, append(cities, s.Here), s.Here.Slug)
	for _, op := range ops {
		if op.Slug == s.Here.Slug {
			if len(rest) > 0 {
				return rest, false
			}
			return words, false
		}
		if s.otherLine(a, op, rest) {
			return nil, true
		}
	}
	return words, false
}

func (s *Service) otherLine(a *answer, city *City, words []string) bool {
	if city.Network == nil {
		return false
	}
	bundle, _, err := city.Network()
	if err != nil {
		return false
	}
	line, _ := lineNamed(bundle, words)
	if line == nil {
		return false
	}
	a.Other = append(a.Other, otherLine{City: city.Slug, CityName: city.Name, RouteID: line.RouteID, Name: line.RouteShortName,
		LongName: line.RouteLongName, Color: line.RouteColor, Slug: line.Slug})
	a.Best = bestOther
	return true
}

func (s *Service) busesNamed(a *answer, words []string) {
	for _, b := range s.buses() {
		for _, w := range words {
			if strings.EqualFold(b.Label, w) {
				a.Buses = append(a.Buses, b)
			}
		}
	}
}

func (s *Service) placesNamed(a *answer, r reading, text string) {
	places := s.places()
	if places == nil || len([]rune(text)) < placeMinLetters {
		return
	}
	if r.only != anyKind && len(a.Lines)+len(a.Stops) > 0 {
		return
	}
	streets := s.streets()
	for _, p := range places.Search(text, resultsKept) {
		far := streets != nil && !streets.Near(router.Point{Lat: p.Lat, Lon: p.Lon})
		a.Places = append(a.Places, placeHit{Place: p, Far: far})
	}
	slices.SortStableFunc(a.Places, func(x, y placeHit) int {
		switch {
		case x.Far == y.Far:
			return 0
		case x.Far:
			return 1
		}
		return -1
	})
}

func namedBy(name, text string) bool { return nameMatches(router.Fold(name), text, false) }

// Something answers the whole text, so it is no trip.
func (s *Service) strong(a *answer, text string, stops []ranked[namedStop]) bool {
	if a.Best != "" || len(a.Buses) > 0 || len(a.Other) > 0 {
		return true
	}
	if len(stops) > 0 && stops[0].rank <= 1 {
		return true
	}
	return slices.ContainsFunc(a.Places, func(p placeHit) bool { return namedBy(p.Name, text) })
}

func (s *Service) trip(a *answer, n *names, q string, words []string) {
	if from, to, ok := tripEnds(q); ok {
		s.tripBetween(a, n, from, to, false)
		return
	}
	// Without a link word, whole words only: "pret bilet" is no trip from "Pretty".
	for i := 1; i < len(words) && a.Trip == nil; i++ {
		s.tripBetween(a, n, strings.Join(words[:i], " "), strings.Join(words[i:], " "), true)
	}
}

func (s *Service) tripBetween(a *answer, n *names, from, to string, whole bool) {
	end := s.end(n, to, whole)
	if end == nil {
		return
	}
	t := &trip{To: *end}
	if from != "" {
		if t.From = s.end(n, from, whole); t.From == nil {
			return
		}
	}
	a.Trip = t
}

// A stop by its whole name, else a place by its name, else a stop starting with it.
func (s *Service) end(n *names, text string, whole bool) *tripEnd {
	named := namedBy
	if whole {
		named = namedWhole
	}
	stops := sortedMatches(matching(n.stops, func(st namedStop) string { return st.name }, text))
	stopEnd := func(st *models.Stop) *tripEnd {
		return &tripEnd{Name: st.StopName, Lat: st.StopLat, Lon: st.StopLon, StopID: st.StopID}
	}
	for _, m := range stops {
		if m.item.name == text {
			return stopEnd(m.item.stop)
		}
	}
	if places := s.places(); places != nil {
		if found := places.Search(text, 1); len(found) > 0 && named(found[0].Name, text) {
			return &tripEnd{Name: found[0].Name, Lat: found[0].Lat, Lon: found[0].Lon}
		}
	}
	if len(stops) > 0 && stops[0].rank <= 1 && named(stops[0].item.name, text) {
		return stopEnd(stops[0].item.stop)
	}
	return nil
}

func namedWhole(name, text string) bool {
	own := router.Words(name)
	for _, w := range router.Words(text) {
		if !slices.Contains(own, w) {
			return false
		}
	}
	return true
}

func (s *Service) isTown(words []string) bool {
	_, ok := townNamed(words, s.Towns())
	return ok
}

func (s *Service) pickBest(a *answer, text string, stops []ranked[namedStop]) {
	switch {
	case a.Best != "":
	case len(a.Buses) > 0:
		a.Best = bestBus
	case a.Trip != nil:
		a.Best = bestTrip
	case len(stops) > 0 && stops[0].item.name == text:
		a.Best = bestStop
	case len(a.Places) > 0 && !a.Places[0].Far && router.Fold(a.Places[0].Name) == text:
		a.Best = bestPlace
	case len(a.Places) > 0 && a.Places[0].Far && len(a.Stops) == 0 && len(a.Lines) == 0:
		s.closest(a, a.Places[0])
	}
}

func (s *Service) closest(a *answer, p placeHit) {
	streets := s.streets()
	if streets == nil {
		return
	}
	stop, meters := streets.Nearest(router.Point{Lat: p.Lat, Lon: p.Lon})
	a.Far = &farPlace{Name: p.Name, Town: p.Town, StopID: stop.ID, Meters: int(math.Round(meters))}
	if lines, err := s.lines(); err == nil {
		if leaving := lines.StationLines(stop.ID); len(leaving) > 0 && len(leaving[0].Lines) > 0 {
			a.Far.RouteID = leaving[0].Lines[0].RouteID
		}
	}
	a.Best = bestFar
}

func digitsOnly(w string) bool {
	return w != "" && !strings.ContainsFunc(w, func(r rune) bool { return !unicode.IsDigit(r) })
}

func (s *Service) notices(a *answer, r reading, words []string) {
	if a.Best != "" {
		return
	}
	switch {
	case r.tickets:
		a.Notice = &notice{Kind: noticeTickets, Operator: s.Here.Operator, URL: s.Here.OperatorURL}
	case len(words) == 1 && digitsOnly(words[0]) && len(a.Lines) == 0 && s.busNumberLike(words[0]):
		a.Notice = &notice{Kind: noticeBusOff, Name: words[0]}
	case a.empty():
		if town, ok := townNamed(words, s.Towns()); ok {
			a.Notice = &notice{Kind: noticeCityMissing, Name: town.Name}
		}
	}
}

func (s *Service) busNumberLike(w string) bool {
	return slices.ContainsFunc(s.buses(), func(b live.Bus) bool { return digitsOnly(b.Label) && len(b.Label) == len(w) })
}
