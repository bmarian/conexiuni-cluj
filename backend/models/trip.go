package models

import (
	"encoding/json"
)

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
	ShapeID              string        `json:"shape_id" db:"shape_id"`
	WheelchairAccessible int           `json:"wheelchair_accessible" db:"wheelchair_accessible"`
	BikesAllowed         int           `json:"bikes_allowed" db:"bikes_allowed"`
}

func (t *Trip) UnmarshalJSON(data []byte) error {
	var raw struct {
		TripID               string        `json:"trip_id"`
		RouteID              int           `json:"route_id"`
		DirectionID          DirectionType `json:"direction_id"`
		TripHeadsign         string        `json:"trip_headsign"`
		BlockID              int           `json:"block_id"`
		ShapeID              string        `json:"shape_id"`
		WheelchairAccessible *int          `json:"wheelchair_accessible"`
		BikesAllowed         *int          `json:"bikes_allowed"`
	}

	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	t.TripID = raw.TripID
	t.RouteID = raw.RouteID
	t.DirectionID = raw.DirectionID
	t.TripHeadsign = raw.TripHeadsign
	t.BlockID = raw.BlockID
	t.ShapeID = raw.ShapeID

	if raw.WheelchairAccessible != nil {
		t.WheelchairAccessible = *raw.WheelchairAccessible
	} else {
		t.WheelchairAccessible = -1
	}

	if raw.BikesAllowed != nil {
		t.BikesAllowed = *raw.BikesAllowed
	} else {
		t.BikesAllowed = -1
	}

	return nil
}
