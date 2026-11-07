package albaiulia

import (
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/feeds/roads"
	"conexiuni-cluj/models"
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
	pages   []page
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
	if s.pages == nil || updated == "" || updated != s.updated {
		pages, err := s.crawl(links)
		if err != nil {
			return nil, err
		}
		s.pages, s.shapes, s.updated = pages, drawn(pages), updated
	}
	return writeZip(s.pages, s.shapes, s.school, time.Now())
}

// One page at a time, each after a pause as long as the last one took to answer.
func (s *site) crawl(links []link) ([]page, error) {
	var out []page
	for _, l := range links {
		start := time.Now()
		data, err := feeds.Download(l.url)
		if err != nil {
			return nil, fmt.Errorf("stpalba.ro: %w", err)
		}
		time.Sleep(time.Since(start))
		p, err := parsePage(data, l)
		if err != nil {
			log.Printf("albaiulia: %s: %v; the route is left out", l.code, err)
			continue
		}
		out = append(out, p)
	}
	placeEverywhere(out)
	return out, nil
}

// A stop the map of one page skips takes its place from another page's map.
func placeEverywhere(pages []page) {
	at := map[string]router.Point{}
	for _, p := range pages {
		for _, s := range p.stops {
			if s.placed {
				at[s.id] = s.at
			}
		}
	}
	for _, p := range pages {
		for i, s := range p.stops {
			if pt, ok := at[s.id]; ok && !s.placed {
				p.stops[i].at, p.stops[i].placed = pt, true
			} else if !ok {
				log.Printf("albaiulia: %s: no page places %s, so the route skips it", p.code, s.name)
			}
		}
	}
}

// Each route on the roads from stop to stop; straight legs without the OSM extract.
func drawn(pages []page) map[string][]router.Point {
	var stops []models.Stop
	for _, p := range pages {
		for _, s := range p.stops {
			if s.placed {
				stops = append(stops, models.Stop{StopLat: s.at.Lat, StopLon: s.at.Lon})
			}
		}
	}
	g, _, err := roads.Around(stops)
	if err != nil {
		log.Printf("albaiulia: %v; the lines get straight legs", err)
	}
	out := make(map[string][]router.Point, len(pages))
	for _, p := range pages {
		var points []router.Point
		for _, s := range p.stops {
			if s.placed {
				points = append(points, s.at)
			}
		}
		out[p.code] = roads.Through(points, func(a, b router.Point) []router.Point { return roads.Path(g, a, b) })
	}
	return out
}
