package models

type PlaydateStopRef struct {
	StopID   int    `json:"stop_id"`
	StopName string `json:"stop_name"`
}

// PlaydateDirection is one direction (out or in) of a route: its headsign,
// the ordered stop sequence, and cumulative per-stop offsets broken out by
// hour of day.
//
// Segment travel-time profiles (what GET /api/stop_times?ref_hour= reads)
// vary by time of day -- a rush-hour segment takes longer than the same
// segment at 2am. This app syncs once and is then browsed offline for up to
// a day, so a single offset snapshot taken at sync time would drift
// increasingly wrong as the day goes on (e.g. sync at 8am, still using
// 8am's segment durations to estimate bus position at 8pm). HourlyOffsets
// avoids that: it has one cumulative-offset array per hour (0-23, local
// time), so the client picks the array matching its own current clock hour
// instead of a stale single value.
type PlaydateDirection struct {
	Headsign string            `json:"headsign"`
	Stops    []PlaydateStopRef `json:"stops"`
	// HourlyOffsets[hour][i] is the cumulative offset_seconds for Stops[i]
	// using that hour's learned-profile estimate (falls back to the
	// geometric estimate where no profile exists, same as GET /api/stop_times
	// always does). Keyed by hour as an int 0-23; Go's encoding/json renders
	// int map keys as JSON string keys ("0".."23"). An hour can be absent if
	// computing it failed -- clients should fall back to the nearest hour
	// present, or to hour 0, rather than assume all 24 exist.
	HourlyOffsets map[int][]int `json:"hourly_offset_seconds"`
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
