package botosani

import (
	"conexiuni-cluj/calendar"
	"conexiuni-cluj/destinations"
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/feeds/roads"
	"conexiuni-cluj/feeds/traces"
	"conexiuni-cluj/models"
	"conexiuni-cluj/router"
	"fmt"
	"log"
	"math"
	"slices"
	"time"
)

type static struct {
	page    string
	mapPage string
	cadence time.Duration
	ids     feeds.IDMap
	school  feeds.School
}

func (s *static) Cadence() time.Duration { return s.cadence }

func (s *static) Snapshot() (*feeds.Snapshot, error) {
	page, err := feeds.Download(s.page)
	if err != nil {
		return nil, fmt.Errorf("eltransbt.ro: %w", err)
	}
	parsed, err := parsePage(page, s.page)
	if err != nil {
		return nil, err
	}
	data, err := feeds.Download(parsed.program)
	if err != nil {
		return nil, fmt.Errorf("eltransbt.ro: %w", err)
	}
	rows, err := readProgram(data)
	if err != nil {
		return nil, fmt.Errorf("eltransbt.ro: %s: %w", parsed.program, err)
	}
	var adit aditMap
	if len(parsed.suburban) == 0 {
		log.Printf("botosani: no lines to the villages on %s", s.page)
	} else if adit, err = s.aditMap(); err != nil {
		return nil, err
	}
	var stops []models.Stop
	for _, l := range parsed.lines {
		for _, m := range l.markers {
			stops = append(stops, models.Stop{StopLat: m.at.Lat, StopLon: m.at.Lon})
		}
	}
	for _, t := range parsed.suburban {
		for _, p := range adit.lines[t.short].points {
			stops = append(stops, models.Stop{StopLat: p.Lat, StopLon: p.Lon})
		}
	}
	g, places, err := roads.Around(stops)
	if err != nil {
		log.Printf("botosani: %v; the lines get straight legs", err)
	}
	var villages []router.Area
	if places != nil {
		for _, a := range places.Areas() {
			if !destinations.IsDistrict(a.Place) {
				villages = append(villages, a)
			}
		}
	}
	return build(sources{
		lines: parsed.lines, programs: parseProgram(rows), suburban: parsed.suburban,
		adit: adit, villages: villages, term: termRuns(s.school, time.Now()),
	}, s.ids, streets{g})
}

type sources struct {
	lines    []pageLine
	programs map[string]*lineProgram
	suburban []suburbanTimes
	adit     aditMap
	villages []router.Area
	// By kind of day.
	term map[string]bool
}

type builder struct {
	snap    *feeds.Snapshot
	ids     feeds.IDMap
	stopIDs map[string]int
}

type way struct {
	stops   []marker
	shape   []router.Point
	offsets []float64
}

func (b *builder) route(short string, ways []way, name func(marker) string) (*models.Timetable, error) {
	routeID, err := b.ids.ExternalID("route", short)
	if err != nil {
		return nil, err
	}
	t := &models.Timetable{RouteShortName: short}
	for d, w := range ways {
		patternID := models.PatternID(routeID, d)
		for k, m := range w.stops {
			key := m.source + ":" + m.id
			id, ok := b.stopIDs[key]
			if !ok {
				if id, err = b.ids.ExternalID("stop", key); err != nil {
					return nil, err
				}
				b.stopIDs[key] = id
				stop := models.Stop{StopID: id, StopName: name(m), StopLat: m.at.Lat, StopLon: m.at.Lon}
				b.snap.Stops = append(b.snap.Stops, stop)
			}
			b.snap.TripStops = append(b.snap.TripStops, models.TripStop{TripID: patternID, StopID: id, StopSequence: k})
		}
		headsign := name(w.stops[len(w.stops)-1])
		trip := models.Trip{TripID: patternID, RouteID: routeID, DirectionID: models.DirectionType(d), TripHeadsign: headsign, ShapeID: patternID}
		b.snap.Trips = append(b.snap.Trips, trip)
		for k, p := range w.shape {
			b.snap.Shapes = append(b.snap.Shapes, models.Shape{ShapeID: patternID, ShapePtLat: p.Lat, ShapePtLon: p.Lon, ShapePtSequence: k})
		}
		if w.offsets != nil {
			if b.snap.Offsets == nil {
				b.snap.Offsets = map[string][]float64{}
			}
			b.snap.Offsets[patternID] = w.offsets
		}
	}
	t.InStopName = name(ways[0].stops[0])
	if len(ways) > 1 {
		t.OutStopName = name(ways[1].stops[0])
	}
	t.RouteLongName = t.InStopName + " - " + name(farEnd(ways[0].stops))
	b.snap.Routes = append(b.snap.Routes, models.Route{
		RouteID: routeID, RouteShortName: short, RouteLongName: t.RouteLongName, RouteType: models.Bus, RouteColor: models.FallbackRouteColor,
	})
	b.snap.Timetables[short] = t
	return t, nil
}

