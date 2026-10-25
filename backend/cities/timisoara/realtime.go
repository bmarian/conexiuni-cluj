package timisoara

import (
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"conexiuni-cluj/feeds"
	"conexiuni-cluj/models"
)

// The vehicles of live.stpt.ro, a Timișoara live map; its terms forbid automated use without permission.
type realtime struct {
	url string
	ids feeds.IDMap
}

func newRealtime(url string, ids feeds.IDMap) *realtime {
	return &realtime{url: url, ids: ids}
}

func (s *realtime) Budget() feeds.Budget { return feeds.Budget{} }

// Ids are the SMTT GTFS feed's.
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

func (s *realtime) Vehicles() ([]models.Vehicle, error) {
	data, err := feeds.Download(s.url)
	if err != nil {
		return nil, err
	}
	var a answer
	if err := json.Unmarshal(data, &a); err != nil {
		return nil, err
	}
	return s.vehicles(a)
}

func (s *realtime) vehicles(a answer) ([]models.Vehicle, error) {
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
			RouteID: routeID, TripID: models.PatternID(routeID, direction),
		})
	}
	return out, nil
}
