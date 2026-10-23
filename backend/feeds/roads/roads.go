// Package roads draws bus routes on the OSM extract's roads.
package roads

import (
	"conexiuni-cluj/geo"
	"conexiuni-cluj/models"
	"conexiuni-cluj/planner"
	"conexiuni-cluj/router"
	"fmt"
	"os"
)

// Around reads as far again as half the stops' spread.
func Around(stops []models.Stop) (*router.WalkGraph, *router.Places, error) {
	if len(stops) == 0 {
		return nil, nil, nil
	}
	box := router.BBox{MinLat: stops[0].StopLat, MinLon: stops[0].StopLon, MaxLat: stops[0].StopLat, MaxLon: stops[0].StopLon}
	for _, s := range stops {
		box.MinLat, box.MaxLat = min(box.MinLat, s.StopLat), max(box.MaxLat, s.StopLat)
		box.MinLon, box.MaxLon = min(box.MinLon, s.StopLon), max(box.MaxLon, s.StopLon)
	}
	box = box.Grow(geo.Haversine(box.MinLat, box.MinLon, box.MaxLat, box.MaxLon) / 2)
	f, err := os.Open(planner.OSMPath())
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	g, places, err := router.LoadRoads(f, box)
	if err != nil {
		return nil, nil, fmt.Errorf("roads: %w", err)
	}
	return g, places, nil
}

// Path is straight without roads.
func Path(g *router.WalkGraph, a, b router.Point) []router.Point {
	if g == nil {
		return []router.Point{a, b}
	}
	_, points := g.Path(g.Snap(a), g.Snap(b))
	return points
}

func Way(g *router.WalkGraph, p router.Point) (router.Point, router.Point, bool) {
	if g == nil {
		return p, p, false
	}
	s := g.Snap(p)
	if s.Seg < 0 {
		return p, p, false
	}
	a, b := g.Ends(s)
	return a, b, a != b
}

func Through(points []router.Point, route func(a, b router.Point) []router.Point) []router.Point {
	var out []router.Point
	for i := 1; i < len(points); i++ {
		leg := route(points[i-1], points[i])
		if len(out) > 0 && len(leg) > 0 {
			leg = leg[1:]
		}
		out = append(out, leg...)
	}
	return out
}
