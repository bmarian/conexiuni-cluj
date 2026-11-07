package botosani

import (
	"conexiuni-cluj/destinations"
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/feeds/roads"
	"conexiuni-cluj/feeds/traces"
	"conexiuni-cluj/router"
	"log"
	"math"
	"slices"
	"strings"
)

type suburbanLine struct {
	suburbanTimes
	line      mapLine
	town, far int
}

func (b *builder) suburban(src sources, st streets, name func(marker) string) error {
	stations := cityStations(src.lines)
	if len(stations) == 0 {
		log.Printf("botosani: no town stations for the lines to the villages")
		return nil
	}
	fromTown := func(m marker) float64 {
		return distance(stations[nearestTo(stations, m.at, func(s station) router.Point { return s.centre })].centre, m.at)
	}
	drawn := map[string]mapLine{}
	for _, t := range src.suburban {
		if line, ok := src.adit.lines[t.short]; ok && t.follows == "" && len(line.stops) >= 2 {
			drawn[t.short] = townFirst(line, fromTown)
		}
	}
	for _, t := range src.suburban {
		if base, ok := drawn[t.follows]; ok && t.follows != "" {
			if line, ok := stitch(base, t.title[1:], src.adit.pieces, src.villages); ok {
				drawn[t.short] = line
			}
		}
	}
	var lines []suburbanLine
	renamed := map[string]string{}
	var all []marker
	seen := map[string]bool{}
	for _, t := range src.suburban {
		line, ok := drawn[t.short]
		if !ok {
			log.Printf("botosani: %s has no stops on the ADIT map", t.short)
			continue
		}
		town := slices.IndexFunc(t.ends, func(e suburbanEnd) bool { return e.hub != "" })
		if len(t.ends) != 2 || town < 0 || t.ends[1-town].hub != "" {
			log.Printf("botosani: %s: the page gives %d ends, not one in town and one out", t.short, len(t.ends))
			continue
		}
		far := 1 - town
		// ADIT names some termini after their commune.
		last, place := line.stops[len(line.stops)-1], t.ends[far].place
		if last.name == unnamedStation || !strings.Contains(nameKey(last.name), nameKey(place)) && nearestVillage(last.at, src.villages, place) {
			renamed[last.id] = place
		}
		lines = append(lines, suburbanLine{t, line, town, far})
		for _, m := range line.stops {
			if !seen[m.id] {
				seen[m.id] = true
				all = append(all, m)
			}
		}
	}
	shared := sharedStations(stations, all)
	named := func(m marker) string {
		if place, ok := renamed[m.id]; ok && m.source == aditSource {
			return place
		}
		return name(m)
	}
	for _, l := range lines {
		line := l.line.keep(func(k int) bool { return named(l.line.stops[k]) != unnamedStation })
		there, ok := suburbanWay(line, shared, stations, st, minutesFrom(l.suburbanTimes, l.town))
		back, okBack := suburbanWay(backwards(line), shared, stations, st, minutesFrom(l.suburbanTimes, l.far))
		if !ok || !okBack {
			log.Printf("botosani: %s: fewer than two stops on the ADIT map", l.short)
			continue
		}
		timetable, err := b.route(l.short, []way{there, back}, named)
		if err != nil {
			return err
		}
		timetable.RouteLongName = l.title[0] + " - " + l.title[len(l.title)-1]
		b.snap.Routes[len(b.snap.Routes)-1].RouteLongName = timetable.RouteLongName
		for day, schedule := range daySchedules(timetable) {
			*schedule = feeds.Columns(clock(l.ends[l.town].running(day, src.term[day])), clock(l.ends[l.far].running(day, src.term[day])))
		}
	}
	return nil
}

func nearestVillage(p router.Point, villages []router.Area, place string) bool {
	village, _ := destinations.Nearest(villages, p.Lat, p.Lon)
	return village != nil && nameKey(village.Name) == nameKey(place)
}

func minutesFrom(t suburbanTimes, end int) int {
	switch len(t.minutes) {
	case 0:
		return 0
	case 1:
		return t.minutes[0]
	case len(t.ends):
		return t.minutes[end]
	}
	log.Printf("botosani: %s: trip time %v does not fit its %d ends", t.short, t.minutes, len(t.ends))
	return 0
}

func (e suburbanEnd) running(day string, term bool) []int {
	var out []int
	for _, d := range e.days[day] {
		if !d.term || term {
			out = append(out, d.minute)
		}
	}
	return out
}

func suburbanWay(line mapLine, shared map[string]int, stations []station, st streets, minutes int) (way, bool) {
	stationOf := func(k int) int {
		if i, ok := shared[line.stops[k].id]; ok {
			return i
		}
		return -1
	}
	// Both sides of a station drawn in a row are one stop; a line leaves from the last.
	kept := make([]bool, len(line.stops))
	for k := range kept {
		kept[k] = k == 0 || stationOf(k) < 0 || stationOf(k) != stationOf(k-1)
	}
	for k := 1; k < len(kept) && !kept[k]; k++ {
		kept[k-1], kept[k] = false, true
	}
	line = line.keep(func(k int) bool { return kept[k] })
	if len(line.stops) < 2 {
		return way{}, false
	}
	w := way{stops: slices.Clone(line.stops)}
	for k, m := range line.stops {
		i, ok := shared[m.id]
		if !ok {
			continue
		}
		candidates := stations[i].markers
		switch {
		case k == 0:
			w.stops[k] = furthest(candidates, line.stops[1].at)
		case k == len(line.stops)-1:
			w.stops[k] = st.rightmost(candidates, line.stops[k-1].at, m.at)
		default:
			w.stops[k] = st.rightmost(candidates, line.stops[k-1].at, line.stops[k+1].at)
		}
	}
	positions := make([]router.Point, len(w.stops))
	for k, m := range w.stops {
		positions[k] = m.at
	}
	lengths := make([]float64, len(w.stops))
	k := 0
	w.shape = roads.Through(positions, func(a, b router.Point) []router.Point {
		k++
		span := append(append([]router.Point{a}, line.points[line.at[k-1]+1:line.at[k]]...), b)
		drawn, leg := traces.Length(span), roads.Through(span, st.path)
		// A drawn point off the road can detour up a side street.
		if direct := st.path(a, b); math.Abs(traces.Length(direct)-drawn) < math.Abs(traces.Length(leg)-drawn) {
			leg = direct
		}
		lengths[k] = traces.Length(leg)
		return leg
	})
	total := 0.0
	for _, l := range lengths {
		total += l
	}
	if minutes > 0 && total > 0 {
		w.offsets = make([]float64, len(lengths))
		for k, l := range lengths {
			w.offsets[k] = float64(minutes) * 60 * l / total
		}
	}
	return w, true
}
