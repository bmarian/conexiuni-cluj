package planner

import (
	"conexiuni-cluj/models"
	"math"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

type Graph struct {
	stops      map[int]models.Stop
	stopList   []models.Stop
	shapes     map[ShapeKey]*ShapePath
	stopShapes map[int][]stopShapeRef
	walkPairs  map[int][]walkNeighbor
	shapeInfos map[int]models.ShapeInfo
	routes     map[int]models.Route
}

var (
	active atomic.Pointer[Graph]
	build  sync.Mutex
)

func Active() *Graph { return active.Load() }

func SetActive(g *Graph) { active.Store(g) }

func IsReady() bool { return active.Load() != nil }

// Build constructs a transit graph from cached data. Safe to call repeatedly;
// callers should serialize via the returned mutex if they want to dedupe rebuilds.
func Build(
	stops []models.Stop,
	routes []models.Route,
	stopTimesByRoute map[string][]models.StopTime,
	timetablesByRoute map[string]models.Timetable,
) *Graph {
	build.Lock()
	defer build.Unlock()

	g := &Graph{
		stops:      make(map[int]models.Stop, len(stops)),
		stopList:   make([]models.Stop, 0, len(stops)),
		shapes:     make(map[ShapeKey]*ShapePath),
		stopShapes: make(map[int][]stopShapeRef),
		shapeInfos: make(map[int]models.ShapeInfo),
		routes:     make(map[int]models.Route, len(routes)),
	}
	for _, s := range stops {
		g.stops[s.StopID] = s
		g.stopList = append(g.stopList, s)
	}
	routeByShortName := make(map[string]models.Route, len(routes))
	for _, r := range routes {
		g.routes[r.RouteID] = r
		routeByShortName[r.RouteShortName] = r
	}

	for shortName, sts := range stopTimesByRoute {
		route, ok := routeByShortName[shortName]
		if !ok {
			continue
		}
		buildShapesForRoute(g, route, sts, timetablesByRoute[shortName])
	}

	for key, path := range g.shapes {
		for i, ss := range path.Stops {
			g.stopShapes[ss.StopID] = append(g.stopShapes[ss.StopID], stopShapeRef{
				Key:     key,
				StopIdx: i,
				Offset:  ss.OffsetArrivalTime,
			})
		}
	}

	g.walkPairs = buildWalkPairs(g.stopList)
	return g
}

func buildShapesForRoute(g *Graph, route models.Route, sts []models.StopTime, tt models.Timetable) {
	// Drop "ghost" routes that have no scheduled departures on any day. These
	// show up in the GTFS feed (e.g. discontinued or planned-but-not-running
	// routes like TE8) but would never produce a real arrival on the client.
	if len(tt.Weekdays.Entries) == 0 && len(tt.Saturday.Entries) == 0 && len(tt.Sunday.Entries) == 0 {
		return
	}
	// Group stop_times by trip_id; pick direction by trip_id suffix.
	type tripGroup struct {
		direction models.DirectionType
		stops     []models.StopTime
	}
	trips := make(map[string]*tripGroup)
	for _, st := range sts {
		dir, ok := tripDirection(st.TripID)
		if !ok {
			continue
		}
		grp, exists := trips[st.TripID]
		if !exists {
			grp = &tripGroup{direction: dir}
			trips[st.TripID] = grp
		}
		grp.stops = append(grp.stops, st)
	}

	// For each direction, pick a representative trip with the most stops.
	bestPerDir := make(map[models.DirectionType]string)
	bestLen := make(map[models.DirectionType]int)
	allTripsPerDir := make(map[models.DirectionType][]string)
	for tripID, grp := range trips {
		if len(grp.stops) == 0 {
			continue
		}
		allTripsPerDir[grp.direction] = append(allTripsPerDir[grp.direction], tripID)
		if len(grp.stops) > bestLen[grp.direction] {
			bestLen[grp.direction] = len(grp.stops)
			bestPerDir[grp.direction] = tripID
		}
	}

	for dir, tripID := range bestPerDir {
		grp := trips[tripID]
		sort.Slice(grp.stops, func(i, j int) bool {
			return grp.stops[i].StopSequence < grp.stops[j].StopSequence
		})
		key := ShapeKey{RouteID: route.RouteID, Direction: dir}
		path := &ShapePath{
			Key:            key,
			RouteShortName: route.RouteShortName,
			Stops:          make([]ShapeStop, 0, len(grp.stops)),
		}
		for _, st := range grp.stops {
			path.Stops = append(path.Stops, ShapeStop{
				StopID:            st.StopID,
				StopSequence:      st.StopSequence,
				OffsetArrivalTime: st.OffsetArrivalTime,
			})
		}
		path.TripIDs = append(path.TripIDs, allTripsPerDir[dir]...)
		sort.Strings(path.TripIDs)
		g.shapes[key] = path
	}

	if _, alreadyStored := g.shapeInfos[route.RouteID]; !alreadyStored {
		g.shapeInfos[route.RouteID] = models.ShapeInfo{
			RouteShortName: route.RouteShortName,
			RouteLongName:  route.RouteLongName,
			RouteId:        route.RouteID,
			RouteType:      route.RouteType,
			RouteColor:     route.RouteColor,
			StopTimes:      sts,
			Timetable:      tt,
		}
	}
}

func buildWalkPairs(stopList []models.Stop) map[int][]walkNeighbor {
	pairs := make(map[int][]walkNeighbor, len(stopList))
	r2 := transferWalkRadius * transferWalkRadius
	// Conservative bbox prefilter to keep this O(n^2) palatable.
	for i := range stopList {
		a := stopList[i]
		for j := i + 1; j < len(stopList); j++ {
			b := stopList[j]
			// Quick rejection by latitude diff (1 deg lat ≈ 111km)
			if math.Abs(a.StopLat-b.StopLat)*111000 > transferWalkRadius {
				continue
			}
			d := haversineMeters(a.StopLat, a.StopLon, b.StopLat, b.StopLon)
			if d*d > r2 {
				continue
			}
			pairs[a.StopID] = append(pairs[a.StopID], walkNeighbor{StopID: b.StopID, Distance: d})
			pairs[b.StopID] = append(pairs[b.StopID], walkNeighbor{StopID: a.StopID, Distance: d})
		}
	}
	for k := range pairs {
		sort.Slice(pairs[k], func(i, j int) bool { return pairs[k][i].Distance < pairs[k][j].Distance })
	}
	return pairs
}

func tripDirection(tripID string) (models.DirectionType, bool) {
	switch {
	case strings.HasSuffix(tripID, "_0"):
		return models.Outbound, true
	case strings.HasSuffix(tripID, "_1"):
		return models.Inbound, true
	}
	return 0, false
}

func haversineMeters(lat1, lon1, lat2, lon2 float64) float64 {
	dLat := (lat2 - lat1) * math.Pi / 180
	dLon := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthRadiusMeters * c
}

func (g *Graph) StopByID(id int) (models.Stop, bool) {
	s, ok := g.stops[id]
	return s, ok
}

// ShapeInfoForRoute returns the cached ShapeInfo (with timetable + stop_times)
// for the given route_id. The same ShapeInfo is shared between directions —
// downstream filters by trip_id.
func (g *Graph) ShapeInfoForRoute(routeID int) (models.ShapeInfo, bool) {
	si, ok := g.shapeInfos[routeID]
	return si, ok
}
