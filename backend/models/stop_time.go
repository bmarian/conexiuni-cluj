package models

type RequestStopTime struct {
	TripID       string `json:"trip_id"`
	StopID       int    `json:"stop_id"`
	StopSequence int    `json:"stop_sequence"`
}

type StopTime struct {
	TripID            string  `json:"trip_id" db:"trip_id"`
	StopID            int     `json:"stop_id" db:"stop_id"`
	OffsetArrivalTime float64 `json:"offset_arrival_time" db:"arrival_time"`
	OffsetConfidence  float64 `json:"offset_confidence"`
	StopSequence      int     `json:"stop_sequence" db:"stop_sequence"`
	StopHeadsign      string  `json:"stop_headsign" db:"stop_headsign"`
	RouteShortName    string  `json:"route_short_name" db:"route_short_name"`
	StopLat           float64 `json:"stop_lat" db:"stop_lat"`
	StopLon           float64 `json:"stop_lon" db:"stop_lon"`
}
