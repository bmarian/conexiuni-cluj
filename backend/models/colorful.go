package models

import (
	"fmt"
	"hash/fnv"
	"math"
	"strings"
)

// ColorfulRouteColor is a line's own colour, from its short name, for the clients' colourful mode.
func ColorfulRouteColor(shortName string) string {
	key := strings.TrimSpace(strings.ToUpper(shortName))
	if key == "" {
		return FallbackRouteColor
	}
	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(key))
	hash := hasher.Sum32()
	return hslToHex(float64(hash%360), float64(66+(hash>>9)%16)/100, float64(44+(hash>>17)%12)/100)
}

func hslToHex(h, s, l float64) string {
	q := l + s - l*s
	if l < 0.5 {
		q = l * (1 + s)
	}
	p := 2*l - q
	channel := func(t float64) int {
		if t < 0 {
			t++
		}
		if t > 1 {
			t--
		}
		v := p
		switch {
		case t < 1.0/6:
			v = p + (q-p)*6*t
		case t < 1.0/2:
			v = q
		case t < 2.0/3:
			v = p + (q-p)*(2.0/3-t)*6
		}
		return int(math.Round(v * 255))
	}
	hue := h / 360
	return fmt.Sprintf("#%02X%02X%02X", channel(hue+1.0/3), channel(hue), channel(hue-1.0/3))
}
