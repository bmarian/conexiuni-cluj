package models

type PlaydateStopRef struct {
	StopID   int    `json:"stop_id"`
	StopName string `json:"stop_name"`
	// OffsetSeconds is cumulative seconds from this direction's first stop,
	// summed from the per-segment offset_arrival_time values GET /api/stop_times
	// already computes. Lets the Playdate app approximate where a scheduled
	// trip currently is between stops (elapsed-since-departure vs this
	// cumulative offset) without shipping live vehicle data.
	OffsetSeconds int `json:"offset_seconds"`
}

// PlaydateDirection is one direction (out or in) of a route: its headsign and
// the ordered stop sequence, ready to draw as a flat line with no further
// lookups.
type PlaydateDirection struct {
	Headsign string            `json:"headsign"`
	Stops    []PlaydateStopRef `json:"stops"`
}

type PlaydateRoute struct {
	RouteID        int                          `json:"route_id"`
	RouteShortName string                       `json:"route_short_name"`
	RouteLongName  string                       `json:"route_long_name"`
	RouteColor     string                       `json:"route_color"`
	Directions     map[string]PlaydateDirection `json:"directions"`
	Timetable      Timetable                    `json:"timetable"`
}

type PlaydateStop struct {
	StopID   int     `json:"stop_id"`
	StopName string  `json:"stop_name"`
	StopLat  float64 `json:"stop_lat"`
	StopLon  float64 `json:"stop_lon"`
}

// PlaydateExport is the full offline snapshot synced once by the Playdate
// app and browsed entirely from local storage afterward. See
// conexiuni-cluj-playdate/AGENTS.md for the consumer's side of this contract.
type PlaydateExport struct {
	GeneratedAt string          `json:"generated_at"`
	Routes      []PlaydateRoute `json:"routes"`
	Stops       []PlaydateStop  `json:"stops"`
}