func build(src sources, ids feeds.IDMap, st streets) (*feeds.Snapshot, error) {
	b := &builder{snap: &feeds.Snapshot{Timetables: map[string]*models.Timetable{}}, ids: ids, stopIDs: map[string]int{}}
	spelling := map[string]string{}
	for _, line := range src.lines {
		for _, r := range line.runs {
			for _, name := range r.names {
				if _, ok := spelling[nameKey(name)]; !ok {
					spelling[nameKey(name)] = name
				}
			}
		}
	}
	stopName := func(m marker) string {
		if name, ok := spelling[nameKey(m.name)]; ok && m.source != aditSource {
			return name
		}
		return m.name
	}
	for _, line := range src.lines {
		placed := placeRuns(line, st)
		if len(placed.missing) > 0 || len(placed.unused) > 0 {
			log.Printf("botosani: %s: stations not on the map %q, markers no run stops at %d", line.short, placed.missing, len(placed.unused))
		}
		program := src.programs[line.short]
		if program == nil {
			log.Printf("botosani: %s has no departures in the programme", line.short)
			program = &lineProgram{}
		}
		var main [2][]marker
		var variants [][]marker
		for i, r := range line.runs {
			switch {
			case len(placed.runs[i]) < 2:
			case r.label == outward:
				main[0] = placed.runs[i]
			case r.label == inward:
				main[1] = placed.runs[i]
			default:
				variants = append(variants, placed.runs[i])
			}
		}
		onRoads := func(dir []marker) way {
			points := make([]router.Point, len(dir))
			for k, m := range dir {
				points[k] = m.at
			}
			return way{stops: dir, shape: roads.Through(points, st.path)}
		}
		if main[0] != nil && main[1] != nil {
			ways := []way{onRoads(main[0]), onRoads(main[1])}
			t, err := b.route(line.short, ways, stopName)
			if err != nil {
				return nil, err
			}
			lengths := [2]float64{traces.Length(ways[0].shape), traces.Length(ways[1].shape)}
			for day, schedule := range daySchedules(t) {
				var columns [2][]int
				for d, dir := range main {
					columns[d] = published(program, stopName(dir[0]), day, false)
				}
				for d, dir := range main {
					if columns[d] == nil && columns[1-d] != nil {
						columns[d] = returns(program, stopName(main[1-d][0]), day, lengths[1-d]/(lengths[0]+lengths[1]))
						log.Printf("botosani: %s %s: %d departures from %s timed from the buses' rounds", line.short, day, len(columns[d]), stopName(dir[0]))
					}
				}
				*schedule = feeds.Columns(clock(columns[0]), clock(columns[1]))
			}
		}
		for i, run := range variants {
			if i > 0 || program.variant == "" {
				log.Printf("botosani: a short run of %s starting at %s has no departures of its own in the programme", line.short, stopName(run[0]))
				continue
			}
			t, err := b.route(line.short+program.variant, []way{onRoads(run)}, stopName)
			if err != nil {
				return nil, err
			}
			for day, schedule := range daySchedules(t) {
				*schedule = feeds.Columns(clock(published(program, stopName(run[0]), day, true)), nil)
			}
		}
	}
	if err := b.suburban(src, st, stopName); err != nil {
		return nil, err
	}
	return b.snap, nil
}

func daySchedules(t *models.Timetable) map[string]*models.DaySchedule {
	return map[string]*models.DaySchedule{calendar.Weekday: &t.Weekdays, calendar.Saturday: &t.Saturday, calendar.Sunday: &t.Sunday}
}

// Nil when the programme prints none from end.
func published(program *lineProgram, end, day string, variant bool) []int {
	var out []int
	for from, days := range program.from {
		if nameKey(from) != nameKey(end) {
			continue
		}
		out = []int{}
		for _, bus := range days[day] {
			for _, d := range bus {
				if d.variant == variant {
					out = append(out, d.minute)
				}
			}
		}
	}
	return out
}

// A bus leaves the far end after the share of its round the way there takes, by length.
func returns(program *lineProgram, end, day string, share float64) []int {
	var out []int
	for from, days := range program.from {
		if nameKey(from) != nameKey(end) {
			continue
		}
		for _, bus := range days[day] {
			for j := 0; j+1 < len(bus); j++ {
				if !bus[j].variant {
					round := bus[j+1].minute - bus[j].minute
					out = append(out, bus[j].minute+int(math.Round(float64(round)*share)))
				}
			}
		}
	}
	return out
}

func clock(minutes []int) []string {
	minutes = slices.Clone(minutes)
	slices.Sort(minutes)
	minutes = slices.Compact(minutes)
	out := make([]string, len(minutes))
	for i, m := range minutes {
		out[i] = fmt.Sprintf("%02d:%02d", m/60, m%60)
	}
	return out
}

func farEnd(dir []marker) marker {
	last := dir[len(dir)-1]
	if nameKey(last.name) != nameKey(dir[0].name) {
		return last
	}
	far := last
	for _, m := range dir {
		if distance(dir[0].at, m.at) > distance(dir[0].at, far.at) {
			far = m
		}
	}
	return far
}
