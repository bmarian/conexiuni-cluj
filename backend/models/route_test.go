package models

import "testing"

func TestFeedRouteColor(t *testing.T) {
	cases := map[string]string{
		"#f3513c": "#F3513C",
		"FF0000":  "#FF0000",
		"#000000": "#000000",
		"#000":    FallbackRouteColor,
		"":        FallbackRouteColor,
	}
	for raw, want := range cases {
		if got := FeedRouteColor(raw); got != want {
			t.Errorf("FeedRouteColor(%q) = %q, want %q", raw, got, want)
		}
	}
}
