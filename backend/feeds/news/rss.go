package news

import (
	"bytes"
	"conexiuni-cluj/feeds"
	"encoding/xml"
	"net/url"
	"strings"
	"time"
)

var pubDateLayouts = []string{"Mon, 2 Jan 2006 15:04:05 -0700", "Mon, 2 Jan 2006 15:04:05 MST"}

func RSS(page []byte, base *url.URL, _ *time.Location) ([]feeds.NewsItem, error) {
	var feed struct {
		Items []struct {
			Title   string `xml:"title"`
			Link    string `xml:"link"`
			PubDate string `xml:"pubDate"`
		} `xml:"channel>item"`
	}
	d := xml.NewDecoder(bytes.NewReader(page))
	d.Strict = false
	d.Entity = xml.HTMLEntity
	if err := d.Decode(&feed); err != nil {
		return nil, err
	}
	var items []feeds.NewsItem
	for _, raw := range feed.Items {
		var at time.Time
		for _, layout := range pubDateLayouts {
			if t, err := time.Parse(layout, strings.TrimSpace(raw.PubDate)); err == nil {
				at = t
				break
			}
		}
		if it, ok := Item(raw.Title, raw.Link, at, base); ok {
			items = append(items, it)
		}
	}
	return items, nil
}
