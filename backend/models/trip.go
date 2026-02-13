package models

import "database/sql"

type DirectionType int

const (
	Outbound DirectionType = 0
	Inbound  DirectionType = 1
)

type Trip struct {
	TripID               string        `json:"trip_id" db:"trip_id"`
	RouteID              int           `json:"route_id" db:"route_id"`
	DirectionID          DirectionType `json:"direction_id" db:"direction_id"`
	TripHeadsign         string        `json:"trip_headsign" db:"trip_headsign"`
	BlockID              int           `json:"block_id" db:"block_id"`
	ShapeID              int           `json:"shape_id" db:"shape_id"`
	WheelchairAccessible sql.NullInt64 `json:"wheelchair_accessible,omitempty" db:"wheelchair_accessible"`
	BikesAllowed         sql.NullInt64 `json:"bikes_allowed,omitempty" db:"bikes_allowed"`
}
