package network

import (
	"conexiuni-cluj/database"
	"conexiuni-cluj/jobs"
	"conexiuni-cluj/models"
	"conexiuni-cluj/transit"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
)

// Timing is what the bundle reads from the city's live side.
type Timing interface {
	// TimedStopTimes are a line's stop times with the travel times learned for the moment at.
	TimedStopTimes(stopTimes []models.StopTime, routeID int, at time.Time) []models.StopTime
	// DayOfType is the first day from from on that runs dayType's timetable.
	DayOfType(from time.Time, dayType string) time.Time
	// ScheduleDays are the days ahead whose timetable is not the usual one, and why.
	ScheduleDays(now time.Time) (map[string]string, map[string]models.DayNote)
	// GraceSec is how late a bus still counts as on time.
	GraceSec() float64
	// Period changes whenever the learned times may have, so a bundle built before is built again.
	Period() int64
}

// The area the city's lines cover.
type Bounds struct {
	South float64 `json:"south"`
	West  float64 `json:"west"`
	North float64 `json:"north"`
	East  float64 `json:"east"`
}

type Service struct {
	loc     *time.Location
	store   *database.Store
	data    *transit.Data
	offered *transit.Availability
	timing  Timing
	// Runner is the city's background work, which rebuilds a stale bundle.
	Runner func() *jobs.Runner

	buildMu  sync.Mutex
	mu       sync.Mutex
	current  *builtNetwork
	building bool
}

func New(loc *time.Location, store *database.Store, data *transit.Data, offered *transit.Availability, timing Timing) *Service {
	return &Service{loc: loc, store: store, data: data, offered: offered, timing: timing, Runner: jobs.None}
}

func (s *Service) Register(api fiber.Router) {
	api.Get("/network", s.serve)
	api.Get("/network/version", s.serveVersion)
}

// Invalidate keeps serving the bundle but has it built again, for a calendar that now reads differently.
func (s *Service) Invalidate() {
	s.mu.Lock()
	if s.current != nil {
		stale := *s.current
		stale.period = -1
		s.current = &stale
	}
	s.mu.Unlock()
}

// Bounds is nil until a bundle is built.
func (s *Service) Bounds() *Bounds {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.current == nil {
		return nil
	}
	return s.current.bounds
}
