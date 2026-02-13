package models

import (
	"database/sql"
)

type Vehicle struct {
	ID                   string          `json:"id" db:"id"`
	Label                string          `json:"label" db:"label"`
	Latitude             sql.NullFloat64 `json:"latitude,omitempty" db:"latitude"`
	Longitude            sql.NullFloat64 `json:"longitude,omitempty" db:"longitude"`
	Timestamp            string          `json:"timestamp" db:"timestamp"`
	VehicleType          int             `json:"vehicleType" db:"vehicle_type"`
	BikeAccessible       sql.NullString  `json:"bikeAccessible,omitempty" db:"bike_accessible"`
	WheelchairAccessible sql.NullString  `json:"wheelchairAccessible,omitempty" db:"wheelchair_accessible"`
	Speed                sql.NullFloat64 `json:"speed,omitempty" db:"speed"`
	RouteID              sql.NullInt64   `json:"routeId,omitempty" db:"route_id"`
	TripID               sql.NullString  `json:"tripId,omitempty" db:"trip_id"`
}
