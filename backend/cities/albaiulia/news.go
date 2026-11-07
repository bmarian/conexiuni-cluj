package albaiulia

import (
	"bytes"
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/feeds/news"
	"net/url"
	"time"

	"golang.org/x/net/html"
)

// stpalba.ro/presa.php, each post a link holding its date and title.
func parseNews(page []byte, base *url.URL, loc *time.Location) ([]feeds.NewsItem, error) {
	doc, err := html.Parse(bytes.NewReader(page))
	if err != nil {
		return nil, err
	}
	var items []feeds.NewsItem
	for _, n := range news.FindAll(doc, news.WithClass("stire")) {
		day, _ := time.ParseInLocation("02-01-2006", news.Text(news.First(n, news.WithClass("stire_data"))), loc)
		if it, ok := news.Item(news.Text(news.First(n, news.WithClass("stire_titlu"))), news.Attr(news.First(n, news.WithTag("a")), "href"), day, base); ok {
			items = append(items, it)
		}
	}
	return items, nil
}
