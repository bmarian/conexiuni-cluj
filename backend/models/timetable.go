package models

type TimetableEntry struct {
	DepartureIn  string `json:"departure_in"`
	DepartureOut string `json:"departure_out"`
}

// Frequency describes a headway-based direction (no individual departures, just
// a service window and an "every N-M min" interval); nil when the direction
// lists explicit times.
type Frequency struct {
	Start      string `json:"start"`
	End        string `json:"end"`
	MinMinutes int    `json:"min_minutes"`
	MaxMinutes int    `json:"max_minutes"`
}

type DaySchedule struct {
	ServiceName  string           `json:"service_name"`
	ServiceStart string           `json:"service_start"`
	Entries      []TimetableEntry `json:"entries"`
	InFrequency  *Frequency       `json:"in_frequency,omitempty"`
	OutFrequency *Frequency       `json:"out_frequency,omitempty"`
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
