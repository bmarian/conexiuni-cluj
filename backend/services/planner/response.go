package planner

import "conexiuni-cluj/models"

// BuildResponse converts a list of internal candidates into the JSON payload
// returned by /api/plan_routes. It collects every stop and shape referenced so
// the client can render without follow-up requests.
func (g *Graph) BuildResponse(candidates []candidate) PlanResponse {
	resp := PlanResponse{
		Plans:  make([]PlannedRouteResp, 0, len(candidates)),
		Stops:  make(map[int]models.Stop),
		Shapes: make(map[int]models.ShapeInfo),
	}

	usedTrips := make(map[string]struct{})
	for _, c := range candidates {
		for _, l := range c.legs {
			usedTrips[l.tripID] = struct{}{}
		}
	}

	addStop := func(id int) {
		if _, ok := resp.Stops[id]; ok {
			return
		}
		if s, ok := g.stops[id]; ok {
			resp.Stops[id] = s
		}
	}
	addRoute := func(routeID int) {
		if _, ok := resp.Shapes[routeID]; ok {
			return
		}
		si, ok := g.shapeInfos[routeID]
		if !ok {
			return
		}
		// Strip stop_times for trips not referenced in this response — the
		// full set can be 150KB+ per route and the client only filters by trip_id.
		filtered := make([]models.StopTime, 0, len(si.StopTimes))
		for _, st := range si.StopTimes {
			if _, used := usedTrips[st.TripID]; used {
				filtered = append(filtered, st)
			}
		}
		siCopy := si
		siCopy.StopTimes = filtered
		resp.Shapes[routeID] = siCopy
	}

	for _, c := range candidates {
		legs := make([]PlannedLegResp, 0, len(c.legs))
		for _, l := range c.legs {
			shape := g.shapes[l.shapeKey]
			intermediates := make([]int, 0)
			if shape != nil {
				for i := l.startIdx + 1; i < l.destIdx && i < len(shape.Stops); i++ {
					intermediates = append(intermediates, shape.Stops[i].StopID)
					addStop(shape.Stops[i].StopID)
				}
			}
			legs = append(legs, PlannedLegResp{
				RouteID:             l.shapeKey.RouteID,
				TripID:              l.tripID,
				StartStopID:         l.startStopID,
				DestStopID:          l.destStopID,
				RideSeconds:         l.rideSec,
				IntermediateStopIDs: intermediates,
			})
			addStop(l.startStopID)
			addStop(l.destStopID)
			addRoute(l.shapeKey.RouteID)
		}
		resp.Plans = append(resp.Plans, PlannedRouteResp{
			Legs:               legs,
			IsDirect:           len(c.legs) == 1,
			WalkStartMeters:    c.walkStart,
			WalkEndMeters:      c.walkEnd,
			WalkTransferMeters: c.walkTransfers,
			TransitDurationSec: c.transitSec,
			TotalDistance:      c.totalDistance,
		})
	}
	return resp
}
