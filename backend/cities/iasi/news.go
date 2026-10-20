package iasi

import (
	"bytes"
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/feeds/news"
	"net/url"
	"regexp"
	"time"

	"golang.org/x/net/html"
)

// sctpiasi.ro marks job ads only in the title.
var sctpJobAd = regexp.MustCompile(`(?i)\bpost\w*\s+(vacant|liber)|\bangajare\b|ocup[aă]rii\s+post`)

func parseNews(page []byte, base *url.URL, loc *time.Location) ([]feeds.NewsItem, error) {
	doc, err := html.Parse(bytes.NewReader(page))
	if err != nil {
		return nil, err
	}
	var items []feeds.NewsItem
	for _, n := range news.FindAll(doc, news.WithClass("stire-ratp")) {
		title := news.Text(news.First(n, news.WithTag("h3")))
		if sctpJobAd.MatchString(title) {
			continue
		}
		day, _ := time.ParseInLocation("02.01.2006", news.Text(news.First(n, news.WithClass("data"))), loc)
		if it, ok := news.Item(title, news.Attr(news.First(n, news.WithClass("readmore")), "href"), day, base); ok {
			items = append(items, it)
		}
	}
	return items, nil
}
