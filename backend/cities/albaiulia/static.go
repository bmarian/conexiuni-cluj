package albaiulia

import (
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/feeds/sitegtfs"
	"conexiuni-cluj/router"
	"fmt"
	"log"
	"sync"
	"time"
)

// site reads STP's timetable pages again only when the index shows a newer timetable.
type site struct {
	index  string
	school feeds.School

	mu      sync.Mutex
	updated string
	lines   []sitegtfs.Line
	shapes  map[string][]router.Point
}

func (s *site) zip() ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := feeds.Download(s.index)
	if err != nil {
		return nil, fmt.Errorf("stpalba.ro: %w", err)
	}
	links, updated, err := parseIndex(data, s.index)
	if err != nil {
		return nil, err
	}
	if s.lines == nil || updated == "" || updated != s.updated {
		pages, err := crawl(links)
		if err != nil {
			return nil, err
		}
		s.lines, s.shapes, s.updated = lines(pages), drawn(pages), updated
	}
	return sitegtfs.Zip(s.lines, s.shapes, s.school, time.Now())
}

// One page at a time, each after a pause as long as the last one took to answer; the stops then take the names the app shows.
func crawl(links []link) ([]page, error) {
	var out []page
	for _, l := range links {
		data, err := feeds.DownloadInTurn(l.url)
		if err != nil {
			return nil, fmt.Errorf("stpalba.ro: %w", err)
		}
		p, err := parsePage(data, l)
		if err != nil {
			log.Printf("albaiulia: %s: %v; the route is left out", l.code, err)
			continue
		}
		out = append(out, p)
	}
	routes := make([]sitegtfs.Route, len(out))
	for i := range out {
		routes[i] = out[i].Route
	}
	sitegtfs.Place(routes)
	for _, p := range out {
		for i := range p.Stops {
			p.Stops[i].Name = stationName(p.Stops[i].Name)
		}
	}
	return out, nil
}

func drawn(pages []page) map[string][]router.Point {
	routes := make([]sitegtfs.Route, len(pages))
	for i := range pages {
		routes[i] = pages[i].Route
	}
	return sitegtfs.Draw(routes)
}
