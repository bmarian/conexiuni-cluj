package models

import (
	"fmt"
	"hash/fnv"
	"math"
	"strings"
)

// ColorfulRouteColor is a line's own colour from its short name alone.
func ColorfulRouteColor(shortName string) string {
	h, s, l, ok := ColorfulHSL(shortName)
	if !ok {
		return FallbackRouteColor
	}
	return HSLHex(h, s, l)
}

// ColorfulHSL is ColorfulRouteColor as hue in degrees, saturation and lightness.
func ColorfulHSL(shortName string) (h, s, l float64, ok bool) {
	key := strings.TrimSpace(strings.ToUpper(shortName))
	if key == "" {
		return 0, 0, 0, false
	}
	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(key))
	hash := hasher.Sum32()
	return float64(hash % 360), float64(66+(hash>>9)%16) / 100, float64(lightnessFrom+(hash>>17)%lightnessSteps) / 100, true
}

const lightnessFrom, lightnessSteps = 44, 12

// ColorfulLightness is the lightness range ColorfulHSL picks from.
func ColorfulLightness() (lo, hi float64) {
	return lightnessFrom / 100.0, (lightnessFrom + lightnessSteps - 1) / 100.0
}

func HSLHex(h, s, l float64) string {
	r, g, b := HSLRGB(h, s, l)
	channel := func(v float64) int { return int(math.Round(v * 255)) }
	return fmt.Sprintf("#%02X%02X%02X", channel(r), channel(g), channel(b))
}

// HSLRGB is the colour's sRGB channels, from 0 to 1.
func HSLRGB(h, s, l float64) (r, g, b float64) {
	q := l + s - l*s
	if l < 0.5 {
		q = l * (1 + s)
	}
	p := 2*l - q
	channel := func(t float64) float64 {
		if t < 0 {
			t++
		}
		if t > 1 {
			t--
		}
		switch {
		case t < 1.0/6:
			return p + (q-p)*6*t
		case t < 1.0/2:
			return q
		case t < 2.0/3:
			return p + (q-p)*(2.0/3-t)*6
		}
		return p
	}
	hue := h / 360
	return channel(hue + 1.0/3), channel(hue), channel(hue - 1.0/3)
}
