package models

import "strconv"

// A pattern is one direction of a line, with its own stops: "{route_id}_0" one way and "{route_id}_1" back.
const (
	OutgoingSuffix = "_0"
	IncomingSuffix = "_1"
)

func PatternID(routeID, direction int) string {
	return strconv.Itoa(routeID) + "_" + strconv.Itoa(direction)
}

// PatternDirection is a pattern id's direction, 0 or 1.
func PatternDirection(id string) (int, bool) {
	switch {
	case len(id) >= 2 && id[len(id)-2:] == OutgoingSuffix:
		return 0, true
	case len(id) >= 2 && id[len(id)-2:] == IncomingSuffix:
		return 1, true
	default:
		return 0, false
	}
}
