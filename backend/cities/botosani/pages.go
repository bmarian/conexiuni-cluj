package botosani

import (
	"bytes"
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/feeds/news"
	"conexiuni-cluj/router"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

type pageLine struct {
	short   string
	markers []marker
	runs    []run
}

type run struct {
	label string
	names []string
}

type marker struct {
	id   string
	name string
	at   router.Point
}

const (
	outward = "TUR"
	inward  = "RETUR"
)

var (
	lineHeading = regexp.MustCompile(`^Traseul\s+(\S+)$`)
	mapData     = regexp.MustCompile(`window\.wpgmp\.mapdata(\d+)\s*=\s*"([^"]+)"`)
	stationSide = regexp.MustCompile(`\s+(?:-\s+)?\d$`)
)

func parsePage(page []byte, pageURL string) ([]pageLine, string, error) {
	maps, err := parseMaps(page)
	if err != nil {
		return nil, "", err
	}
	doc, err := html.Parse(bytes.NewReader(page))
	if err != nil {
		return nil, "", fmt.Errorf("eltransbt.ro: %w", err)
	}
	program, err := programLink(doc, pageURL)
	if err != nil {
		return nil, "", err
	}
	var out []pageLine
	var line *pageLine
	parts := news.FindAll(doc, func(n *html.Node) bool {
		return n.Type == html.ElementNode && (n.Data == "h2" || n.Data == "h3" || n.Data == "p" || news.Attr(n, "data-map-id") != "")
	})
	for _, n := range parts {
		switch {
		case n.Data == "h2" || n.Data == "h3":
			line = nil
			if m := lineHeading.FindStringSubmatch(news.Text(n)); m != nil {
				out = append(out, pageLine{short: m[1]})
				line = &out[len(out)-1]
			}
		case line == nil:
		case n.Data == "p":
			label, list, ok := strings.Cut(news.Text(n), ":")
			names := strings.Split(list, "–")
			if !ok || len(names) < 2 {
				continue
			}
			r := run{label: strings.ToUpper(strings.TrimSpace(label))}
			for _, name := range names {
				r.names = append(r.names, strings.TrimSpace(name))
			}
			line.runs = append(line.runs, r)
		default:
			line.markers = maps[news.Attr(n, "data-map-id")]
		}
	}
	return out, program, nil
}

// Not "Programul anual special de circulație", the holiday one.
func programLink(doc *html.Node, pageURL string) (string, error) {
	base, err := url.Parse(pageURL)
	if err != nil {
		return "", err
	}
	for _, a := range news.FindAll(doc, news.WithTag("a")) {
		href := news.Attr(a, "href")
		if strings.HasSuffix(strings.ToLower(href), ".pdf") && strings.Contains(nameKey(news.Text(a)), "programdecirculatie") {
			link, err := base.Parse(href)
			if err != nil {
				return "", err
			}
			return link.String(), nil
		}
	}
	return "", fmt.Errorf("eltransbt.ro: no timetable PDF on %s", pageURL)
}

func parseMaps(page []byte) (map[string][]marker, error) {
	out := map[string][]marker{}
	for _, m := range mapData.FindAllSubmatch(page, -1) {
		raw, err := base64.StdEncoding.DecodeString(string(m[2]))
		if err != nil {
			return nil, fmt.Errorf("eltransbt.ro: map %s: %w", m[1], err)
		}
		var data struct {
			Places []struct {
				ID       string `json:"id"`
				Title    string `json:"title"`
				Location struct {
					Lat string `json:"lat"`
					Lng string `json:"lng"`
				} `json:"location"`
			} `json:"places"`
		}
		if err := json.Unmarshal(raw, &data); err != nil {
			return nil, fmt.Errorf("eltransbt.ro: map %s: %w", m[1], err)
		}
		for _, p := range data.Places {
			lat, errLat := strconv.ParseFloat(p.Location.Lat, 64)
			lon, errLon := strconv.ParseFloat(p.Location.Lng, 64)
			if errLat != nil || errLon != nil {
				continue
			}
			name := strings.TrimSpace(p.Title)
			if first, rest, ok := strings.Cut(name, " "); ok && nameKey(first) == "statie" {
				name = rest
			}
			out[string(m[1])] = append(out[string(m[1])], marker{id: p.ID, name: stationSide.ReplaceAllString(name, ""), at: router.Point{Lat: lat, Lon: lon}})
		}
	}
	return out, nil
}

// The page writes "Electro Alfa" where the map has "Electroalfa".
func nameKey(name string) string {
	return strings.ReplaceAll(feeds.NormalizeStopLabel(name), " ", "")
}
