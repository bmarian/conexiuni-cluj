// Package sources turns each operator's feeds into the app's own shape.
package sources

import (
	"conexiuni-cluj/models"
	"time"
)

// Trips are patterns, one per route and direction, with id "{route_id}_{0|1}".
type Snapshot struct {
	Routes    []models.Route
	Stops     []models.Stop
	Trips     []models.Trip
	Shapes    []models.Shape
	TripStops []models.TripStop
	// Seconds from the previous stop, by pattern id; only feeds with times fill it.
	Offsets map[string][]float64
	// The same by pattern id, kind of day (calendar.ScheduleDays) and hour of first-stop departure, 24 hours each; -1 where no run times a segment.
	HourlyOffsets map[string]map[string][][]float64
	// By route short name.
	Timetables map[string]*models.Timetable
}

type IDMap interface {
	ExternalID(kind, external string) (int, error)
}

type Static interface {
	Snapshot() (*Snapshot, error)
	Cadence() time.Duration
}

type Timetables interface {
	Timetables(snap *Snapshot) (map[string]*models.Timetable, error)
	Cadence() time.Duration
}

type Budget struct {
	Remaining int
	// Zero means unmetered.
	Limit int
}

type Realtime interface {
	// TripID is already a pattern id; vehicles off any pattern are left out.
	Vehicles() ([]models.Vehicle, error)
	Budget() Budget
}

// StopIndex is the stop's position on a direction's pattern, -1 when that direction does not serve it.
type Placement interface {
	StopIndex(routeID, direction, stopID int) int
}

type Credit struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type Credited interface {
	Credit() Credit
}

type Quota struct {
	Name  string
	Used  int
	Limit int
}

type Metered interface {
	Quotas() []Quota
}
