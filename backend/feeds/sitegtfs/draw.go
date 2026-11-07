package sitegtfs

import (
	"conexiuni-cluj/feeds/roads"
	"conexiuni-cluj/models"
	"conexiuni-cluj/router"
	"log"
)

// Place gives a stop one route's page leaves unplaced the place another page gives the same stop id, and says which stay unplaced.
func Place(routes []Route) {
	at := map[string]router.Point{}
	for _, r := range routes {
		for _, s := range r.Stops {
			if s.Placed {
				at[s.ID] = s.At
			}
		}
	}
	for _, r := range routes {
		for i, s := range r.Stops {
			if pt, ok := at[s.ID]; ok && !s.Placed {
				r.Stops[i].At, r.Stops[i].Placed = pt, true
			} else if !ok {
				log.Printf("sitegtfs: %s: no page places %s, so the route skips it", r.ID, s.Name)
			}
		}
	}
}

// Draw lays each route on the roads from stop to stop; straight legs without the OSM extract.
func Draw(routes []Route) map[string][]router.Point {
	var stops []models.Stop
	for _, r := range routes {
		for _, s := range r.Stops {
			if s.Placed {
				stops = append(stops, models.Stop{StopLat: s.At.Lat, StopLon: s.At.Lon})
			}
		}
	}
	g, _, err := roads.Around(stops)
	if err != nil {
		log.Printf("sitegtfs: %v; the lines get straight legs", err)
	}
	out := make(map[string][]router.Point, len(routes))
	for _, r := range routes {
		var points []router.Point
		for _, s := range r.Stops {
			if s.Placed {
				points = append(points, s.At)
			}
		}
		out[r.ID] = roads.Through(points, func(a, b router.Point) []router.Point { return roads.Path(g, a, b) })
	}
	return out
}
