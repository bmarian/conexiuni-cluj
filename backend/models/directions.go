package models

type DirectionsSummary struct {
	Distance float64 `json:"distance"` // meters
	Duration float64 `json:"duration"` // seconds
}

type DirectionsRoute struct {
	Summary  DirectionsSummary `json:"summary"`
	Geometry string            `json:"geometry"` // Google-encoded polyline
}

type DirectionsResponse struct {
	Routes []DirectionsRoute `json:"routes"`
}
