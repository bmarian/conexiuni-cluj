package network

import (
	"conexiuni-cluj/models"
	"fmt"
	"testing"
)

func TestLineColorsSetLinesThatMeetApart(t *testing.T) {
	// Eight lines at one station, two more meeting them only at a station next door, and one alone.
	stations := map[string]map[int]bool{"99": {9: true}, "N1": {2: true}, "N2": {2: true}}
	for i := range 8 {
		stations[fmt.Sprint(i+1)] = map[int]bool{1: true}
	}
	near := map[int][]int{1: {2}, 2: {1}}
	colors := lineColors(stations, near)
	inPalette := map[string]bool{}
	for _, c := range palette() {
		inPalette[c.hex] = true
	}
	for a := range stations {
		if !inPalette[colors[a]] {
			t.Errorf("%s: %s is not from the palette", a, colors[a])
		}
		for b := range stations {
			if a >= b || a == "99" || b == "99" {
				continue
			}
			if d := labOfHex(colors[a]).dist(labOfHex(colors[b])); d < 0.12 {
				t.Errorf("%s %s and %s %s meet but are only %.3f apart", a, colors[a], b, colors[b], d)
			}
		}
	}
	again := lineColors(stations, near)
	for name, color := range colors {
		if again[name] != color {
			t.Errorf("%s: %s, then %s", name, color, again[name])
		}
	}
}

func TestNearStationsUsesTheTypicalGap(t *testing.T) {
	// Stations 1-2 and 3-4 are pairs 30 m apart, 1 km from each other; 5 sits 2 km out.
	const deg = 1.0 / 111_195
	at := map[int]place{1: {46, 23}, 2: {46 + 30*deg, 23}, 3: {46 + 1000*deg, 23}, 4: {46 + 1030*deg, 23}, 5: {46 + 3000*deg, 23}}
	near := nearStations(at)
	if len(near[1]) != 1 || near[1][0] != 2 || len(near[3]) != 1 || near[3][0] != 4 {
		t.Errorf("pairs should be near each other: %v", near)
	}
	if len(near[5]) != 0 {
		t.Errorf("a far station is near nothing: %v", near[5])
	}
}

func TestWhiteReadable(t *testing.T) {
	for hex, want := range map[string]bool{"#1D71B8": true, "#FFE066": false, "#7F7F7F": false} {
		if got := models.WhiteReadable(hex); got != want {
			t.Errorf("%s: %v, want %v", hex, got, want)
		}
	}
}
