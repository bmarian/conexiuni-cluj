package models

import "database/sql"

type Timetable struct {
	RouteShortName string         `json:"route_short_name" db:"route_short_name"`
	RouteLongName  sql.NullString `json:"route_long_name" db:"route_long_name"`
	Weekdays       sql.NullString `json:"weekdays" db:"weekdays"`
	Saturday       sql.NullString `json:"saturday" db:"saturday"`
	Sunday         sql.NullString `json:"sunday" db:"sunday"`
}
