package models

type StopInfo struct {
	StopID          int          `json:"stop_id"`
	StopName        string       `json:"stop_name"`
	StopDesc        string       `json:"stop_desc"`
	StopLat         float64      `json:"stop_lat"`
	StopLon         float64      `json:"stop_lon"`
	LocationType    LocationType `json:"location_type"`
	StopCode        string       `json:"stop_code"`
	OutgoingTripIds []string     `json:"outgoing_trip_ids"`
	IncomingTripIds []string     `json:"incoming_trip_ids"`
	ShapesInfo      []ShapeInfo  `json:"shapes_info"`
}

type ShapeInfo struct {
	RouteShortName string     `json:"route_short_name"`
	StopTimes      []StopTime `json:"stop_time"`
	Timetable      Timetable  `json:"timetable"`
}

type StopInfoDB struct {
	StopID          int      `json:"stop_id" db:"stop_id"`
	TripIDs         []string `json:"trip_ids" db:"trip_ids"`
	ShapesShortName []string `json:"shapes_short_name" db:"shapes_short_name"`
}
