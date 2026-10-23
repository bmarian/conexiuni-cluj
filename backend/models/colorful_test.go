package models

import "testing"

func TestColorfulRouteColorIsReadable(t *testing.T) {
	for _, name := range []string{"25", "35", "24B", "M26N", "8L", "101", " 5n ", "Ș1", "RoHu", "N104", "8", "24", "7"} {
		got := ColorfulRouteColor(name)
		h, s, l, _ := ColorfulHSL(name)
		if c := whiteContrast(h, s, ReadableLightness(h, s, l)); c < minContrast {
			t.Errorf("%q: %s has contrast %.2f with white", name, got, c)
		}
		if want := HSLHex(h, s, ReadableLightness(h, s, l)); got != want {
			t.Errorf("%q: %s, want %s", name, got, want)
		}
	}
	if got := ColorfulRouteColor("25"); got != "#462EE0" {
		t.Errorf("a line already readable keeps its colour: got %s", got)
	}
	if got := ColorfulRouteColor(""); got != FallbackRouteColor {
		t.Errorf("no name: got %s", got)
	}
}
