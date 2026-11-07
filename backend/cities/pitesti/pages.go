package pitesti

import (
	"bytes"
	"conexiuni-cluj/calendar"
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/feeds/news"
	"conexiuni-cluj/feeds/sitegtfs"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

// A line on one of Publitrans's indexes; id is the page's own, as in "1_2.php?id=20".
type link struct{ url, id, name string }

// One of Publitrans's line pages: the way out and the way back, or a single circuit.
type linePage struct {
	link
	ways []way
}

// One way as the page lists it.
type way struct {
	stops []sitegtfs.Stop
	// Each run's minute at each stop by kind of day, 24+ after midnight; the way out has none at its last stop, where the page lists the way back's departures.
	runs map[string][][]int
}

const (
	dayHeading = "Program"
	// The page's word for a day without runs ("nu circulă").
	noRuns        = "NU CI"
	minutesPerDay = 24 * 60
)

var (
	lineName  = regexp.MustCompile(`^Linia\s+(\S+)$`)
	stopTitle = regexp.MustCompile(`^\d+\.\s*Statia:\s*(.+)$`)
	clockTime = regexp.MustCompile(`^(\d{1,2}):(\d{2})$`)
)

// The line pages an index lists, in its order.
func parseIndex(data []byte, indexURL string) ([]link, error) {
	doc, err := html.Parse(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("publitrans2000.ro: %w", err)
	}
	base, err := url.Parse(indexURL)
	if err != nil {
		return nil, err
	}
	var out []link
	for _, row := range news.FindAll(doc, news.WithTag("tr")) {
		name := ""
		for _, td := range news.FindAll(row, news.WithTag("td")) {
			if m := lineName.FindStringSubmatch(news.Text(td)); m != nil {
				name = m[1]
			}
		}
		a := news.First(row, news.WithTag("a"))
		if name == "" || a == nil {
			continue
		}
		href, err := base.Parse(news.Attr(a, "href"))
		if err != nil {
			continue
		}
		if id := href.Query().Get("id"); id != "" {
			out = append(out, link{url: href.String(), id: id, name: name})
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("publitrans2000.ro: no lines on %s", indexURL)
	}
	return out, nil
}

func parsePage(data []byte, l link) (linePage, error) {
	doc, err := html.Parse(bytes.NewReader(data))
	if err != nil {
		return linePage{}, err
	}
	p := linePage{link: l}
	out, back := news.First(doc, withID("myForm1")), news.First(doc, withID("myForm2"))
	for _, form := range []*html.Node{out, back} {
		if len(news.FindAll(form, news.WithClass("collapsible1"))) == 0 {
			continue
		}
		w, err := parseWay(form, form == out && back != nil)
		if err != nil {
			return linePage{}, err
		}
		p.ways = append(p.ways, w)
	}
	if len(p.ways) == 0 {
		return linePage{}, fmt.Errorf("no stops")
	}
	return p, nil
}

func withID(id string) func(*html.Node) bool {
	return func(n *html.Node) bool { return n.Type == html.ElementNode && news.Attr(n, "id") == id }
}

// Each stop is a button with its name and the block after it, which lists its times under a heading per kind of day.
func parseWay(form *html.Node, untimedEnd bool) (way, error) {
	titles, blocks := news.FindAll(form, news.WithClass("collapsible1")), news.FindAll(form, news.WithClass("content1"))
	if len(titles) != len(blocks) {
		return way{}, fmt.Errorf("%d stops but %d timetables", len(titles), len(blocks))
	}
	w := way{runs: map[string][][]int{}}
	byStop := map[string][][]int{}
	for i, title := range titles {
		m := stopTitle.FindStringSubmatch(news.Text(title))
		if m == nil {
			return way{}, fmt.Errorf("stop %q", news.Text(title))
		}
		name := strings.TrimSpace(m[1])
		w.stops = append(w.stops, sitegtfs.Stop{ID: name, Name: name})
		if untimedEnd && i == len(titles)-1 {
			break
		}
		times, err := stopTimes(blocks[i])
		if err != nil {
			return way{}, fmt.Errorf("%s: %w", name, err)
		}
		for _, day := range calendar.ScheduleDays {
			byStop[day] = append(byStop[day], times[day])
		}
	}
	for day, lists := range byStop {
		for _, list := range lists {
			if len(list) != len(lists[0]) {
				return way{}, fmt.Errorf("%s: stops with %d and %d runs", day, len(lists[0]), len(list))
			}
		}
		for k := range lists[0] {
			run := make([]int, len(lists))
			for i, list := range lists {
				run[i] = list[k]
			}
			w.runs[day] = append(w.runs[day], run)
		}
	}
	return w, nil
}

// A stop's times by kind of day in the order its runs pass, those after midnight past 24:00.
func stopTimes(block *html.Node) (map[string][]int, error) {
	out := map[string][]int{}
	day := ""
	for c := block.FirstChild; c != nil; c = c.NextSibling {
		if news.WithClass("txt_slogan")(c) {
			day = kindOf(strings.TrimPrefix(news.Text(c), dayHeading))
			continue
		}
		if c.Type != html.TextNode || day == "" {
			continue
		}
		for _, field := range strings.Split(c.Data, ";") {
			field = strings.TrimSpace(field)
			if field == "" || field == noRuns {
				continue
			}
			m := clockTime.FindStringSubmatch(field)
			if m == nil {
				return nil, fmt.Errorf("time %q", field)
			}
			h, _ := strconv.Atoi(m[1])
			minute, _ := strconv.Atoi(m[2])
			at := h*60 + minute
			if list := out[day]; len(list) > 0 {
				for at < list[len(list)-1] {
					at += minutesPerDay
				}
			}
			out[day] = append(out[day], at)
		}
	}
	return out, nil
}

func kindOf(heading string) string {
	if kinds := feeds.KindsOfDay(heading); len(kinds) == 1 {
		return kinds[0]
	}
	return ""
}
