package pitesti

import (
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/feeds/sitegtfs"
	"log"
	"time"
)

// site reads Publitrans's line pages and stop map; the pages show no timetable date, so every refresh reads them again.
type site struct {
	indexes []string
	stops   string
	school  feeds.School
}

func (s *site) zip() ([]byte, error) {
	var links []link
	seen := map[string]bool{}
	for _, index := range s.indexes {
		data, err := feeds.DownloadInTurn(index)
		if err != nil {
			return nil, err
		}
		found, err := parseIndex(data, index)
		if err != nil {
			return nil, err
		}
		for _, l := range found {
			if !seen[l.id] {
				seen[l.id] = true
				links = append(links, l)
			}
		}
	}
	var pages []linePage
	for _, l := range links {
		data, err := feeds.DownloadInTurn(l.url)
		if err != nil {
			return nil, err
		}
		p, err := parsePage(data, l)
		if err != nil {
			log.Printf("pitesti: line %s: %v; the line is left out", l.name, err)
			continue
		}
		pages = append(pages, p)
	}
	markers, err := s.stopMap()
	if err != nil {
		return nil, err
	}
	place(pages, markers)
	lines, shapes := network(pages)
	return sitegtfs.Zip(lines, shapes, s.school, time.Now())
}

func (s *site) stopMap() ([]marker, error) {
	page, err := feeds.DownloadInTurn(s.stops)
	if err != nil {
		return nil, err
	}
	kml, err := stopMapURL(page)
	if err != nil {
		return nil, err
	}
	data, err := feeds.DownloadInTurn(kml)
	if err != nil {
		return nil, err
	}
	return parseStopMap(data)
}
