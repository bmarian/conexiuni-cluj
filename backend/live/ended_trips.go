package live

import (
	"conexiuni-cluj/geo"
	"conexiuni-cluj/models"
	"sync"
	"time"
)

type endedTrip struct {
	from, to     string
	at           string
	feedCaughtUp bool
}

type endedTripsState struct {
	sync.Mutex
	byVehicle map[int]endedTrip
}

// The feed keeps some buses on a finished trip through the stand and partway back (57 at Disp. Alverna).
func (eng *Engine) followEndedTrips(vehicles []models.Vehicle) {
	eng.endedTrips.Lock()
	defer eng.endedTrips.Unlock()
	seen := make(map[int]bool, len(vehicles))
	for i := range vehicles {
		v := &vehicles[i]
		seen[v.ID] = true
		e, ok := eng.endedTrips.byVehicle[v.ID]
		if ok && (v.TripID == e.from && e.feedCaughtUp || v.TripID != e.from && v.TripID != e.to) {
			delete(eng.endedTrips.byVehicle, v.ID)
			ok = false
		}
		if !ok {
			next, reached := eng.reachedTripEnd(*v)
			if !reached {
				continue
			}
			e = endedTrip{from: v.TripID, to: next, at: v.Timestamp}
		}
		if v.TripID == e.to {
			e.feedCaughtUp = true
			if parseVehicleTime(v.TripChangedAt, time.Time{}) > parseVehicleTime(e.at, time.Time{}) {
				v.PrevTripID, v.TripChangedAt = e.from, e.at
			}
		} else {
			v.TripID, v.PrevTripID, v.TripChangedAt = e.to, e.from, e.at
		}
		eng.endedTrips.byVehicle[v.ID] = e
	}
	for id := range eng.endedTrips.byVehicle {
		if !seen[id] {
			delete(eng.endedTrips.byVehicle, id)
		}
	}
}

func (eng *Engine) reachedTripEnd(v models.Vehicle) (string, bool) {
	if v.TripID == "" || v.TripID == "-1" {
		return "", false
	}
	from, ok := eng.getTripSegmentTracker(v.TripID)
	if !ok || len(from.Stops) < 2 || len(from.Shape) < 2 {
		return "", false
	}
	last := from.Stops[len(from.Stops)-1]
	if geo.Haversine(v.Latitude, v.Longitude, last.Lat, last.Lon) > segmentStopProximityMeters ||
		geo.ClosestShapeIndex(v.Latitude, v.Longitude, from.Shape) < from.Stops[len(from.Stops)-2].ShapeIdx {
		return "", false
	}
	nextID := otherDirectionTripID(v.TripID)
	next, ok := eng.getTripSegmentTracker(nextID)
	if !ok || next.RouteID != from.RouteID || len(next.Stops) < 2 {
		return "", false
	}
	first := next.Stops[0]
	if geo.Haversine(last.Lat, last.Lon, first.Lat, first.Lon) > throughTerminusMeters {
		return "", false
	}
	return nextID, true
}
