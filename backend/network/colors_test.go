package network

import (
	"conexiuni-cluj/models"
	"fmt"
	"testing"
)

func labOfHex(hex string) lab {
	var r, g, b int
	_, _ = fmt.Sscanf(hex, "#%02X%02X%02X", &r, &g, &b)
	return oklab(float64(r)/255, float64(g)/255, float64(b)/255)
}

func TestLineColorsSetLinesAtOneStationApart(t *testing.T) {
	// 47 and 6 hash to nearly the same cyan; 99 shares no station.
	stations := map[string]map[int]bool{"47": {1: true, 2: true}, "6": {1: true}, "8": {2: true}, "99": {3: true}}
	colors := lineColors(stations)
	if got, want := colors["99"], models.ColorfulRouteColor("99"); got != want {
		t.Errorf("a line alone keeps its own colour: got %s, want %s", got, want)
	}
	before := labOfHex(models.ColorfulRouteColor("47")).dist(labOfHex(models.ColorfulRouteColor("6")))
	after := labOfHex(colors["47"]).dist(labOfHex(colors["6"]))
	if after <= before*2 {
		t.Errorf("47 and 6 share a station but stay close: %.3f, was %.3f", after, before)
	}
	again := lineColors(stations)
	for name, color := range colors {
		if again[name] != color {
			t.Errorf("%s: %s, then %s", name, color, again[name])
		}
	}
}
