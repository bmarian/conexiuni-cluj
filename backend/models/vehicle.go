package models

import (
	"encoding/json"
)

type Vehicle struct {
	ID                   int     `json:"id" db:"id"`
	Label                string  `json:"label" db:"label"`
	Latitude             float64 `json:"latitude" db:"latitude"`
	Longitude            float64 `json:"longitude" db:"longitude"`
	Timestamp            string  `json:"timestamp" db:"timestamp"`
	VehicleType          int     `json:"vehicle_type" db:"vehicle_type"`
	BikeAccessible       string  `json:"bike_accessible" db:"bike_accessible"`
	WheelchairAccessible string  `json:"wheelchair_accessible" db:"wheelchair_accessible"`
	Speed                float64 `json:"speed" db:"speed"`
	RouteID              int     `json:"route_id" db:"route_id"`
	TripID               string  `json:"trip_id" db:"trip_id"`

	// RawSpeed is Tranzy's reading before the EMA smoothing and the minimum-speed
	// floor applied to Speed. Speed is floored so ETA maths never divides by ~0,
	// which also makes it useless for telling "parked" from "crawling" — that is
	// what RawSpeed and StationarySince are for.
	RawSpeed float64 `json:"raw_speed" db:"raw_speed"`
	// StationarySince is the vehicle timestamp at which it was last seen more
	// than StationaryRadiusMeters away from where it sits now. Empty only until
	// the first fix. now - StationarySince is how long it has been standing.
	StationarySince string `json:"stationary_since" db:"anchor_at"`

	// anchor position StationarySince is measured from; persisted, not serialized.
	anchorLat  float64
	anchorLon  float64
	observedAt int64
}

func (v *Vehicle) SetAnchor(lat, lon float64, at string, observedAt int64) {
	v.anchorLat = lat
	v.anchorLon = lon
	v.StationarySince = at
	v.observedAt = observedAt
}

func (v *Vehicle) Anchor() (lat, lon float64) { return v.anchorLat, v.anchorLon }

func (v *Vehicle) ObservedAt() int64 { return v.observedAt }

func (v *Vehicle) UnmarshalJSON(data []byte) error {
	var raw struct {
		ID                   int      `json:"id"`
		Label                string   `json:"label"`
		Latitude             *float64 `json:"latitude"`
		Longitude            *float64 `json:"longitude"`
		Timestamp            string   `json:"timestamp"`
		VehicleType          int      `json:"vehicle_type"`
		BikeAccessible       *string  `json:"bike_accessible"`
		WheelchairAccessible *string  `json:"wheelchair_accessible"`
		Speed                *float64 `json:"speed"`
		RouteID              *int     `json:"route_id"`
		TripID               *string  `json:"trip_id"`
	}

	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	v.ID = raw.ID
	v.Label = raw.Label
	if raw.Latitude != nil {
		v.Latitude = *raw.Latitude
	} else {
		v.Latitude = -1.0
	}
	if raw.Longitude != nil {
		v.Longitude = *raw.Longitude
	} else {
		v.Longitude = -1.0
	}
	v.Timestamp = raw.Timestamp
	v.VehicleType = raw.VehicleType
	if raw.BikeAccessible != nil {
		v.BikeAccessible = *raw.BikeAccessible
	} else {
		v.BikeAccessible = "UNKNOWN"
	}
	if raw.WheelchairAccessible != nil {
		v.WheelchairAccessible = *raw.WheelchairAccessible
	} else {
		v.WheelchairAccessible = "UNKNOWN"
	}
	if raw.Speed != nil {
		v.Speed = *raw.Speed
	} else {
		v.Speed = -1.0
	}
	v.RawSpeed = v.Speed
	if raw.RouteID != nil {
		v.RouteID = *raw.RouteID
	} else {
		v.RouteID = -1
	}
	if raw.TripID != nil {
		v.TripID = *raw.TripID
	} else {
		v.TripID = "-1"
	}

	return nil
}
