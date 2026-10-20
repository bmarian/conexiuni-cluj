package transit

import (
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/models"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

func (d *Data) Timetable(routeShortName string) (*models.Timetable, error) {
	var t models.Timetable
	var weekdaysJSON, saturdayJSON, sundayJSON string

	err := d.db.QueryRow(
		`SELECT route_short_name, route_long_name, in_stop_name, out_stop_name, weekdays, saturday, sunday
		 FROM timetable WHERE route_short_name = ?`,
		routeShortName,
	).Scan(
		&t.RouteShortName,
		&t.RouteLongName,
		&t.InStopName,
		&t.OutStopName,
		&weekdaysJSON,
		&saturdayJSON,
		&sundayJSON,
	)
	if err != nil {
		return nil, fmt.Errorf("error querying timetable: %w", err)
	}

	if err := json.Unmarshal([]byte(weekdaysJSON), &t.Weekdays); err != nil {
		return nil, fmt.Errorf("error unmarshalling weekdays: %w", err)
	}
	if err := json.Unmarshal([]byte(saturdayJSON), &t.Saturday); err != nil {
		return nil, fmt.Errorf("error unmarshalling saturday: %w", err)
	}
	if err := json.Unmarshal([]byte(sundayJSON), &t.Sunday); err != nil {
		return nil, fmt.Errorf("error unmarshalling sunday: %w", err)
	}

	// Rows stored before the 24+ hour notation.
	feeds.NormalizeEntries(t.Weekdays.Entries)
	feeds.NormalizeEntries(t.Saturday.Entries)
	feeds.NormalizeEntries(t.Sunday.Entries)

	return &t, nil
}

// ParseDeparture is a timetable time ("07:25", "24:15*") in minutes after midnight.
func ParseDeparture(s string) (int, bool) {
	h, rest, ok := strings.Cut(strings.TrimLeft(strings.TrimSpace(s), "*"), ":")
	if !ok || len(rest) < 2 {
		return 0, false
	}
	hours, err := strconv.Atoi(h)
	if err != nil {
		return 0, false
	}
	minutes, err := strconv.Atoi(rest[:2])
	if err != nil {
		return 0, false
	}
	return hours*60 + minutes, true
}

// Departures are a direction's first-stop departures on a kind of day, in minutes, sorted; anything but saturday and sunday is a weekday.
func Departures(t *models.Timetable, dayType string, directionID int) []float64 {
	day := t.Weekdays
	switch dayType {
	case "saturday":
		day = t.Saturday
	case "sunday":
		day = t.Sunday
	}
	var out []float64
	for _, e := range day.Entries {
		raw := e.DepartureIn
		if directionID == 1 {
			raw = e.DepartureOut
		}
		if m, ok := ParseDeparture(raw); ok {
			out = append(out, float64(m))
		}
	}
	sort.Float64s(out)
	return out
}
