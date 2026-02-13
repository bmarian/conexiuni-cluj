package models

import "database/sql"

type ScheduleEntry struct {
	InTime  string `json:"in_time"`
	OutTime string `json:"out_time"`
}

type Schedule struct {
	ServiceName  string          `json:"service_name"`
	ServiceStart string          `json:"service_start"`
	InStopName   string          `json:"in_stop_name"`
	OutStopName  string          `json:"out_stop_name"`
	Times        []ScheduleEntry `json:"times"`
}

type Timetable struct {
	RouteShortName string         `json:"route_short_name" db:"route_short_name"`
	RouteLongName  sql.NullString `json:"route_long_name,omitempty" db:"route_long_name"`
	Weekdays       sql.NullString `json:"weekdays,omitempty" db:"weekdays"` // JSON string
	Saturday       sql.NullString `json:"saturday,omitempty" db:"saturday"` // JSON string
	Sunday         sql.NullString `json:"sunday,omitempty" db:"sunday"`     // JSON string
}
