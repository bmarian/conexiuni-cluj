package sibiu

import (
	"bytes"
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/feeds/news"
	"net/url"
	"time"

	"golang.org/x/net/html"
)

// tursib.ro/news, where each post's date comes just before its title.
func parseNews(page []byte, base *url.URL, loc *time.Location) ([]feeds.NewsItem, error) {
	doc, err := html.Parse(bytes.NewReader(page))
	if err != nil {
		return nil, err
	}
	var items []feeds.NewsItem
	var day time.Time
	for _, n := range news.FindAll(doc, func(n *html.Node) bool { return news.WithTag("time")(n) || news.WithClass("entry-title")(n) }) {
		if n.Data == "time" {
			day, _ = time.ParseInLocation("2006-01-02", news.Attr(n, "datetime"), loc)
			continue
		}
		link := news.First(n, news.WithTag("a"))
		if it, ok := news.Item(news.Text(link), news.Attr(link, "href"), day, base); ok {
			items = append(items, it)
		}
		day = time.Time{}
	}
	return items, nil
}
