package destinations

import (
	"conexiuni-cluj/geo"
	"conexiuni-cluj/router"
	"sort"
)

func IsDistrict(place string) bool {
	return place == "suburb" || place == "neighbourhood" || place == "quarter"
}

// The place labels' rule: the nearest district when one is nearer than any town, else that town.
func (x *Index) addAreas(areas []router.Area) {
	var districts, towns []router.Area
	for _, a := range areas {
		if IsDistrict(a.Place) {
			districts = append(districts, a)
		} else {
			towns = append(towns, a)
		}
	}
	byArea := map[router.Area]*Destination{}
	ids := make([]int, 0, len(x.calls))
	for id := range x.calls {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	for _, id := range ids {
		stop := x.stops[id]
		if stop == nil {
			continue
		}
		d, dm := nearest(districts, stop.StopLat, stop.StopLon)
		t, tm := nearest(towns, stop.StopLat, stop.StopLon)
		area := t
		if d != nil && (t == nil || dm < tm) {
			area = d
		}
		if area == nil {
			continue
		}
		dest := byArea[*area]
		if dest == nil {
			dest = &Destination{Name: area.Name, District: IsDistrict(area.Place), Lat: area.Lat, Lon: area.Lon}
			byArea[*area] = dest
			x.list = append(x.list, dest)
		}
		dest.StopIDs = append(dest.StopIDs, id)
		x.areaOf[id] = dest
	}
	// The area with the most stops keeps a shared slug; the others add their nearest town.
	sort.SliceStable(x.list, func(i, j int) bool { return len(x.list[i].StopIDs) > len(x.list[j].StopIDs) })
	kept := x.list[:0]
	for _, dest := range x.list {
		dest.Slug = router.Slug(dest.Name)
		if x.bySlug[dest.Slug] != nil {
			others := make([]router.Area, 0, len(towns))
			for _, t := range towns {
				if t.Name != dest.Name {
					others = append(others, t)
				}
			}
			if t, _ := nearest(others, dest.Lat, dest.Lon); t != nil {
				dest.Slug += "-" + router.Slug(t.Name)
			}
		}
		if dest.Slug == "" || x.bySlug[dest.Slug] != nil {
			for _, id := range dest.StopIDs {
				delete(x.areaOf, id)
			}
			continue
		}
		x.bySlug[dest.Slug] = dest
		kept = append(kept, dest)
	}
	x.list = kept
	sizes := make(map[*Destination]int, len(x.list))
	for _, dest := range x.list {
		sizes[dest] = len(dest.StopIDs)
	}
	for _, dest := range x.list {
		x.fill(dest, sizes)
	}
	sort.Slice(x.list, func(i, j int) bool { return router.Slug(x.list[i].Name) < router.Slug(x.list[j].Name) })
}

func nearest(among []router.Area, lat, lon float64) (*router.Area, float64) {
	var best *router.Area
	bestM := 0.0
	for i := range among {
		if m := geo.Haversine(lat, lon, among[i].Lat, among[i].Lon); best == nil || m < bestM {
			best, bestM = &among[i], m
		}
	}
	return best, bestM
}

func (x *Index) fill(dest *Destination, sizes map[*Destination]int) {
	lines := map[int]bool{}
	for _, id := range dest.StopIDs {
		for _, p := range x.calls[id] {
			lines[p.route.RouteID] = true
		}
	}
	for id := range lines {
		dest.RouteIDs = append(dest.RouteIDs, id)
	}
	x.sortRoutes(dest.RouteIDs)
	sort.SliceStable(dest.StopIDs, func(i, j int) bool {
		a, b := len(x.calls[dest.StopIDs[i]]), len(x.calls[dest.StopIDs[j]])
		if a != b {
			return a > b
		}
		return x.stops[dest.StopIDs[i]].StopName < x.stops[dest.StopIDs[j]].StopName
	})

	byOrigin := map[*Destination]map[int]Ride{}
	for _, p := range x.patterns {
		// The shortest ride from an area starts at its latest stop before dest.
		last := map[*Destination]int{}
		done := map[*Destination]bool{}
		for j, id := range p.stops {
			if x.areaOf[id] == dest {
				for origin, i := range last {
					if done[origin] {
						continue
					}
					done[origin] = true
					ride, ok := x.ride(p, i, j)
					if !ok {
						continue
					}
					rides := byOrigin[origin]
					if rides == nil {
						rides = map[int]Ride{}
						byOrigin[origin] = rides
					}
					if have, seen := rides[ride.RouteID]; !seen || ride.expected() < have.expected() {
						rides[ride.RouteID] = ride
					}
				}
				continue
			}
			if origin := x.areaOf[id]; origin != nil {
				last[origin] = j
				done[origin] = false
			}
		}
	}
	origins := make([]*Destination, 0, len(byOrigin))
	for origin := range byOrigin {
		origins = append(origins, origin)
	}
	sort.Slice(origins, func(i, j int) bool {
		if sizes[origins[i]] != sizes[origins[j]] {
			return sizes[origins[i]] > sizes[origins[j]]
		}
		return origins[i].Slug < origins[j].Slug
	})
	dest.From = make([]Origin, 0, len(origins))
	for _, origin := range origins {
		o := Origin{Slug: origin.Slug, Name: origin.Name}
		for _, r := range byOrigin[origin] {
			o.Rides = append(o.Rides, r)
		}
		sort.Slice(o.Rides, func(i, j int) bool {
			if a, b := o.Rides[i].expected(), o.Rides[j].expected(); a != b {
				return a < b
			}
			return lineLess(x.routes[o.Rides[i].RouteID].RouteShortName, x.routes[o.Rides[j].RouteID].RouteShortName)
		})
		dest.From = append(dest.From, o)
	}
}
