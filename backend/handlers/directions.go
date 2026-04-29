package handlers

import (
	"conexiuni-cluj/models"
	"conexiuni-cluj/services/ors"
	"encoding/json"
	"fmt"
)

// Skips HandleCached because every call has unique user coordinates.
func GetDirections(client *ors.Client, fromLat, fromLng, toLat, toLng float64) (models.DirectionsResponse, error) {
	raw, err := client.GetDirections("foot-walking", fromLat, fromLng, toLat, toLng)
	if err != nil {
		return models.DirectionsResponse{}, err
	}

	var orsResp struct {
		Routes []struct {
			Summary  models.DirectionsSummary `json:"summary"`
			Geometry string                   `json:"geometry"`
		} `json:"routes"`
	}
	if err := json.Unmarshal(raw, &orsResp); err != nil {
		return models.DirectionsResponse{}, fmt.Errorf("directions: parse ORS response: %w", err)
	}

	routes := make([]models.DirectionsRoute, 0, len(orsResp.Routes))
	for _, r := range orsResp.Routes {
		routes = append(routes, models.DirectionsRoute{Summary: r.Summary, Geometry: r.Geometry})
	}
	return models.DirectionsResponse{Routes: routes}, nil
}
