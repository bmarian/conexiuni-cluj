package models

import _ "database/sql"

type RouteType int

const (
	Tram       RouteType = 0
	Subway     RouteType = 1
	Rail       RouteType = 2
	Bus        RouteType = 3
	Ferry      RouteType = 4
	CableTram  RouteType = 5
	AerialLift RouteType = 6
	Funicular  RouteType = 7
	Trolleybus RouteType = 11
	Monorail   RouteType = 12
)

type Route struct {
	RouteID        int       `json:"route_id" db:"route_id"`
	AgencyID       int       `json:"agency_id" db:"agency_id"`
	RouteShortName string    `json:"route_short_name" db:"route_short_name"`
	RouteLongName  string    `json:"route_long_name" db:"route_long_name"`
	RouteType      RouteType `json:"route_type" db:"route_type"`
	RouteDesc      string    `json:"route_desc" db:"route_desc"`
	RouteColor     string    `json:"route_color" db:"route_color"`
}
