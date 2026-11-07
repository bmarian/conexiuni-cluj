package pitesti

import (
	"bytes"
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/feeds/news"
	"conexiuni-cluj/feeds/sitegtfs"
	"conexiuni-cluj/router"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"maps"
	"math"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

// The stops page embeds Publitrans's Google My Maps map of every stop; this is that map's KML export.
const myMapsKML = "https://www.google.com/maps/d/kml?forcekml=1&mid="

type marker struct {
	name string
	at   router.Point
}

var (
	// The page and the map write a terminus with or without "Cap (de) Linie" and a note in brackets, as in "Cap Linie Trivale I (Biserica)".
	terminus = regexp.MustCompile(`^cap (?:de )?linie `)
	note     = regexp.MustCompile(`\s*\([^)]*\)`)
	// A closing 1 or 2 is the side of the road, as in "Lanariei 1" and "Scoala Nr. 2-1".
	side = regexp.MustCompile(`[\s-][12]$`)
)

func stopMapURL(page []byte) (string, error) {
	doc, err := html.Parse(bytes.NewReader(page))
	if err != nil {
		return "", err
	}
	for _, frame := range news.FindAll(doc, news.WithTag("iframe")) {
		src, err := url.Parse(news.Attr(frame, "src"))
		if err == nil && strings.Contains(src.Path, "/maps/d/") && src.Query().Get("mid") != "" {
			return myMapsKML + url.QueryEscape(src.Query().Get("mid")), nil
		}
	}
	return "", fmt.Errorf("publitrans2000.ro: no stop map")
}

func parseStopMap(data []byte) ([]marker, error) {
	var out []marker
	dec := xml.NewDecoder(bytes.NewReader(data))
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("stop map: %w", err)
		}
		start, ok := tok.(xml.StartElement)
		if !ok || start.Name.Local != "Placemark" {
			continue
		}
		var p struct {
			Name        string `xml:"name"`
			Coordinates string `xml:"Point>coordinates"`
		}
		if err := dec.DecodeElement(&p, &start); err != nil {
			return nil, fmt.Errorf("stop map: %w", err)
		}
		lonLat := strings.Split(strings.TrimSpace(p.Coordinates), ",")
		if len(lonLat) < 2 {
			continue
		}
		lon, errLon := strconv.ParseFloat(lonLat[0], 64)
		lat, errLat := strconv.ParseFloat(lonLat[1], 64)
		if errLon == nil && errLat == nil {
			out = append(out, marker{name: strings.TrimSpace(p.Name), at: router.Point{Lat: lat, Lon: lon}})
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("stop map: no stops")
	}
	return out, nil
}

func exactKey(name string) string { return feeds.NormalizeStopLabel(name) }

func bareKey(name string) string {
	return terminus.ReplaceAllString(feeds.NormalizeStopLabel(note.ReplaceAllString(name, "")), "")
}

func stationKey(name string) string { return bareKey(stationName(name)) }

// The name the app shows, so both sides of a road make one station.
func stationName(name string) string { return strings.TrimSpace(side.ReplaceAllString(name, "")) }

type stopMap struct {
	markers []marker
	// Each marker's stop id: its name, numbered when another marker has it too.
	ids         []string
	exact, bare map[string][]int
	// The names the pages give, which take the markers named after them.
	named map[string]bool
}

func newStopMap(markers []marker, pages []linePage) *stopMap {
	m := &stopMap{markers: markers, exact: map[string][]int{}, bare: map[string][]int{}, named: map[string]bool{}}
	seen := map[string]int{}
	for i, mk := range markers {
		id := mk.name
		if seen[mk.name]++; seen[mk.name] > 1 {
			id += "#" + strconv.Itoa(seen[mk.name])
		}
		m.ids = append(m.ids, id)
		m.exact[exactKey(mk.name)] = append(m.exact[exactKey(mk.name)], i)
		m.bare[bareKey(mk.name)] = append(m.bare[bareKey(mk.name)], i)
	}
	for _, p := range pages {
		for _, w := range p.ways {
			for _, s := range w.stops {
				m.named[exactKey(s.Name)], m.named[bareKey(s.Name)] = true, true
			}
		}
	}
	return m
}

// The markers a page's stop may be: those of its name, else of its name without "Cap Linie" and notes, else its station's unnumbered one when no page takes it, as "Spitalul Militar" for "Spitalul Militar 2".
func (m *stopMap) candidates(name string) []int {
	if c := m.exact[exactKey(name)]; len(c) > 0 {
		return c
	}
	if c := m.bare[bareKey(name)]; len(c) > 0 {
		return c
	}
	var out []int
	for _, i := range m.bare[stationKey(name)] {
		if !m.named[exactKey(m.markers[i].name)] && !m.named[bareKey(m.markers[i].name)] {
			out = append(out, i)
		}
	}
	return out
}

// place puts every way's stops on the map; a name several markers share takes the one nearest the way's placed neighbours.
func place(pages []linePage, markers []marker) {
	m := newStopMap(markers, pages)
	placed, all := 0, 0
	unplaced := map[string]bool{}
	for _, p := range pages {
		for _, w := range p.ways {
			found := make([][]int, len(w.stops))
			for i, s := range w.stops {
				w.stops[i].Name = stationName(s.Name)
				if found[i] = m.candidates(s.Name); len(found[i]) == 1 {
					m.put(&w.stops[i], found[i][0])
				}
			}
			for i, c := range found {
				if len(c) > 1 {
					m.put(&w.stops[i], nearest(c, m.markers, w.stops, i))
				}
				if all++; w.stops[i].Placed {
					placed++
				} else {
					unplaced[w.stops[i].ID] = true
				}
			}
		}
	}
	log.Printf("pitesti: %d of %d stops on the stop map; not on it, so their routes skip them: %s", placed, all, strings.Join(slices.Sorted(maps.Keys(unplaced)), ", "))
}

func (m *stopMap) put(s *sitegtfs.Stop, i int) {
	s.ID, s.At, s.Placed = m.ids[i], m.markers[i].at, true
}

func nearest(candidates []int, markers []marker, stops []sitegtfs.Stop, at int) int {
	best, bestMetres := candidates[0], math.Inf(1)
	for _, c := range candidates {
		metres := 0.0
		for _, j := range neighbours(stops, at) {
			metres += markers[c].at.Meters(stops[j].At)
		}
		if metres < bestMetres {
			best, bestMetres = c, metres
		}
	}
	return best
}

// The nearest placed stop before and after.
func neighbours(stops []sitegtfs.Stop, at int) []int {
	var out []int
	for j := at - 1; j >= 0; j-- {
		if stops[j].Placed {
			out = append(out, j)
			break
		}
	}
	for j := at + 1; j < len(stops); j++ {
		if stops[j].Placed {
			out = append(out, j)
			break
		}
	}
	return out
}
