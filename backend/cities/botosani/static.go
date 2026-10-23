package botosani

import (
	"conexiuni-cluj/calendar"
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
	cadence time.Duration
	ids     feeds.IDMap
}

func (s *static) Cadence() time.Duration { return s.cadence }

func (s *static) Snapshot() (*feeds.Snapshot, error) {
	page, err := feeds.Download(s.page)
	if err != nil {
		return nil, fmt.Errorf("eltransbt.ro: %w", err)
	}
	lines, program, err := parsePage(page, s.page)
	if err != nil {
		return nil, err
	}
	data, err := feeds.Download(program)
	if err != nil {
		return nil, fmt.Errorf("eltransbt.ro: %w", err)
	}
	rows, err := readProgram(data)
	if err != nil {
		return nil, fmt.Errorf("eltransbt.ro: %s: %w", program, err)
	}
	var stops []models.Stop
	for _, l := range lines {
		for _, m := range l.markers {
			stops = append(stops, models.Stop{StopLat: m.at.Lat, StopLon: m.at.Lon})
		}
	}
	g, _, err := roads.Around(stops)
	if err != nil {
		log.Printf("botosani: %v; the lines get straight legs", err)
	}
	return build(lines, parseProgram(rows), s.ids, streets{g})
}

func build(lines []pageLine, programs map[string]*lineProgram, ids feeds.IDMap, st streets) (*feeds.Snapshot, error) {
	snap := &feeds.Snapshot{Timetables: map[string]*models.Timetable{}}
	stopIDs := map[string]int{}
	for _, line := range lines {
		placed := placeRuns(line, st)
		if len(placed.missing) > 0 || len(placed.unused) > 0 {
			log.Printf("botosani: %s: stations not on the map %q, markers no run stops at %d", line.short, placed.missing, len(placed.unused))
		}
		spelling := map[string]string{}
		for _, r := range line.runs {
			for _, name := range r.names {
				if _, ok := spelling[nameKey(name)]; !ok {
					spelling[nameKey(name)] = name
				}
			}
		}
		stopName := func(m marker) string {
			if name, ok := spelling[nameKey(m.name)]; ok {
				return name
			}
			return m.name
		}
		program := programs[line.short]
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
		add := func(short string, dirs [][]marker) (*models.Timetable, [][]router.Point, error) {
			routeID, err := ids.ExternalID("route", short)
			if err != nil {
				return nil, nil, err
			}
			var shapes [][]router.Point
			t := &models.Timetable{RouteShortName: short}
			for d, dir := range dirs {
				patternID := models.PatternID(routeID, d)
				points := make([]router.Point, len(dir))
				for k, m := range dir {
					id, ok := stopIDs[m.id]
					if !ok {
						if id, err = ids.ExternalID("stop", "eltransbt.ro:"+m.id); err != nil {
							return nil, nil, err
						}
						stopIDs[m.id] = id
						snap.Stops = append(snap.Stops, models.Stop{StopID: id, StopName: stopName(m), StopLat: m.at.Lat, StopLon: m.at.Lon})
					}
					points[k] = m.at
					snap.TripStops = append(snap.TripStops, models.TripStop{TripID: patternID, StopID: id, StopSequence: k})
				}
				snap.Trips = append(snap.Trips, models.Trip{TripID: patternID, RouteID: routeID, DirectionID: models.DirectionType(d), TripHeadsign: stopName(dir[len(dir)-1]), ShapeID: patternID})
				shape := roads.Through(points, st.path)
				for k, p := range shape {
					snap.Shapes = append(snap.Shapes, models.Shape{ShapeID: patternID, ShapePtLat: p.Lat, ShapePtLon: p.Lon, ShapePtSequence: k})
				}
				shapes = append(shapes, shape)
			}
			t.InStopName = stopName(dirs[0][0])
			if len(dirs) > 1 {
				t.OutStopName = stopName(dirs[1][0])
			}
			t.RouteLongName = t.InStopName + " - " + stopName(farEnd(dirs[0]))
			snap.Routes = append(snap.Routes, models.Route{RouteID: routeID, RouteShortName: short, RouteLongName: t.RouteLongName, RouteType: models.Bus, RouteColor: models.FallbackRouteColor})
			snap.Timetables[short] = t
			return t, shapes, nil
		}
		if main[0] != nil && main[1] != nil {
			t, shapes, err := add(line.short, main[:])
			if err != nil {
				return nil, err
			}
			lengths := [2]float64{traces.Length(shapes[0]), traces.Length(shapes[1])}
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
			t, _, err := add(line.short+program.variant, [][]marker{run})
			if err != nil {
				return nil, err
			}
			for day, schedule := range daySchedules(t) {
				*schedule = feeds.Columns(clock(published(program, stopName(run[0]), day, true)), nil)
			}
		}
	}
	return snap, nil
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
