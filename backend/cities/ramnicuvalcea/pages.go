package ramnicuvalcea

import (
	"bytes"
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/feeds/news"
	"conexiuni-cluj/feeds/sitegtfs"
	"conexiuni-cluj/router"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// A line in the index: its number and its page.
type link struct{ url, name string }

// A line's stops each way, in the order its page lists them.
type ways [2][]sitegtfs.Stop

// One run of a line at one stop: ETA's trip id and the minute of the day it leaves or, at its last stop, arrives.
type passing struct {
	stop   string
	line   string
	way    int
	days   []string
	trip   string
	minute int
	only   sitegtfs.Limit
}

var (
	updatedOn = regexp.MustCompile(`Ultima actualizare a programului:\s*([^<]+)`)
	// Each marker of the stops map follows the link to its stop's page.
	mapStop = regexp.MustCompile(`link_statie\.href = '/s/(\d+)';[^}]*?position: \{lat: (-?[\d.]+), lng: (-?[\d.]+)\}`)
)

// The line pages in the index's order.
func parseIndex(data []byte, indexURL string) ([]link, error) {
	doc, err := html.Parse(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("eta-bus.ro: %w", err)
	}
	base, err := url.Parse(indexURL)
	if err != nil {
		return nil, err
	}
	var out []link
	seen := map[string]bool{}
	for _, a := range news.FindAll(doc, func(n *html.Node) bool { return news.WithTag("a")(n) && news.Attr(n, "data-route_id") != "" }) {
		href, err := base.Parse(news.Attr(a, "href"))
		name := news.Text(news.First(a, news.WithClass("card-title")))
		if err != nil || name == "" || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, link{url: href.String(), name: name})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("eta-bus.ro: no lines on %s", indexURL)
	}
	return out, nil
}

// The day the whole timetable was last changed, as a line's timetable page prints it; empty when the page does not say.
func parseUpdated(data []byte) string {
	if m := updatedOn.FindSubmatch(data); m != nil {
		return strings.TrimSpace(string(m[1]))
	}
	return ""
}

func parseMap(data []byte) map[string]router.Point {
	out := map[string]router.Point{}
	for _, m := range mapStop.FindAllSubmatch(data, -1) {
		lat, errLat := strconv.ParseFloat(string(m[2]), 64)
		lon, errLon := strconv.ParseFloat(string(m[3]), 64)
		if errLat == nil && errLon == nil {
			out[string(m[1])] = router.Point{Lat: lat, Lon: lon}
		}
	}
	return out
}

func parseLine(data []byte) (ways, error) {
	var out ways
	doc, err := html.Parse(bytes.NewReader(data))
	if err != nil {
		return out, err
	}
	for _, table := range news.FindAll(doc, news.WithTag("table")) {
		w := way(table)
		if w < 0 {
			continue
		}
		for _, a := range news.FindAll(table, news.WithClass("statie-link")) {
			out[w] = append(out[w], sitegtfs.Stop{ID: news.Attr(a, "data-stop_id"), Name: news.Text(a)})
		}
	}
	if len(out[0]) < 2 {
		return out, fmt.Errorf("no stops")
	}
	return out, nil
}

// Every line's runs at a stop, read from the stop's page with arrivals on.
func parseStop(data []byte) ([]passing, error) {
	doc, err := html.Parse(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	legend := map[string]sitegtfs.Limit{}
	for _, card := range news.FindAll(doc, news.WithClass("info-colors")) {
		for _, p := range news.FindAll(card, news.WithTag("p")) {
			mark := news.First(p, func(n *html.Node) bool { return news.Attr(n, "data-block_id") != "" })
			if only, ok := sitegtfs.LimitOf(news.Text(p)); mark != nil && ok {
				legend[news.Attr(mark, "data-block_id")] = only
			}
		}
	}
	var out []passing
	for _, card := range news.FindAll(doc, news.WithClass("program")) {
		head := news.First(card, news.WithTag("h4"))
		if head == nil {
			continue
		}
		days := feeds.KindsOfDay(news.Text(head))
		slices.Sort(days)
		for _, row := range news.FindAll(card, news.WithClass("plecari")) {
			w := way(row)
			line := news.Text(news.First(row, news.WithClass("traseu-link")))
			if w < 0 || line == "" {
				continue
			}
			for _, span := range news.FindAll(row, func(n *html.Node) bool { return news.Attr(n, "data-trip_id") != "" }) {
				at, err := time.Parse("15:04", news.Text(span))
				if err != nil {
					return nil, fmt.Errorf("line %s: %w", line, err)
				}
				p := passing{stop: news.Attr(row, "data-stop_id"), line: line, way: w, days: days, trip: news.Attr(span, "data-trip_id"), minute: at.Hour()*60 + at.Minute()}
				if block := news.Attr(span, "data-block_id"); block != "" {
					only, ok := legend[block]
					if !ok {
						return nil, fmt.Errorf("no legend for %q", block)
					}
					p.only = only
				}
				out = append(out, p)
			}
		}
	}
	return out, nil
}

// ETA marks each direction's stops and runs with a class "direction-0" or "direction-1"; -1 for neither.
func way(n *html.Node) int {
	for _, class := range strings.Fields(news.Attr(n, "class")) {
		if d, ok := strings.CutPrefix(class, "direction-"); ok {
			if w, err := strconv.Atoi(d); err == nil && w >= 0 && w < len(ways{}) {
				return w
			}
		}
	}
	return -1
}
