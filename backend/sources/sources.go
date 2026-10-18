// Package sources is what a city must feed the app: a static Snapshot in the app's own shape, and optionally
// its timetables and its vehicles. Every operator gets an adapter here; nothing past this package knows which.
package sources

import (
	"conexiuni-cluj/models"
	"time"
)

// Snapshot is the whole static network. Trips are patterns, one per route and direction, with id "{route_id}_{0|1}".
type Snapshot struct {
	Routes    []models.Route
	Stops     []models.Stop
	Trips     []models.Trip
	Shapes    []models.Shape
	TripStops []models.TripStop
	// By route short name; a source without departure times leaves it empty.
	Timetables map[string]*models.Timetable
}

type Static interface {
	Snapshot() (*Snapshot, error)
	// How often a fresh Snapshot is worth asking for.
	Cadence() time.Duration
}

// Timetables adds departure times to a Snapshot that has none of its own.
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
	// Vehicles on a pattern, with TripID already a pattern id. Vehicles off any pattern are left out.
	Vehicles() ([]models.Vehicle, error)
	Budget() Budget
}

type Quota struct {
	Name  string
	Used  int
	Limit int
}

// Metered sources report their daily quotas for the admin page.
type Metered interface {
	Quotas() []Quota
}
