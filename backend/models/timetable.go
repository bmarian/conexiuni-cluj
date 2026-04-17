package models

type TimetableEntry struct {
	DepartureIn  string `json:"departure_in"`
	DepartureOut string `json:"departure_out"`
}

type DaySchedule struct {
	ServiceName  string           `json:"service_name"`
	ServiceStart string           `json:"service_start"`
	Entries      []TimetableEntry `json:"entries"`
}

type Timetable struct {
	RouteShortName string      `json:"route_short_name"`
	RouteLongName  string      `json:"route_long_name"`
	InStopName     string      `json:"in_stop_name"`
	OutStopName    string      `json:"out_stop_name"`
	Weekdays       DaySchedule `json:"weekdays"`
	Saturday       DaySchedule `json:"saturday"`
	Sunday         DaySchedule `json:"sunday"`
}
