package slobozia

import (
	"bytes"
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/feeds/news"
	"conexiuni-cluj/feeds/sitegtfs"
	"conexiuni-cluj/router"
	"fmt"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
)

// site reads sptl.ro's bundle again only when the home page names another; its name changes with its contents.
type site struct {
	home   string
	school feeds.School

	mu     sync.Mutex
	bundle string
	lines  []sitegtfs.Line
	shapes map[string][]router.Point
}

func (s *site) zip() ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	page, err := feeds.DownloadInTurn(s.home)
	if err != nil {
		return nil, fmt.Errorf("sptl.ro: %w", err)
	}
	bundle, err := bundleURL(page, s.home)
	if err != nil {
		return nil, err
	}
	if bundle != s.bundle {
		data, err := feeds.Download(bundle)
		if err != nil {
			return nil, fmt.Errorf("sptl.ro: %w", err)
		}
		stops, routes, err := parseBundle(data)
		if err != nil {
			return nil, fmt.Errorf("sptl.ro: %s: %w", bundle, err)
		}
		s.lines = lines(stops, routes)
		var ways []sitegtfs.Route
		for _, l := range s.lines {
			for _, w := range l.Ways {
				if w != nil {
					ways = append(ways, *w)
				}
			}
		}
		sitegtfs.Place(ways)
		s.shapes, s.bundle = sitegtfs.Draw(ways), bundle
	}
	return sitegtfs.Zip(s.lines, s.shapes, s.school, time.Now())
}

// The script holding the network, among those the page preloads.
func bundleURL(page []byte, home string) (string, error) {
	doc, err := html.Parse(bytes.NewReader(page))
	if err != nil {
		return "", fmt.Errorf("sptl.ro: %w", err)
	}
	base, err := url.Parse(home)
	if err != nil {
		return "", err
	}
	for _, link := range news.FindAll(doc, news.WithTag("link")) {
		href, err := base.Parse(news.Attr(link, "href"))
		if err == nil && strings.HasPrefix(path.Base(href.Path), "busData-") {
			return href.String(), nil
		}
	}
	return "", fmt.Errorf("sptl.ro: no network bundle on %s", home)
}
