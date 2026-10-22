package suceava

import (
	"conexiuni-cluj/calendar"
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/feeds/news"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

// Every line page of tpl-sv.ro in one answer of the site's own WordPress API.
const linePagesURL = "https://tpl-sv.ro/wp-json/wp/v2/pages?search=linia&per_page=100&_fields=slug,content"

type page struct {
	Slug    string `json:"slug"`
	Content struct {
		Rendered string `json:"rendered"`
	} `json:"content"`
}

// A line as its page shows it.
type pageLine struct {
	short string
	// Station names per direction, the outward one first; a loop has one.
	directions [][]string
	// First-stop departures in minutes, by kind of day and direction.
	departures map[string][][]int
}

func fetchLines(url string) ([]pageLine, error) {
	data, err := feeds.Download(url)
	if err != nil {
		return nil, fmt.Errorf("tpl-sv.ro: %w", err)
	}
	var pages []page
	if err := json.Unmarshal(data, &pages); err != nil {
		return nil, fmt.Errorf("tpl-sv.ro: %w", err)
	}
	return parseLines(pages), nil
}

func parseLines(pages []page) []pageLine {
	var out []pageLine
	for _, p := range pages {
		short, ok := strings.CutPrefix(p.Slug, "linia-")
		if !ok {
			continue
		}
		doc, err := html.Parse(strings.NewReader(p.Content.Rendered))
		if err != nil {
			continue
		}
		line := pageLine{short: strings.ToUpper(short), directions: directions(doc)}
		if len(line.directions) == 0 {
			continue
		}
		line.departures = departures(doc, line.directions)
		out = append(out, line)
	}
	return out
}

// A loop's page lists its stations once, outside any section.
func directions(doc *html.Node) [][]string {
	var out [][]string
	for _, section := range news.FindAll(doc, news.WithClass("ts-route-section")) {
		if names := capsules(section); len(names) > 1 {
			out = append(out, names)
		}
	}
	if len(out) == 0 {
		if names := capsules(doc); len(names) > 1 {
			out = append(out, names)
		}
	}
	return out
}

func capsules(n *html.Node) []string {
	var out []string
	for _, c := range news.FindAll(n, news.WithClass("ts-capsule")) {
		if name := strings.Join(strings.Fields(news.Text(c)), " "); name != "" {
			out = append(out, name)
		}
	}
	return out
}

// Lists headed "Plecări <place>" under a day title; other lists (first and last runs, intervals) describe lines the zip has.
func departures(doc *html.Node, dirs [][]string) map[string][][]int {
	out := map[string][][]int{}
	var kinds []string
	sub := ""
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			switch {
			case news.WithClass("tpl-day-title")(c):
				kinds, sub = dayKinds(news.Text(c)), ""
			case news.WithClass("tpl-sub")(c):
				sub = news.Text(c)
			case news.WithTag("li")(c):
				label, times := listItem(c)
				if label == "" {
					label = sub
				}
				dir := directionOf(label, dirs)
				if dir < 0 || len(times) == 0 {
					continue
				}
				for _, kind := range kinds {
					if out[kind] == nil {
						out[kind] = make([][]int, len(dirs))
					}
					out[kind][dir] = append(out[kind][dir], times...)
				}
			default:
				walk(c)
			}
		}
	}
	walk(doc)
	for _, byDir := range out {
		for _, times := range byDir {
			slices.Sort(times)
		}
	}
	return out
}

func dayKinds(title string) []string {
	words := strings.Fields(feeds.NormalizeStopLabel(title))
	has := func(w string) bool { return slices.Contains(words, w) }
	switch {
	case has("luni") && has("duminica"):
		return []string{calendar.Weekday, calendar.Saturday, calendar.Sunday}
	case has("luni"):
		return []string{calendar.Weekday}
	case has("weekend"), has("sambata") && has("duminica"):
		return []string{calendar.Saturday, calendar.Sunday}
	case has("sambata"):
		return []string{calendar.Saturday}
	case has("duminica"):
		return []string{calendar.Sunday}
	}
	return nil
}

// The text before the first time badge, and every badge's time in minutes.
func listItem(li *html.Node) (string, []int) {
	var label strings.Builder
	var times []int
	for c := li.FirstChild; c != nil; c = c.NextSibling {
		if news.WithClass("tpl-badge")(c) {
			if m, ok := clockMinutes(news.Text(c)); ok {
				times = append(times, m)
			}
			continue
		}
		if len(times) == 0 {
			label.WriteString(news.Text(c) + " ")
		}
	}
	return strings.TrimSpace(label.String()), times
}

func clockMinutes(s string) (int, bool) {
	h, m, ok := strings.Cut(strings.TrimSpace(s), ":")
	hours, errH := strconv.Atoi(h)
	minutes, errM := strconv.Atoi(m)
	if !ok || errH != nil || errM != nil || minutes < 0 || minutes > 59 || hours < 0 {
		return 0, false
	}
	return hours*60 + minutes, true
}

// The page's own words around the place a list starts from.
var labelWords = []string{"plecari", "plecare", "din", "de", "la", "statia"}

// The direction whose station best matches the place a list names, nearest its start on a tie; -1 when none does or the list is not of departures.
func directionOf(label string, dirs [][]string) int {
	words := nameWords(label)
	if len(words) == 0 || !strings.HasPrefix(words[0].text, "plec") {
		return -1
	}
	words = slices.DeleteFunc(words, func(w word) bool { return slices.Contains(labelWords, w.text) })
	best, bestScore, bestPos := -1, 0, 0
	for d, names := range dirs {
		for i, name := range names {
			score := wordsShared(words, nameWords(name))
			if score > bestScore || (score == bestScore && score > 0 && i < bestPos) {
				best, bestScore, bestPos = d, score, i
			}
		}
	}
	return best
}
