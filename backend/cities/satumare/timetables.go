package satumare

import (
	"conexiuni-cluj/calendar"
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/models"
	"log"
)

// Transurban's own departures from tusm.ro's line pages; weekend and holiday lists are both Saturday's and Sunday's.
type timetables struct {
	index string
}

func (s *timetables) Timetables(snap *feeds.Snapshot) (map[string]*models.Timetable, error) {
	routes, err := crawl(s.index)
	if err != nil {
		return nil, err
	}
	return lay(snap, routes), nil
}

// One page at a time, each after a pause as long as it took to answer; the index shows the first line itself.
func crawl(index string) (map[string]route, error) {
	data, err := feeds.DownloadInTurn(index)
	if err != nil {
		return nil, err
	}
	links, err := parseIndex(data, index)
	if err != nil {
		return nil, err
	}
	out := map[string]route{}
	shown := read(out, data, index)
	for _, l := range links {
		if shown[l.line] {
			continue
		}
		data, err := feeds.DownloadInTurn(l.url)
		if err != nil {
			return nil, err
		}
		read(out, data, l.url)
	}
	return out, nil
}

// Adds a page's routes by line number and says which it had.
func read(into map[string]route, data []byte, url string) map[string]bool {
	routes, err := parsePage(data)
	if err != nil {
		log.Printf("satumare: %s: %v; its lines keep the zip's times", url, err)
	}
	labels := map[string]bool{}
	for _, r := range routes {
		into[r.label], labels[r.label] = r, true
	}
	return labels
}

// Every line of the snapshot: the site's departures where it lists any, else the zip's own weekday ones.
func lay(snap *feeds.Snapshot, routes map[string]route) map[string]*models.Timetable {
	out := make(map[string]*models.Timetable, len(snap.Routes))
	for _, sr := range snap.Routes {
		r, ok := routes[sr.RouteShortName]
		if !ok || !r.runs() {
			if t, ok := snap.Timetables[sr.RouteShortName]; ok {
				out[sr.RouteShortName] = t
			}
			continue
		}
		t := &models.Timetable{RouteShortName: sr.RouteShortName, RouteLongName: sr.RouteLongName, InStopName: r.from[0], OutStopName: r.from[1]}
		for kind, day := range map[string]*models.DaySchedule{calendar.Weekday: &t.Weekdays, calendar.Saturday: &t.Saturday, calendar.Sunday: &t.Sunday} {
			*day = feeds.Columns(r.times[0][kind], r.times[1][kind])
			feeds.NormalizeEntries(day.Entries)
		}
		feeds.AlignDirections(t, feeds.FirstStopNames(snap, sr.RouteID), "tusm.ro")
		out[sr.RouteShortName] = t
	}
	return out
}

func (r route) runs() bool {
	for _, byKind := range r.times {
		for _, times := range byKind {
			if len(times) > 0 {
				return true
			}
		}
	}
	return false
}
