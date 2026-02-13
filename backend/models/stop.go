package models

import "database/sql"

type Stop struct {
	StopID       int            `json:"stop_id" db:"stop_id"`
	StopName     string         `json:"stop_name" db:"stop_name"`
	StopDesc     sql.NullString `json:"stop_desc,omitempty" db:"stop_desc"`
	StopLat      float64        `json:"stop_lat" db:"stop_lat"`
	StopLon      float64        `json:"stop_lon" db:"stop_lon"`
	LocationType sql.NullInt64  `json:"location_type,omitempty" db:"location_type"`
	StopCode     sql.NullString `json:"stop_code,omitempty" db:"stop_code"`
}
