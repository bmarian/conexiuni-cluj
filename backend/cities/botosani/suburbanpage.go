package botosani

import (
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/feeds/news"
	"log"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

type suburbanTimes struct {
	short   string
	follows string
	title   []string
	ends    []suburbanEnd
	// One for both ways or one per end.
	minutes []int
}

type suburbanEnd struct {
	place string
	hub   string
	days  map[string][]timed
	noted map[string][]timed
}

type timed struct {
	minute int
	term   bool
}

var (
	suburbanHeading = regexp.MustCompile(`^LINIA\s+([A-Z]+\d+)(\**)_(.+)$`)
	endHeading      = regexp.MustCompile(`^Orar plecare din\s+(.+?)\s*(?:\(\s*(?:Nod\s+)?(.+?)\s*\))?\s*:?$`)
	// "**cursa de la ora *20:15 va avea indicativul BT07** și următorul traseu Botoșani–Zăicești–…".
	runNote = regexp.MustCompile(`indicativul\s+([A-Z]+\d+)\**\s+și\s+următorul\s+traseu\s+(.+?)\.?$`)
	// *hh:mm runs in term only; hh:mm** is another line's run.
	listedTime  = regexp.MustCompile(`(\*?)(\d{1,2}):(\d{2})(\**)`)
	tripMinutes = regexp.MustCompile(`(\d+)\s*(?:de\s+)?minute`)
)

// For "LINIA MT08**_…".
const secondRoute = "B"

func places(route string) []string {
	var out []string
	for _, place := range strings.Split(route, "–") {
		out = append(out, strings.TrimSpace(place))
	}
	return out
}

func parseSuburban(doc *html.Node) []suburbanTimes {
	var out []suburbanTimes
	current := -1
	for _, n := range news.FindAll(doc, func(n *html.Node) bool {
		return n.Type == html.ElementNode && (n.Data == "p" || n.Data == "li")
	}) {
		text := strings.Join(strings.Fields(news.Text(n)), " ")
		if m := suburbanHeading.FindStringSubmatch(text); m != nil {
			t := suburbanTimes{short: m[1], title: places(m[3])}
			if m[2] != "" {
				t.short, t.follows = m[1]+secondRoute, m[1]
			}
			out = append(out, t)
			current = len(out) - 1
			continue
		}
		if current < 0 {
			continue
		}
		line := &out[current]
		if m := endHeading.FindStringSubmatch(text); m != nil {
			end := suburbanEnd{place: m[1], hub: m[2], days: map[string][]timed{}, noted: map[string][]timed{}}
			line.ends = append(line.ends, end)
			continue
		}
		if m := runNote.FindStringSubmatch(text); m != nil && len(line.ends) > 0 {
			out = noteRuns(out, current, m[1], places(m[2]))
			continue
		}
		label, rest, ok := strings.Cut(text, ":")
		if !ok {
			continue
		}
		if nameKey(label) == "duratacalatoriei" {
			first, _, _ := strings.Cut(rest, ". ")
			for _, m := range tripMinutes.FindAllStringSubmatch(first, -1) {
				minutes, _ := strconv.Atoi(m[1])
				line.minutes = append(line.minutes, minutes)
			}
			continue
		}
		kinds := feeds.KindsOfDay(label)
		if len(kinds) == 0 || len(line.ends) == 0 {
			continue
		}
		end := &line.ends[len(line.ends)-1]
		for _, m := range listedTime.FindAllStringSubmatch(rest, -1) {
			h, _ := strconv.Atoi(m[2])
			minute, _ := strconv.Atoi(m[3])
			for _, day := range kinds {
				d := timed{minute: h*60 + minute, term: m[1] != ""}
				if m[4] != "" {
					end.noted[day] = append(end.noted[day], d)
				} else {
					end.days[day] = append(end.days[day], d)
				}
			}
		}
	}
	for _, t := range out {
		for _, e := range t.ends {
			if len(e.noted) > 0 {
				log.Printf("botosani: %s: runs from %s marked for another line, with no note naming it", t.short, e.place)
			}
		}
	}
	return out
}

func noteRuns(out []suburbanTimes, current int, short string, route []string) []suburbanTimes {
	from := &out[current].ends[len(out[current].ends)-1]
	noted := from.noted
	from.noted = map[string][]timed{}
	town := slices.IndexFunc(out[current].ends, func(e suburbanEnd) bool { return e.hub != "" })
	if town < 0 {
		log.Printf("botosani: %s: a note under %s, which has no end in town", short, out[current].short)
		return out
	}
	townPlace := out[current].ends[town]
	if nameKey(route[0]) != nameKey(townPlace.place) {
		route = slices.Clone(route)
		slices.Reverse(route)
	}
	at := slices.IndexFunc(out, func(t suburbanTimes) bool { return t.short == short })
	if at < 0 {
		follows := out[current].short
		if out[current].follows != "" {
			follows = out[current].follows
		}
		out = append(out, suburbanTimes{short: short, follows: follows, title: route, ends: []suburbanEnd{
			{place: route[0], hub: townPlace.hub, days: map[string][]timed{}},
			{place: route[len(route)-1], days: map[string][]timed{}},
		}})
		at = len(out) - 1
	}
	end := slices.IndexFunc(out[at].ends, func(e suburbanEnd) bool { return nameKey(e.place) == nameKey(from.place) })
	if end < 0 {
		log.Printf("botosani: %s's runs noted at %s, not an end of its route", short, from.place)
		return out
	}
	for day, runs := range noted {
		out[at].ends[end].days[day] = append(out[at].ends[end].days[day], runs...)
	}
	return out
}
