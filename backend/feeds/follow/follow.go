// Package follow puts buses that report only their line on one of its patterns, from the way they move along its shapes.
package follow

import (
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/live"
	"conexiuni-cluj/models"
	"conexiuni-cluj/predictions"
	"slices"
	"sync"
	"time"
)

type Line struct {
	RouteID int
	// A longer line number the bus's label may have been cut from; the line it names wins wherever it fits.
	Cut bool
}

type Bus struct {
	// Plate or fleet number, the same at every poll.
	ID       string
	Label    string
	Lat, Lon float64
	// When the feed says it was there; zero for the time of the poll.
	At    time.Time
	Lines []Line
}

type Follower struct {
	ids   feeds.IDMap
	trace feeds.Tracing

	mu   sync.Mutex
	last map[string]reading
}

func New(ids feeds.IDMap, trace feeds.Tracing) *Follower {
	return &Follower{ids: ids, trace: trace, last: map[string]reading{}}
}

// A bus as the feed shows it; its pattern once its movement tells.
type reading struct {
	lat, lon float64
	at       int64
	// Where it last moved from: a bus creeping a few metres a poll has still moved once they add up.
	fromLat, fromLon   float64
	routeID, direction int
	placed             bool
}

// Vehicles are the buses of one poll that a pattern fits; the others wait until they move.
func (f *Follower) Vehicles(buses []Bus, now time.Time) ([]models.Vehicle, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	seen := make(map[string]reading, len(buses))
	var out []models.Vehicle
	for _, b := range buses {
		if len(b.Lines) == 0 {
			continue
		}
		at := now
		if !b.At.IsZero() {
			at = b.At
		}
		prev, hasPrev := f.last[b.ID]
		cur := reading{lat: b.Lat, lon: b.Lon, at: at.Unix(), fromLat: b.Lat, fromLon: b.Lon}
		var moved bool
		cur.routeID, cur.direction, cur.placed, moved = f.follow(b.Lines, prev, hasPrev, cur)
		if hasPrev && !moved {
			cur.fromLat, cur.fromLon = prev.fromLat, prev.fromLon
		}
		seen[b.ID] = cur
		if !cur.placed {
			continue
		}
		id, err := f.ids.ExternalID("vehicle", b.ID)
		if err != nil {
			return nil, err
		}
		speed := 0.0
		if hasPrev && cur.at > prev.at {
			speed = predictions.HaversineMeters(prev.lat, prev.lon, cur.lat, cur.lon) / float64(cur.at-prev.at) * 3.6
		}
		out = append(out, models.Vehicle{
			ID: id, Label: b.Label, Latitude: b.Lat, Longitude: b.Lon, Timestamp: at.UTC().Format(time.RFC3339),
			Speed: speed, RawSpeed: speed, RouteID: cur.routeID, TripID: models.PatternID(cur.routeID, cur.direction),
		})
	}
	f.last = seen
	return out, nil
}

type fit struct {
	routeID, direction int
	cut                bool
	along, off         float64
	progress           float64
	moving             bool
}

// follow keeps a bus on its pattern while it fits and does not go back along it, else takes the pattern it moves forward on, the only one that fits it, or the only one starting where it stands.
func (f *Follower) follow(lines []Line, prev reading, hasPrev bool, cur reading) (routeID, direction int, placed, moved bool) {
	moved = hasPrev && predictions.HaversineMeters(prev.fromLat, prev.fromLon, cur.lat, cur.lon) > live.StationaryRadiusMeters
	var fits []fit
	labelled := false
	for _, line := range lines {
		for direction := 0; direction <= 1; direction++ {
			along, off, ok := f.trace.Along(line.RouteID, direction, cur.lat, cur.lon)
			if !ok || off > predictions.MaxOffRouteMeters {
				continue
			}
			ft := fit{routeID: line.RouteID, direction: direction, cut: line.Cut, along: along, off: off}
			if moved {
				if before, prevOff, ok := f.trace.Along(line.RouteID, direction, prev.fromLat, prev.fromLon); ok && prevOff <= predictions.MaxOffRouteMeters {
					ft.progress, ft.moving = along-before, true
				}
			}
			fits = append(fits, ft)
			labelled = labelled || !line.Cut
		}
	}
	if labelled {
		// The line on the display wins whenever it fits.
		fits = slices.DeleteFunc(fits, func(ft fit) bool { return ft.cut })
	}
	if hasPrev && prev.placed {
		for _, ft := range fits {
			if ft.routeID == prev.routeID && ft.direction == prev.direction && (!ft.moving || ft.progress >= 0) {
				return ft.routeID, ft.direction, true, moved
			}
		}
	}
	var best *fit
	for i, ft := range fits {
		if ft.moving && ft.progress > 0 && (best == nil || ft.off < best.off) {
			best = &fits[i]
		}
	}
	if best != nil {
		return best.routeID, best.direction, true, moved
	}
	if len(fits) == 1 {
		return fits[0].routeID, fits[0].direction, true, moved
	}
	// Standing where one direction starts (and the other ends), a bus is about to leave on it.
	starting := slices.DeleteFunc(slices.Clone(fits), func(ft fit) bool { return ft.along > predictions.TerminusRadiusMeters })
	if len(starting) == 1 {
		return starting[0].routeID, starting[0].direction, true, moved
	}
	return 0, 0, false, moved
}
