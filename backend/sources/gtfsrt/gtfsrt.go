// Package gtfsrt reads a GTFS-realtime vehiclePositions feed.
package gtfsrt

import (
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"conexiuni-cluj/models"
	"conexiuni-cluj/predictions"
	"conexiuni-cluj/sources"
)

type Source struct {
	url string
	ids sources.IDMap
	// Finds the direction when the feed leaves it out.
	Placement sources.Placement
	client    *http.Client

	mu sync.Mutex
	// The previous fixes, for feeds without speed.
	last map[int]fix
}

type fix struct {
	lat, lon float64
	at       int64
}

func New(url string, ids sources.IDMap) *Source {
	return &Source{url: url, ids: ids, client: &http.Client{Timeout: 30 * time.Second}, last: map[int]fix{}}
}

func (s *Source) Budget() sources.Budget { return sources.Budget{} }

func (s *Source) Vehicles() ([]models.Vehicle, error) {
	resp, err := s.client.Get(s.url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gtfsrt: %s: status %d", s.url, resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	feed, err := decode(data)
	if err != nil {
		return nil, err
	}
	return s.vehicles(feed, time.Now().Unix())
}

func (s *Source) vehicles(feed feedMessage, now int64) ([]models.Vehicle, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	seen := make(map[int]fix, len(feed.positions))
	out := make([]models.Vehicle, 0, len(feed.positions))
	for _, p := range feed.positions {
		if p.routeID == "" || p.vehicleID == "" {
			continue
		}
		routeID, err := s.ids.ExternalID("route", p.routeID)
		if err != nil {
			return nil, err
		}
		direction, ok := s.direction(routeID, p)
		if !ok {
			continue
		}
		id, err := s.ids.ExternalID("vehicle", p.vehicleID)
		if err != nil {
			return nil, err
		}
		at := p.timestamp
		if at == 0 {
			at = feed.timestamp
		}
		if at == 0 {
			at = now
		}
		reading := fix{lat: p.lat, lon: p.lon, at: at}
		speed := p.speed * 3.6
		if !p.hasSpeed {
			speed = speedKmh(s.last[id], reading)
		}
		label := p.label
		if label == "" {
			label = p.vehicleID
		}
		out = append(out, models.Vehicle{
			ID: id, Label: label, Latitude: p.lat, Longitude: p.lon,
			Timestamp: time.Unix(at, 0).UTC().Format(time.RFC3339),
			Speed:     speed, RawSpeed: speed,
			RouteID: routeID, TripID: fmt.Sprintf("%d_%d", routeID, direction),
		})
		seen[id] = reading
	}
	s.last = seen
	return out, nil
}

// Without the feed's own, the one direction serving the next stop, or at a shared terminus the one matching its sequence.
func (s *Source) direction(routeID int, p position) (int, bool) {
	if p.direction >= 0 {
		return p.direction, true
	}
	if s.Placement == nil || p.stopID == "" {
		return 0, false
	}
	stopID, err := s.ids.ExternalID("stop", p.stopID)
	if err != nil {
		return 0, false
	}
	var serving, exact []int
	for direction := 0; direction <= 1; direction++ {
		idx := s.Placement.StopIndex(routeID, direction, stopID)
		if idx < 0 {
			continue
		}
		serving = append(serving, direction)
		if idx == p.stopSequence {
			exact = append(exact, direction)
		}
	}
	switch {
	case len(serving) == 1:
		return serving[0], true
	case len(exact) == 1:
		return exact[0], true
	}
	return 0, false
}

func speedKmh(prev, cur fix) float64 {
	if prev.at == 0 || cur.at <= prev.at {
		return 0
	}
	return predictions.HaversineMeters(prev.lat, prev.lon, cur.lat, cur.lon) / float64(cur.at-prev.at) * 3.6
}
