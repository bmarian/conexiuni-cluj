package suceava

import (
	"conexiuni-cluj/calendar"
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/geo"
	"conexiuni-cluj/models"
	"conexiuni-cluj/planner"
	"conexiuni-cluj/router"
	"fmt"
	"log"
	"os"
	"slices"
	"strings"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// static is the volunteer-made zip's network and, from the operator's own pages, the lines it lacks.
type static struct {
	zip   feeds.Static
	pages string
	ids   feeds.IDMap
}

func (s *static) Cadence() time.Duration { return s.zip.Cadence() }

func (s *static) Snapshot() (*feeds.Snapshot, error) {
	snap, err := s.zip.Snapshot()
	if err != nil {
		return nil, err
	}
	lines, err := fetchLines(s.pages)
	if err != nil {
		return nil, err
	}
	have := map[string]bool{}
	for _, r := range snap.Routes {
		have[r.RouteShortName] = true
	}
	var missing []pageLine
	for _, l := range lines {
		if !have[l.short] && len(l.departures) > 0 {
			missing = append(missing, l)
		}
	}
	if len(missing) == 0 {
		return snap, nil
	}
	roads, places := loadOSM(snap.Stops)
	if err := addLines(snap, missing, newPlacer(snap, roads, places), s.ids); err != nil {
		return nil, err
	}
	return snap, nil
}

// The roads and places around the zip's stops, as far again as half their spread; nil without the extract.
func loadOSM(stops []models.Stop) (*router.WalkGraph, *router.Places) {
	if len(stops) == 0 {
		return nil, nil
	}
	box := router.BBox{MinLat: stops[0].StopLat, MinLon: stops[0].StopLon, MaxLat: stops[0].StopLat, MaxLon: stops[0].StopLon}
	for _, s := range stops {
		box.MinLat, box.MaxLat = min(box.MinLat, s.StopLat), max(box.MaxLat, s.StopLat)
		box.MinLon, box.MaxLon = min(box.MinLon, s.StopLon), max(box.MaxLon, s.StopLon)
	}
	box = box.Grow(geo.Haversine(box.MinLat, box.MinLon, box.MaxLat, box.MaxLon) / 2)
	f, err := os.Open(planner.OSMPath())
	if err != nil {
		log.Printf("suceava: %v; the lines from tpl-sv.ro get straight legs and only the zip's stops", err)
		return nil, nil
	}
	defer f.Close()
	roads, places, err := router.LoadRoads(f, box)
	if err != nil {
		log.Printf("suceava: roads: %v; the lines from tpl-sv.ro get straight legs and only the zip's stops", err)
		return nil, nil
	}
	return roads, places
}

var title = cases.Title(language.Romanian)

func addLines(snap *feeds.Snapshot, lines []pageLine, p *placer, ids feeds.IDMap) error {
	placed, counts := p.place(lines)
	color := commonColor(snap.Routes)
	stopIDs := map[string]int{}
	pointsOf := map[string][]router.Point{}
	stopFor := func(s station) (models.Stop, error) {
		if s.stop != nil {
			return *s.stop, nil
		}
		// One stop per name and point; a second point of a name is a second stop.
		base := "tpl-sv.ro:" + feeds.NormalizeStopLabel(s.name)
		n := slices.Index(pointsOf[base], s.at)
		if n < 0 {
			n, pointsOf[base] = len(pointsOf[base]), append(pointsOf[base], s.at)
		}
		key := base
		if n > 0 {
			key = fmt.Sprintf("%s#%d", base, n+1)
		}
		stop := models.Stop{StopName: title.String(strings.ToLower(s.name)), StopLat: s.at.Lat, StopLon: s.at.Lon}
		if id, ok := stopIDs[key]; ok {
			stop.StopID = id
			return stop, nil
		}
		id, err := ids.ExternalID("stop", key)
		if err != nil {
			return stop, err
		}
		stop.StopID, stopIDs[key] = id, id
		snap.Stops = append(snap.Stops, stop)
		return stop, nil
	}
	added := 0
	for i, line := range lines {
		dirs := placed[i]
		if len(dirs) == 0 {
			log.Printf("suceava: line %s from tpl-sv.ro left out: nothing places its first or last station", line.short)
			continue
		}
		// The zip's route ids, so the line keeps its number once the zip has it.
		routeID, err := ids.ExternalID("route", "TPL"+line.short)
		if err != nil {
			return err
		}
		var first [2]string
		for d, dir := range dirs {
			patternID := models.PatternID(routeID, d)
			var names []string
			for k, s := range dir {
				stop, err := stopFor(s)
				if err != nil {
					return err
				}
				names = append(names, stop.StopName)
				snap.TripStops = append(snap.TripStops, models.TripStop{TripID: patternID, StopID: stop.StopID, StopSequence: k})
			}
			first[d] = names[0]
			snap.Trips = append(snap.Trips, models.Trip{TripID: patternID, RouteID: routeID, DirectionID: models.DirectionType(d), TripHeadsign: names[len(names)-1], ShapeID: patternID})
			for k, pt := range p.shape(dir) {
				snap.Shapes = append(snap.Shapes, models.Shape{ShapeID: patternID, ShapePtLat: pt.Lat, ShapePtLon: pt.Lon, ShapePtSequence: k})
			}
		}
		longName := longName(dirs[0], first[0])
		snap.Routes = append(snap.Routes, models.Route{RouteID: routeID, RouteShortName: line.short, RouteLongName: longName, RouteType: models.Bus, RouteColor: color})
		t := &models.Timetable{RouteShortName: line.short, RouteLongName: longName, InStopName: first[0], OutStopName: first[1]}
		for kind, schedule := range map[string]*models.DaySchedule{calendar.Weekday: &t.Weekdays, calendar.Saturday: &t.Saturday, calendar.Sunday: &t.Sunday} {
			byDir := line.departures[kind]
			var columns [2][]string
			for d := range min(len(byDir), len(columns)) {
				for _, m := range byDir[d] {
					columns[d] = append(columns[d], fmt.Sprintf("%02d:%02d", m/60, m%60))
				}
			}
			*schedule = feeds.Columns(columns[0], columns[1])
		}
		if snap.Timetables == nil {
			snap.Timetables = map[string]*models.Timetable{}
		}
		snap.Timetables[line.short] = t
		added++
	}
	log.Printf("suceava: %d lines from tpl-sv.ro added; stations: %d on the zip's stops, %d at places of their name, %d spaced along the road, %d left out as nothing places them",
		added, counts[fromZip], counts[fromName], counts[spaced], counts[unplaced])
	return nil
}

// A line is named after its ends, a loop after its start and the station furthest from it.
func longName(dir []station, start string) string {
	firstAt, last := dir[0].at, dir[len(dir)-1]
	if feeds.NormalizeStopLabel(dir[0].name) != feeds.NormalizeStopLabel(last.name) {
		return start + " - " + stationName(last)
	}
	far, farM := last, 0.0
	for _, s := range dir {
		if m := geo.Haversine(firstAt.Lat, firstAt.Lon, s.at.Lat, s.at.Lon); m > farM {
			far, farM = s, m
		}
	}
	return start + " - " + stationName(far)
}

func stationName(s station) string {
	if s.stop != nil {
		return s.stop.StopName
	}
	return title.String(strings.ToLower(s.name))
}

// The colour most of the zip's lines share.
func commonColor(routes []models.Route) string {
	count := map[string]int{}
	best := ""
	for _, r := range routes {
		count[r.RouteColor]++
		if best == "" || count[r.RouteColor] > count[best] {
			best = r.RouteColor
		}
	}
	return best
}
