package news

import (
	"bytes"
	"conexiuni-cluj/sources"
	"net/url"
	"slices"
	"strings"
	"time"

	"golang.org/x/net/html"
)

func SCTP(page []byte, base *url.URL, loc *time.Location) ([]sources.NewsItem, error) {
	doc, err := html.Parse(bytes.NewReader(page))
	if err != nil {
		return nil, err
	}
	var items []sources.NewsItem
	for _, n := range findAll(doc, withClass("stire-ratp")) {
		day, _ := time.ParseInLocation("02.01.2006", text(first(n, withClass("data"))), loc)
		if it, ok := item(text(first(n, withTag("h3"))), attr(first(n, withClass("readmore")), "href"), day, base); ok {
			items = append(items, it)
		}
	}
	return items, nil
}

// CTBus reads ctbus.ro/comunicate, where each post's anchor starts with its date.
func CTBus(page []byte, base *url.URL, loc *time.Location) ([]sources.NewsItem, error) {
	doc, err := html.Parse(bytes.NewReader(page))
	if err != nil {
		return nil, err
	}
	var items []sources.NewsItem
	for _, n := range findAll(doc, withClass("recente-stirea")) {
		// Job ads.
		if first(n, withClass("fa-user-plus")) != nil {
			continue
		}
		link := first(first(n, withClass("recente-titlu")), withTag("a"))
		href := attr(link, "href")
		date, _, _ := strings.Cut(strings.TrimPrefix(href, "#"), "-")
		day, _ := time.ParseInLocation("20060102", date, loc)
		if it, ok := item(text(link), href, day, base); ok {
			items = append(items, it)
		}
	}
	return items, nil
}

// Tursib reads tursib.ro/news, where each post's date comes just before its title.
func Tursib(page []byte, base *url.URL, loc *time.Location) ([]sources.NewsItem, error) {
	doc, err := html.Parse(bytes.NewReader(page))
	if err != nil {
		return nil, err
	}
	var items []sources.NewsItem
	var day time.Time
	for _, n := range findAll(doc, func(n *html.Node) bool { return withTag("time")(n) || withClass("entry-title")(n) }) {
		if n.Data == "time" {
			day, _ = time.ParseInLocation("2006-01-02", attr(n, "datetime"), loc)
			continue
		}
		link := first(n, withTag("a"))
		if it, ok := item(text(link), attr(link, "href"), day, base); ok {
			items = append(items, it)
		}
		day = time.Time{}
	}
	return items, nil
}

func withTag(tag string) func(*html.Node) bool {
	return func(n *html.Node) bool { return n.Type == html.ElementNode && n.Data == tag }
}

func withClass(class string) func(*html.Node) bool {
	return func(n *html.Node) bool {
		return n.Type == html.ElementNode && slices.Contains(strings.Fields(attr(n, "class")), class)
	}
}

// In document order; a match's own descendants are not searched.
func findAll(n *html.Node, match func(*html.Node) bool) []*html.Node {
	var out []*html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if match(c) {
				out = append(out, c)
				continue
			}
			walk(c)
		}
	}
	if n != nil {
		walk(n)
	}
	return out
}

func first(n *html.Node, match func(*html.Node) bool) *html.Node {
	if n == nil {
		return nil
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if match(c) {
			return c
		}
		if found := first(c, match); found != nil {
			return found
		}
	}
	return nil
}

func attr(n *html.Node, key string) string {
	if n == nil {
		return ""
	}
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func text(n *html.Node) string {
	var sb strings.Builder
	var collect func(*html.Node)
	collect = func(n *html.Node) {
		if n.Type == html.TextNode {
			sb.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			collect(c)
		}
	}
	if n != nil {
		collect(n)
	}
	return strings.TrimSpace(sb.String())
}
