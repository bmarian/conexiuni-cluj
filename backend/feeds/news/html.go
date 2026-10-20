package news

import (
	"slices"
	"strings"

	"golang.org/x/net/html"
)

// Helpers for the operators whose news is a web page rather than a feed.

func WithTag(tag string) func(*html.Node) bool {
	return func(n *html.Node) bool { return n.Type == html.ElementNode && n.Data == tag }
}

func WithClass(class string) func(*html.Node) bool {
	return func(n *html.Node) bool {
		return n.Type == html.ElementNode && slices.Contains(strings.Fields(Attr(n, "class")), class)
	}
}

// In document order; a match's own descendants are not searched.
func FindAll(n *html.Node, match func(*html.Node) bool) []*html.Node {
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

func First(n *html.Node, match func(*html.Node) bool) *html.Node {
	if n == nil {
		return nil
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if match(c) {
			return c
		}
		if found := First(c, match); found != nil {
			return found
		}
	}
	return nil
}

func Attr(n *html.Node, key string) string {
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

func Text(n *html.Node) string {
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
