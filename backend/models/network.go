package models

type NetworkBundle struct {
	Version       string         `json:"version"`
	StaticVersion string         `json:"static_version"`
	GeneratedAt   string         `json:"generated_at"`
	Routes        []NetworkRoute `json:"routes"`
	Stops         []Stop         `json:"stops"`
}

type NetworkRoute struct {
	RouteID        int                `json:"route_id"`
	AgencyID       int                `json:"agency_id"`
	RouteShortName string             `json:"route_short_name"`
	RouteLongName  string             `json:"route_long_name"`
	RouteType      RouteType          `json:"route_type"`
	RouteDesc      string             `json:"route_desc"`
	RouteColor     string             `json:"route_color"`
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
