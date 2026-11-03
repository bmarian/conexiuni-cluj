// Package modeshift reads the buses of a Modeshift journey planner, each on the direction of the trip it names.
package modeshift

import (
	"encoding/json"
	"strconv"
	"time"

	"conexiuni-cluj/feeds"
	"conexiuni-cluj/models"
)

// Runs knows the direction of each trip the city's last GTFS import listed.
type Runs interface {
	RunDirections(runIDs []string) (map[string]int, error)
}

type Source struct {
	url  string
	ids  feeds.IDMap
	runs Runs
}

func New(url string, ids feeds.IDMap, runs Runs) *Source {
	return &Source{url: url, ids: ids, runs: runs}
}

func (s *Source) Budget() feeds.Budget { return feeds.Budget{} }

type bus struct {
	RouteID   int     `json:"routeId"`
	TripID    string  `json:"tripId"`
	VehicleID string  `json:"vehicleId"`
	Label     string  `json:"label"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	// m/s, left out while standing.
	Speed       float64   `json:"speed"`
	LastUpdated time.Time `json:"lastUpdated"`
}

func (s *Source) Vehicles() ([]models.Vehicle, error) {
	data, err := feeds.Download(s.url)
	if err != nil {
		return nil, err
	}
	var buses []bus
	if err := json.Unmarshal(data, &buses); err != nil {
		return nil, err
	}
	runIDs := make([]string, len(buses))
	for i, b := range buses {
		runIDs[i] = b.TripID
	}
	directions, err := s.runs.RunDirections(runIDs)
	if err != nil {
		return nil, err
	}
	out := make([]models.Vehicle, 0, len(buses))
	for _, b := range buses {
		direction, ok := directions[b.TripID]
		if !ok || b.VehicleID == "" {
			continue
		}
		routeID, err := s.ids.ExternalID("route", strconv.Itoa(b.RouteID))
		if err != nil {
			return nil, err
		}
		id, err := s.ids.ExternalID("vehicle", b.VehicleID)
		if err != nil {
			return nil, err
		}
		label := b.Label
		if label == "" {
			label = b.VehicleID
		}
		at := b.LastUpdated
		if at.IsZero() {
			at = time.Now()
		}
		speed := b.Speed * 3.6
		out = append(out, models.Vehicle{
			ID: id, Label: label, Latitude: b.Latitude, Longitude: b.Longitude,
			Timestamp: at.UTC().Format(time.RFC3339),
			Speed:     speed, RawSpeed: speed,
			RouteID: routeID, TripID: models.PatternID(routeID, direction),
		})
	}
	return out, nil
}
