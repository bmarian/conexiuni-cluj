package models

type RequestStopTime struct {
	TripID       string `json:"trip_id"`
	StopID       int    `json:"stop_id"`
	StopSequence int    `json:"stop_sequence"`
}

type StopTime struct {
	TripID            string  `json:"trip_id" db:"trip_id"`
	OffsetArrivalTime float64 `json:"offset_arrival_time" db:"arrival_time"`
	StopID            int     `json:"stop_id" db:"stop_id"`
	StopSequence      int     `json:"stop_sequence" db:"stop_sequence"`
	StopHeadsign      string  `json:"stop_headsign" db:"stop_headsign"`
}
