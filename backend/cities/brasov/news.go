package brasov

import (
	"bytes"
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/feeds/news"
	"net/url"
	"time"

	"golang.org/x/net/html"
)

// ratbv.ro/noutati, each post a box with a linked title and its date.
func parseNews(page []byte, base *url.URL, loc *time.Location) ([]feeds.NewsItem, error) {
	doc, err := html.Parse(bytes.NewReader(page))
	if err != nil {
		return nil, err
	}
	var items []feeds.NewsItem
	for _, n := range news.FindAll(doc, news.WithClass("stire-interior")) {
		link := news.First(n, news.WithTag("a"))
		day, _ := time.ParseInLocation("02.01.2006", news.Text(news.First(n, news.WithClass("data"))), loc)
		if it, ok := news.Item(news.Text(link), news.Attr(link, "href"), day, base); ok {
			items = append(items, it)
		}
	}
	return items, nil
}
