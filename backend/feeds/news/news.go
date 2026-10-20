// Package news reads operators' news pages.
package news

import (
	"conexiuni-cluj/feeds"
	"fmt"
	"html"
	"net/url"
	"strings"
	"time"
	"unicode"
)

type Parser func(page []byte, base *url.URL, loc *time.Location) ([]feeds.NewsItem, error)

type Source struct {
	url     string
	parse   Parser
	cadence time.Duration
}

func New(pageURL string, parse Parser, cadence time.Duration) *Source {
	return &Source{url: pageURL, parse: parse, cadence: cadence}
}

func (s *Source) Cadence() time.Duration { return s.cadence }

func (s *Source) News(loc *time.Location) ([]feeds.NewsItem, error) {
	page, err := feeds.Download(s.url)
	if err != nil {
		return nil, err
	}
	base, err := url.Parse(s.url)
	if err != nil {
		return nil, err
	}
	items, err := s.parse(page, base, loc)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", s.url, err)
	}
	// A page that changed its layout reads as empty; keep the last news instead.
	if len(items) == 0 {
		return nil, fmt.Errorf("%s: no news found", s.url)
	}
	return items, nil
}

func resolve(base *url.URL, href string) string {
	ref, err := url.Parse(strings.TrimSpace(href))
	if err != nil || href == "" {
		return ""
	}
	u := base.ResolveReference(ref)
	if query := u.Query(); len(query) > 0 {
		for key := range query {
			if strings.HasPrefix(key, "utm_") {
				query.Del(key)
			}
		}
		u.RawQuery = query.Encode()
	}
	return u.String()
}

func cleanTitle(s string) string {
	s = strings.Join(strings.Fields(html.UnescapeString(s)), " ")
	return strings.TrimLeftFunc(s, unicode.IsMark)
}

// Item is a page's post, or false when its title, link or date is missing.
func Item(title, href string, at time.Time, base *url.URL) (feeds.NewsItem, bool) {
	it := feeds.NewsItem{Title: cleanTitle(title), URL: resolve(base, href), At: at}
	return it, it.Title != "" && it.URL != "" && !at.IsZero()
}
