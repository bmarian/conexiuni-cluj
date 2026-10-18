// Package stpt reads the vehicles of STPT Timișoara's live map (live.stpt.ro).
package stpt

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"conexiuni-cluj/models"
	"conexiuni-cluj/sources"
)

type Source struct {
	url    string
	ids    sources.IDMap
	client *http.Client
}

func New(url string, ids sources.IDMap) *Source {
	return &Source{url: url, ids: ids, client: &http.Client{Timeout: 30 * time.Second}}
}

func (s *Source) Budget() sources.Budget { return sources.Budget{} }

// Ids are the SMTT GTFS feed's: routeId is its route_id, directionId its direction_id.
type answer struct {
	Success bool `json:"success"`
	Data    struct {
		Vehicles []vehicle `json:"vehicles"`
	} `json:"data"`
}

type vehicle struct {
	ID          string  `json:"id"`
	Lat         float64 `json:"lat"`
	Lng         float64 `json:"lng"`
	Speed       float64 `json:"speed"` // km/h
	RouteID     string  `json:"routeId"`
	DirectionID string  `json:"directionId"`
	Timestamp   int64   `json:"timestamp"` // ms
}

func (s *Source) Vehicles() ([]models.Vehicle, error) {
	resp, err := s.client.Get(s.url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("stpt: %s: status %d", s.url, resp.StatusCode)
	}
	var a answer
	if err := json.NewDecoder(resp.Body).Decode(&a); err != nil {
		return nil, err
	}
	return s.vehicles(a)
}

func (s *Source) vehicles(a answer) ([]models.Vehicle, error) {
	if !a.Success {
		return nil, errors.New("stpt: the vehicles answer says it failed")
	}
	out := make([]models.Vehicle, 0, len(a.Data.Vehicles))
	for _, v := range a.Data.Vehicles {
		direction, err := strconv.Atoi(v.DirectionID)
		if v.RouteID == "" || v.ID == "" || err != nil || direction < 0 || direction > 1 {
			continue
		}
		routeID, err := s.ids.ExternalID("route", v.RouteID)
		if err != nil {
			return nil, err
		}
		id, err := s.ids.ExternalID("vehicle", v.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, models.Vehicle{
			ID: id, Label: v.ID, Latitude: v.Lat, Longitude: v.Lng,
			Timestamp: time.UnixMilli(v.Timestamp).UTC().Format(time.RFC3339),
			Speed:     v.Speed, RawSpeed: v.Speed,
			RouteID: routeID, TripID: fmt.Sprintf("%d_%d", routeID, direction),
		})
	}
	return out, nil
}
