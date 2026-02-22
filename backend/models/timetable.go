package models

type TimetableEntry struct {
	DepartureIn  string `json:"departure_in"`
	DepartureOut string `json:"departure_out"`
}

type Timetable struct {
	RouteShortName string           `json:"route_short_name"`
	RouteLongName  string           `json:"route_long_name"`
	InStopName     string           `json:"in_stop_name"`
	OutStopName    string           `json:"out_stop_name"`
	Weekdays       []TimetableEntry `json:"weekdays"`
	Saturday       []TimetableEntry `json:"saturday"`
	Sunday         []TimetableEntry `json:"sunday"`
}
