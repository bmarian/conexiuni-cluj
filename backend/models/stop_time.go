package models

import "database/sql"

type StopTime struct {
	TripID            string          `json:"trip_id" db:"trip_id"`
	ArrivalTime       sql.NullString  `json:"arrival_time,omitempty" db:"arrival_time"`
	DepartureTime     sql.NullString  `json:"departure_time,omitempty" db:"departure_time"`
	StopID            int             `json:"stop_id" db:"stop_id"`
	StopSequence      int             `json:"stop_sequence" db:"stop_sequence"`
	StopHeadsign      sql.NullString  `json:"stop_headsign,omitempty" db:"stop_headsign"`
	PickupType        sql.NullInt64   `json:"pickup_type,omitempty" db:"pickup_type"`
	DropOffType       sql.NullInt64   `json:"drop_off_type,omitempty" db:"drop_off_type"`
	ShapeDistTraveled sql.NullFloat64 `json:"shape_dist_traveled,omitempty" db:"shape_dist_traveled"`
	Timepoint         sql.NullInt64   `json:"timepoint,omitempty" db:"timepoint"`
}
