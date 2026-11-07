package albaiulia

import (
	"bytes"
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/feeds/news"
	"conexiuni-cluj/router"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// One of STP's timetable pages: a single route, one way.
type page struct {
	code, label string
	stops       []stop
	// Minutes from the first stop to each.
	minutes    []int
	departures []departure
}

type stop struct {
	id, name string
	at       router.Point
	placed   bool
}

type departure struct {
	day    string
	minute int
	only   limit
}

// The days a page's legend keeps a marked departure to; empty for every day of its kind.
type limit string

const (
	term     limit = "term"
	holidays limit = "holidays"
)

type link struct{ url, code, label string }

var (
	updatedOn   = regexp.MustCompile(`Orarul a fost actualizat la:\s*(?:<[^>]*>)?\s*(\d{2}-\d{2}-\d{4})`)
	stopMinutes = regexp.MustCompile(`dpcjs=\[([^\]]*)\]`)
	mapStop     = regexp.MustCompile(`\['((?:[^'\\]|\\.)*)',\s*(-?[\d.]+),\s*(-?[\d.]+),\s*\d+\]`)
	departureAt = regexp.MustCompile(`modifOreStatii\('(\d{1,2}):(\d{2})'\)`)
	weekdays    = map[string]time.Weekday{"luni": time.Monday, "marti": time.Tuesday, "miercuri": time.Wednesday, "joi": time.Thursday, "vineri": time.Friday, "sambata": time.Saturday, "duminica": time.Sunday}
)

// The line pages in the index's order, and the day the timetable was last changed, empty when the index does not say.
func parseIndex(data []byte, indexURL string) ([]link, string, error) {
	doc, err := html.Parse(bytes.NewReader(data))
	if err != nil {
		return nil, "", fmt.Errorf("stpalba.ro: %w", err)
	}
	base, err := url.Parse(indexURL)
	if err != nil {
		return nil, "", err
	}
	var out []link
	seen := map[string]bool{}
	for _, a := range news.FindAll(doc, news.WithTag("a")) {
		href, err := base.Parse(news.Attr(a, "href"))
		if err != nil || href.Path != base.Path {
			continue
		}
		if code := href.Query().Get("linie"); code != "" && !seen[code] {
			seen[code] = true
			out = append(out, link{url: href.String(), code: code, label: news.Text(a)})
		}
	}
	if len(out) == 0 {
		return nil, "", fmt.Errorf("stpalba.ro: no lines on %s", indexURL)
	}
	updated := ""
	if m := updatedOn.FindSubmatch(data); m != nil {
		updated = string(m[1])
	}
	return out, updated, nil
}

func parsePage(data []byte, l link) (page, error) {
	doc, err := html.Parse(bytes.NewReader(data))
	if err != nil {
		return page{}, err
	}
	p := page{code: l.code, label: l.label}
	for _, a := range news.FindAll(doc, news.WithTag("a")) {
		href, err := url.Parse(news.Attr(a, "href"))
		if err != nil {
			continue
		}
		if id := href.Query().Get("statia_curenta"); id != "" {
			p.stops = append(p.stops, stop{id: id, name: news.Text(a)})
		}
	}
	m := stopMinutes.FindSubmatch(data)
	if len(p.stops) < 2 || m == nil {
		return page{}, fmt.Errorf("no stops")
	}
	for _, field := range strings.Split(string(m[1]), ",") {
		minute, err := strconv.Atoi(strings.TrimSpace(field))
		if err != nil {
			return page{}, fmt.Errorf("minutes %q: %w", m[1], err)
		}
		p.minutes = append(p.minutes, minute)
	}
	if len(p.minutes) < len(p.stops) {
		return page{}, fmt.Errorf("%d stops but minutes for %d", len(p.stops), len(p.minutes))
	}
	p.minutes = p.minutes[:len(p.stops)]
	place(p.stops, data)
	p.departures, err = departures(doc)
	return p, err
}

// The map lists the stops in order but skips some, so each takes the next marker of its own name.
func place(stops []stop, data []byte) {
	next := 0
	markers := mapStop.FindAllSubmatch(data, -1)
	for i := range stops {
		for j := next; j < len(markers); j++ {
			if string(markers[j][1]) != stops[i].name {
				continue
			}
			lat, errLat := strconv.ParseFloat(string(markers[j][2]), 64)
			lon, errLon := strconv.ParseFloat(string(markers[j][3]), 64)
			if errLat == nil && errLon == nil {
				stops[i].at, stops[i].placed = router.Point{Lat: lat, Lon: lon}, true
			}
			next = j + 1
			break
		}
	}
}

func departures(doc *html.Node) ([]departure, error) {
	table := news.First(doc, news.WithClass("tabel_ore_plecare"))
	if table == nil {
		return nil, fmt.Errorf("no departures table")
	}
	legend := map[string]limit{}
	for _, p := range news.FindAll(doc, news.WithTag("p")) {
		text := news.Text(p)
		wording := strings.TrimLeft(text, "*")
		if wording == text {
			continue
		}
		if only, ok := limitOf(wording); ok {
			legend[text[:len(text)-len(wording)]] = only
		}
	}
	var days []string
	var out []departure
	for _, row := range news.FindAll(table, news.WithTag("tr")) {
		if heads := news.FindAll(row, news.WithTag("th")); len(heads) > 0 {
			days = days[:0]
			for _, th := range heads {
				day := ""
				if kinds := feeds.KindsOfDay(news.Text(th)); len(kinds) == 1 {
					day = kinds[0]
				}
				days = append(days, day)
			}
			continue
		}
		for i, td := range news.FindAll(row, news.WithTag("td")) {
			if i >= len(days) || days[i] == "" {
				continue
			}
			for _, span := range news.FindAll(td, news.WithClass("link_click_ora")) {
				m := departureAt.FindStringSubmatch(news.Attr(span, "onclick"))
				if m == nil {
					continue
				}
				h, _ := strconv.Atoi(m[1])
				minute, _ := strconv.Atoi(m[2])
				d := departure{day: days[i], minute: h*60 + minute}
				text := news.Text(span)
				if marks := text[len(strings.TrimRight(text, "*")):]; marks != "" {
					only, ok := legend[marks]
					if !ok {
						return nil, fmt.Errorf("no legend for %q", marks)
					}
					d.only = only
				}
				out = append(out, d)
			}
		}
	}
	return out, nil
}

// Read from the legend's wording: the school terms, the school holidays, or one weekday.
func limitOf(legend string) (limit, bool) {
	text := feeds.NormalizeStopLabel(legend)
	switch {
	case strings.Contains(text, "vacant"):
		return holidays, true
	case strings.Contains(text, "scol"):
		return term, true
	}
	for word, day := range weekdays {
		if strings.Contains(text, word) {
			return limit(day.String()), true
		}
	}
	return "", false
}

func (l limit) allows(day time.Time, school feeds.School) bool {
	switch l {
	case "":
		return true
	case term:
		return school.SchoolDay(day)
	case holidays:
		return !school.SchoolDay(day)
	}
	return day.Weekday().String() == string(l)
}
