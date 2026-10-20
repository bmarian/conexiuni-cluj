// Package newstest checks a news parser against a saved page.
package newstest

import (
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/feeds/news"
	"net/url"
	"testing"
	"time"
)

var Bucharest, _ = time.LoadLocation("Europe/Bucharest")

func Parse(t *testing.T, parser news.Parser, page, base string) []feeds.NewsItem {
	t.Helper()
	u, _ := url.Parse(base)
	items, err := parser([]byte(page), u, Bucharest)
	if err != nil {
		t.Fatal(err)
	}
	return items
}

func Expect(t *testing.T, got []feeds.NewsItem, want ...feeds.NewsItem) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d items %+v, want %d", len(got), got, len(want))
	}
	for i := range want {
		if got[i].Title != want[i].Title || got[i].URL != want[i].URL || !got[i].At.Equal(want[i].At) {
			t.Errorf("item %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func Day(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, Bucharest) }
