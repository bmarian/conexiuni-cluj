package constanta

import (
	"bytes"
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/feeds/news"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// ctbus.ro/comunicate, where each post's anchor starts with its date.
func parseNews(page []byte, base *url.URL, loc *time.Location) ([]feeds.NewsItem, error) {
	doc, err := html.Parse(bytes.NewReader(page))
	if err != nil {
		return nil, err
	}
	var items []feeds.NewsItem
	for _, n := range news.FindAll(doc, news.WithClass("recente-stirea")) {
		// Job ads.
		if news.First(n, news.WithClass("fa-user-plus")) != nil {
			continue
		}
		link := news.First(news.First(n, news.WithClass("recente-titlu")), news.WithTag("a"))
		href := news.Attr(link, "href")
		date, _, _ := strings.Cut(strings.TrimPrefix(href, "#"), "-")
		day, _ := time.ParseInLocation("20060102", date, loc)
		if it, ok := news.Item(news.Text(link), href, day, base); ok {
			items = append(items, it)
		}
	}
	return items, nil
}
