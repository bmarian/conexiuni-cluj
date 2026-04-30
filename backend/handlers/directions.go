package handlers

import (
	"conexiuni-cluj/database"
	"conexiuni-cluj/models"
	"conexiuni-cluj/services/ors"
	"encoding/json"
	"fmt"
	"math"
	"time"
)

func GetDirections(client *ors.Client, cacheShelfLife time.Duration, fromLat, fromLng, toLat, toLng float64) (models.DirectionsResponse, error) {
	// Round to ~5 meters precision to increase cache hits for nearby requests.
	// 0.000045 degrees is roughly 5 meters.
	const step = 0.000045
	rFromLat := math.Round(fromLat/step) * step
	rFromLng := math.Round(fromLng/step) * step
	rToLat := math.Round(toLat/step) * step
	rToLng := math.Round(toLng/step) * step

	cacheID := fmt.Sprintf("DIR_%.5f_%.5f_%.5f_%.5f", rFromLat, rFromLng, rToLat, rToLng)

	return HandleCached(cacheID, cacheShelfLife,
		func() (models.DirectionsResponse, error) { return getDirectionsFromDB(cacheID) },
		func() (models.DirectionsResponse, error) {
			return fetchDirectionsFromORS(client, fromLat, fromLng, toLat, toLng)
		},
		func(data models.DirectionsResponse) error { return storeDirectionsInDB(cacheID, data) },
		CacheOpts[models.DirectionsResponse]{},
	)
}

func fetchDirectionsFromORS(client *ors.Client, fromLat, fromLng, toLat, toLng float64) (models.DirectionsResponse, error) {
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

func getDirectionsFromDB(cacheID string) (models.DirectionsResponse, error) {
	var dataStr string
	err := database.DB.QueryRow(`SELECT data FROM directions WHERE id = ?`, cacheID).Scan(&dataStr)
	if err != nil {
		return models.DirectionsResponse{}, err
	}

	var data models.DirectionsResponse
	if err := json.Unmarshal([]byte(dataStr), &data); err != nil {
		return models.DirectionsResponse{}, fmt.Errorf("failed to unmarshal directions from DB: %w", err)
	}
	return data, nil
}

func storeDirectionsInDB(cacheID string, data models.DirectionsResponse) error {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal directions for DB: %w", err)
	}

	_, err = database.DB.Exec(`
		INSERT OR REPLACE INTO directions (id, data)
		VALUES (?, ?)
	`, cacheID, string(dataBytes))
	return err
}
