package models

type StopInfo struct {
	StopID   int    `json:"stop_id" db:"stop_id"`
	StopName string `json:"stop_name" db:"stop_name"`
}
