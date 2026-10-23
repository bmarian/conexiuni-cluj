package models

type NetworkBundle struct {
	Version       string             `json:"version"`
	StaticVersion string             `json:"static_version"`
	GeneratedAt   string             `json:"generated_at"`
	Routes        []NetworkRoute     `json:"routes"`
	Stops         []Stop             `json:"stops"`
	Rules         NetworkRules       `json:"rules"`
	ScheduleDays  map[string]string  `json:"schedule_days,omitempty"`
	DayNotes      map[string]DayNote `json:"day_notes,omitempty"`
}

type DayNote struct {
	Reason  string `json:"reason"`
	Holiday string `json:"holiday,omitempty"`
}

// What the learners found that an offline timetable list still needs.
type NetworkRules struct {
	// A timetable run this many minutes late is still coming.
	LateGraceMin float64 `json:"late_grace_min"`
}

type NetworkRoute struct {
	RouteID        int                `json:"route_id"`
	AgencyID       int                `json:"agency_id"`
	RouteShortName string             `json:"route_short_name"`
	RouteLongName  string             `json:"route_long_name"`
	RouteType      RouteType          `json:"route_type"`
	RouteDesc      string             `json:"route_desc"`
	RouteColor     string             `json:"route_color"`
	RouteColorful  string             `json:"route_colorful"`
	Timetable      *Timetable         `json:"timetable"`
	Trips          []NetworkTrip      `json:"trips"`
	Offsets        map[string][][]int `json:"offsets"`
}

type NetworkTrip struct {
	TripID        string `json:"trip_id"`
	StopIDs       []int  `json:"stop_ids"`
	StopSequences []int  `json:"stop_sequences"`
	ShapeHash     string `json:"shape_hash,omitempty"`
}

type NetworkVersion struct {
	Version       string `json:"version"`
	StaticVersion string `json:"static_version"`
	GeneratedAt   string `json:"generated_at"`
}
