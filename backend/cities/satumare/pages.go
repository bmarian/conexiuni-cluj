package satumare

import (
	"bytes"
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/feeds/news"
	"fmt"
	"net/url"

	"golang.org/x/net/html"
)

// One route on a line page ("LINIA 10A: Ambudului » Spicului Wilburg"): the departures from each end, tur first.
type route struct {
	label string
	from  [2]string
	times [2]map[string][]string
}

type link struct{ line, url string }

// The line pages in the index's order.
func parseIndex(data []byte, indexURL string) ([]link, error) {
	doc, err := html.Parse(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("tusm.ro: %w", err)
	}
	base, err := url.Parse(indexURL)
	if err != nil {
		return nil, err
	}
	var out []link
	seen := map[string]bool{}
	for _, a := range news.FindAll(doc, news.WithTag("a")) {
		href, err := base.Parse(news.Attr(a, "href"))
		if err != nil || href.Path != base.Path {
			continue
		}
		if line := href.Query().Get("linia"); line != "" && !seen[line] {
			seen[line] = true
			href.RawQuery = url.Values{"linia": {line}}.Encode()
			out = append(out, link{line: line, url: href.String()})
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("tusm.ro: no lines on %s", indexURL)
	}
	return out, nil
}

// Every route a line page shows: the line itself, then its variants.
func parsePage(data []byte) ([]route, error) {
	doc, err := html.Parse(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("tusm.ro: %w", err)
	}
	header, slots := news.WithClass("station-timetable"), news.WithClass("station_with_slots")
	var out []route
	for _, box := range news.FindAll(doc, routeBox) {
		r := route{label: news.Text(news.First(box, news.WithClass("selected_line")))}
		for _, holder := range news.FindAll(box, news.WithClass("timetables_list_holder")) {
			way := 0
			if news.WithClass("direction-retur")(holder) {
				way = 1
			}
			r.times[way] = map[string][]string{}
			var kinds []string
			for _, block := range news.FindAll(holder, func(n *html.Node) bool { return header(n) || slots(n) }) {
				if header(block) {
					kinds = feeds.KindsOfDay(news.Text(news.First(block, news.WithClass("station_timetable_title"))))
					r.from[way] = news.Text(news.First(block, news.WithTag("b")))
					continue
				}
				for _, at := range news.FindAll(block, news.WithClass("date_time")) {
					for _, kind := range kinds {
						r.times[way][kind] = append(r.times[way][kind], news.Text(at))
					}
				}
			}
		}
		out = append(out, r)
	}
	return out, nil
}

// The line and each of its variants sit in a box of their own.
func routeBox(n *html.Node) bool {
	return news.WithClass("box_holder")(n) && news.Attr(n, "data-busline-id") != ""
}
