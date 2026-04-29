package handlers

import (
	"conexiuni-cluj/services/ors"
	"encoding/json"
)

// Skips HandleCached because every call has unique user coordinates.
func GetDirections(client *ors.Client, fromLat, fromLng, toLat, toLng float64) (json.RawMessage, error) {
	return client.GetDirections("foot-walking", fromLat, fromLng, toLat, toLng)
}
