package ramnicuvalcea

import (
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/feeds/sitegtfs"
	"conexiuni-cluj/router"
	"fmt"
	"log"
	"net/url"
	"sync"
	"time"
)

// site reads ETA's pages again only when a timetable page shows a newer update.
type site struct {
	index, stops string
	school       feeds.School

	mu      sync.Mutex
	updated string
	lines   []sitegtfs.Line
	shapes  map[string][]router.Point
}

func (s *site) zip() ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := feeds.DownloadInTurn(s.index)
	if err != nil {
		return nil, err
	}
	index, err := parseIndex(data, s.index)
	if err != nil {
		return nil, err
	}
	// Every line links its timetable as {line}/program, and each one prints the day the whole timetable changed.
	data, err = feeds.DownloadInTurn(index[0].url + "/program?dir=dus")
	if err != nil {
		return nil, err
	}
	if updated := parseUpdated(data); s.lines == nil || updated == "" || updated != s.updated {
		lines, err := s.crawl(index)
		if err != nil {
			return nil, err
		}
		s.lines, s.shapes, s.updated = lines, sitegtfs.Draw(routes(lines)), updated
	}
	return sitegtfs.Zip(s.lines, s.shapes, s.school, time.Now())
}

// The stops map, every line's page, then the page of every stop the lines serve, where the runs' times are.
func (s *site) crawl(index []link) ([]sitegtfs.Line, error) {
	data, err := feeds.DownloadInTurn(s.stops)
	if err != nil {
		return nil, err
	}
	at := parseMap(data)
	pages := map[string]ways{}
	var stops []string
	seen := map[string]bool{}
	for _, l := range index {
		data, err := feeds.DownloadInTurn(l.url)
		if err != nil {
			return nil, err
		}
		page, err := parseLine(data)
		if err != nil {
			log.Printf("ramnicuvalcea: line %s: %v; the line is left out", l.name, err)
			continue
		}
		for w := range page {
			for i := range page[w] {
				st := &page[w][i]
				st.At, st.Placed = at[st.ID]
				st.Name = stationName(st.Name)
				if !seen[st.ID] {
					seen[st.ID] = true
					stops = append(stops, st.ID)
				}
			}
		}
		pages[l.name] = page
	}
	base, err := url.Parse(s.stops)
	if err != nil {
		return nil, err
	}
	var passings []passing
	for _, id := range stops {
		// Stop pages sit next to the map; with arrivals on they also list the runs that end there.
		stop, err := base.Parse(id + "?arrivals=on")
		if err != nil {
			return nil, err
		}
		data, err := feeds.DownloadInTurn(stop.String())
		if err != nil {
			return nil, err
		}
		p, err := parseStop(data)
		if err != nil {
			log.Printf("ramnicuvalcea: stop %s: %v; its times are left out", id, err)
			continue
		}
		passings = append(passings, p...)
	}
	out := lines(index, pages, passings)
	if len(out) == 0 {
		return nil, fmt.Errorf("eta-bus.ro: no runs on %d stop pages", len(stops))
	}
	sitegtfs.Place(routes(out))
	return out, nil
}

func routes(lines []sitegtfs.Line) []sitegtfs.Route {
	var out []sitegtfs.Route
	for _, l := range lines {
		for _, r := range l.Ways {
			if r != nil {
				out = append(out, *r)
			}
		}
	}
	return out
}
