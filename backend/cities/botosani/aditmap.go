package botosani

import (
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/router"
	"encoding/json"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
)

const aditSource = "aditmbt.ro"

type mapLine struct {
	name   string
	points []router.Point
	stops  []marker
	// stops[k] is points[at[k]].
	at []int
}

// MetroDreamin's default.
const unnamedStation = "Station Name"

var (
	mapFrame    = regexp.MustCompile(`src="(https://metrodreamin\.com/view/[^"]+)"`)
	nextData    = regexp.MustCompile(`(?s)<script id="__NEXT_DATA__"[^>]*>(.*?)</script>`)
	mapLineCode = regexp.MustCompile(`^([A-Z]+\d+)(?:_|\s|$)`)
)

func (s *static) aditMap() (aditMap, error) {
	page, err := feeds.Download(s.mapPage)
	if err != nil {
		return aditMap{}, fmt.Errorf("aditmbt.ro: %w", err)
	}
	link, err := mapLink(page)
	if err != nil {
		return aditMap{}, err
	}
	system, err := feeds.Download(link)
	if err != nil {
		return aditMap{}, fmt.Errorf("metrodreamin.com: %w", err)
	}
	return parseMap(system)
}

func mapLink(page []byte) (string, error) {
	m := mapFrame.FindSubmatch(page)
	if m == nil {
		return "", fmt.Errorf("aditmbt.ro: no MetroDreamin map on the page")
	}
	return string(m[1]), nil
}

type aditMap struct {
	// By line number, the piece with the most stations.
	lines  map[string]mapLine
	pieces []mapLine
}

func parseMap(page []byte) (aditMap, error) {
	m := nextData.FindSubmatch(page)
	if m == nil {
		return aditMap{}, fmt.Errorf("metrodreamin.com: no system data on the page")
	}
	var data struct {
		Props struct {
			PageProps struct {
				FullSystem struct {
					Map struct {
						Stations map[string]struct {
							Name       string  `json:"name"`
							Lat        float64 `json:"lat"`
							Lng        float64 `json:"lng"`
							IsWaypoint bool    `json:"isWaypoint"`
						} `json:"stations"`
						Lines map[string]struct {
							Name       string   `json:"name"`
							StationIDs []string `json:"stationIds"`
						} `json:"lines"`
					} `json:"map"`
				} `json:"fullSystem"`
			} `json:"pageProps"`
		} `json:"props"`
	}
	if err := json.Unmarshal(m[1], &data); err != nil {
		return aditMap{}, fmt.Errorf("metrodreamin.com: %w", err)
	}
	system := data.Props.PageProps.FullSystem.Map
	out := aditMap{lines: map[string]mapLine{}}
	for _, key := range slices.Sorted(maps.Keys(system.Lines)) {
		l := system.Lines[key]
		code := mapLineCode.FindStringSubmatch(l.Name)
		if code == nil {
			continue
		}
		line := mapLine{name: key}
		for _, id := range l.StationIDs {
			s, ok := system.Stations[id]
			if !ok {
				continue
			}
			if !s.IsWaypoint {
				name := strings.Join(strings.Fields(s.Name), " ")
				line.stops = append(line.stops, marker{id: id, source: aditSource, name: name, at: router.Point{Lat: s.Lat, Lon: s.Lng}})
				line.at = append(line.at, len(line.points))
			}
			line.points = append(line.points, router.Point{Lat: s.Lat, Lon: s.Lng})
		}
		if len(line.stops) == 0 {
			continue
		}
		out.pieces = append(out.pieces, line)
		short := code[1]
		if kept, ok := out.lines[short]; ok && len(kept.stops) >= len(line.stops) {
			continue
		}
		out.lines[short] = line
	}
	return out, nil
}

func (line mapLine) keep(stop func(k int) bool) mapLine {
	out := mapLine{name: line.name, points: line.points}
	for k, m := range line.stops {
		if stop(k) {
			out.stops = append(out.stops, m)
			out.at = append(out.at, line.at[k])
		}
	}
	return out
}

func backwards(line mapLine) mapLine {
	out := mapLine{name: line.name, points: slices.Clone(line.points), stops: slices.Clone(line.stops), at: slices.Clone(line.at)}
	slices.Reverse(out.points)
	slices.Reverse(out.stops)
	slices.Reverse(out.at)
	for k := range out.at {
		out.at[k] = len(out.points) - 1 - out.at[k]
	}
	return out
}

func townFirst(line mapLine, fromTown func(marker) float64) mapLine {
	if fromTown(line.stops[0]) <= fromTown(line.stops[len(line.stops)-1]) {
		return line
	}
	return backwards(line)
}
