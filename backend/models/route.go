package models

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math"
	"strings"
)

type RouteType int

const (
	Tram       RouteType = 0
	Subway     RouteType = 1
	Rail       RouteType = 2
	Bus        RouteType = 3
	Ferry      RouteType = 4
	CableTram  RouteType = 5
	AerialLift RouteType = 6
	Funicular  RouteType = 7
	Trolleybus RouteType = 11
	Monorail   RouteType = 12
)

type Route struct {
	RouteID        int       `json:"route_id" db:"route_id"`
	AgencyID       int       `json:"agency_id" db:"agency_id"`
	RouteShortName string    `json:"route_short_name" db:"route_short_name"`
	RouteLongName  string    `json:"route_long_name" db:"route_long_name"`
	RouteType      RouteType `json:"route_type" db:"route_type"`
	RouteDesc      string    `json:"route_desc" db:"route_desc"`
	RouteColor     string    `json:"route_color" db:"route_color"`
}

func (r *Route) UnmarshalJSON(data []byte) error {
	type rawRoute Route
	var raw rawRoute
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	*r = Route(raw)
	r.RouteColor = ResolveRouteDisplayColor(r.RouteShortName)
	return nil
}

// ResolveRouteDisplayColor deterministically maps a route short name to a display color
func ResolveRouteDisplayColor(routeShortName string) string {
	key := strings.TrimSpace(strings.ToUpper(routeShortName))
	if key == "" {
		return "#64748B"
	}

	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(key))
	hash := hasher.Sum32()

	h := int(hash % 360)
	s := int(66 + ((hash >> 9) % 16))  // 66..81
	l := int(44 + ((hash >> 17) % 12)) // 44..55

	return hslToHex(h, s, l)
}

func hslToHex(h, s, l int) string {
	hf := float64(h%360) / 360.0
	sf := float64(clampInt(s, 0, 100)) / 100.0
	lf := float64(clampInt(l, 0, 100)) / 100.0

	var r, g, b float64
	if sf == 0 {
		r, g, b = lf, lf, lf
	} else {
		q := lf * (1 + sf)
		if lf >= 0.5 {
			q = lf + sf - (lf * sf)
		}
		p := 2*lf - q
		r = hueToRGB(p, q, hf+1.0/3.0)
		g = hueToRGB(p, q, hf)
		b = hueToRGB(p, q, hf-1.0/3.0)
	}

	ri := int(math.Round(r * 255))
	gi := int(math.Round(g * 255))
	bi := int(math.Round(b * 255))

	return fmt.Sprintf("#%02X%02X%02X", clampInt(ri, 0, 255), clampInt(gi, 0, 255), clampInt(bi, 0, 255))
}

func hueToRGB(p, q, t float64) float64 {
	for t < 0 {
		t += 1
	}
	for t > 1 {
		t -= 1
	}
	switch {
	case t < 1.0/6.0:
		return p + (q-p)*6*t
	case t < 1.0/2.0:
		return q
	case t < 2.0/3.0:
		return p + (q-p)*(2.0/3.0-t)*6
	default:
		return p
	}
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
