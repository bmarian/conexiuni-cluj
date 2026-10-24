package targumures

import (
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/feeds/follow"
	"conexiuni-cluj/models"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// The buses on live.transportlocal.ro, Transport Local's own map: fleet number, line and position, rebuilt every few seconds; no direction or speed.
type realtime struct {
	url      string
	trace    feeds.Tracing
	follower *follow.Follower
}

func newRealtime(url string, ids feeds.IDMap, trace feeds.Tracing) *realtime {
	return &realtime{url: url, trace: trace, follower: follow.New(ids, trace)}
}

func (s *realtime) Budget() feeds.Budget { return feeds.Budget{} }

func (s *realtime) Vehicles() ([]models.Vehicle, error) {
	data, err := feeds.Download(s.url)
	if err != nil {
		return nil, fmt.Errorf("live.transportlocal.ro: %w", err)
	}
	buses, err := parseJourneys(data, s.trace.RouteIDs())
	if err != nil {
		return nil, fmt.Errorf("live.transportlocal.ro: %w", err)
	}
	return s.follower.Vehicles(buses, time.Now())
}

// A bus on no line of the network ("0" or empty out of service) is left out.
func parseJourneys(data []byte, routes map[string]int) ([]follow.Bus, error) {
	var feed struct {
		Journeys []struct {
			Vehicle   string    `json:"vehicle_id"`
			Route     string    `json:"route"`
			Lat       float64   `json:"lat"`
			Lon       float64   `json:"lon"`
			Timestamp time.Time `json:"timestamp"`
		} `json:"journeys"`
	}
	if err := json.Unmarshal(data, &feed); err != nil {
		return nil, err
	}
	var out []follow.Bus
	for _, j := range feed.Journeys {
		routeID, ok := routes[strings.TrimSpace(j.Route)]
		if !ok || j.Vehicle == "" || j.Lat == 0 || j.Lon == 0 {
			continue
		}
		out = append(out, follow.Bus{ID: j.Vehicle, Label: j.Vehicle, Lat: j.Lat, Lon: j.Lon, At: j.Timestamp, Lines: []follow.Line{{RouteID: routeID}}})
	}
	return out, nil
}
