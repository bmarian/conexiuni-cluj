package ramnicuvalcea

import (
	"bytes"
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/feeds/news"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// The months as eta-bus.ro shortens them, in Romanian.
var months = map[string]time.Month{
	"ian": time.January, "feb": time.February, "mar": time.March, "apr": time.April, "mai": time.May, "iun": time.June,
	"iul": time.July, "aug": time.August, "sep": time.September, "oct": time.October, "noi": time.November, "dec": time.December,
}

// eta-bus.ro/comunicate, each post an article dated like "15 Sep 2026" or " 6 Iul 2026".
func parseNews(page []byte, base *url.URL, loc *time.Location) ([]feeds.NewsItem, error) {
	doc, err := html.Parse(bytes.NewReader(page))
	if err != nil {
		return nil, err
	}
	var items []feeds.NewsItem
	for _, n := range news.FindAll(doc, news.WithTag("article")) {
		link := news.First(news.First(n, news.WithClass("entry-title")), news.WithTag("a"))
		if it, ok := news.Item(news.Text(link), news.Attr(link, "href"), postDay(news.Attr(n, "data-date"), loc), base); ok {
			items = append(items, it)
		}
	}
	return items, nil
}

func postDay(text string, loc *time.Location) time.Time {
	fields := strings.Fields(strings.ToLower(text))
	if len(fields) != 3 {
		return time.Time{}
	}
	d, errDay := strconv.Atoi(fields[0])
	month, ok := months[strings.TrimSuffix(fields[1], ".")]
	year, errYear := strconv.Atoi(fields[2])
	if errDay != nil || !ok || errYear != nil {
		return time.Time{}
	}
	return time.Date(year, month, d, 0, 0, 0, 0, loc)
}
