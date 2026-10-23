// Package planner answers /plan, /places and /areas: journeys on foot and by line over the city's network, and the places a rider can type.
package planner

import (
	"conexiuni-cluj/database"
	"conexiuni-cluj/models"
	"conexiuni-cluj/router"
	"conexiuni-cluj/transit"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v3"
)

// As many runs of an option as a stop page shows.
const runsShown = 3

// Timing is what the planner reads from the city's live side.
type Timing interface {
	// TimedStopTimes are a line's stop times with the travel times learned for the moment at.
	TimedStopTimes(stopTimes []models.StopTime, routeID int, at time.Time) []models.StopTime
	// ProfileKey is equal for two moments that get the same learned times.
	ProfileKey(t time.Time) string
	ScheduleDay(t time.Time) string
	// Period changes whenever the learned times may have, so networks built before are built again.
	Period() int64
	// TransferSlack is the margin kept between alighting and the next boarding.
	TransferSlack() time.Duration
	// Horizon is how far ahead the live arrivals reach.
	Horizon() time.Duration
	// LiveAt is what the stop pages show for a plan leaving at when, and when such a plan is worth asking again; nil without a fresh table.
	LiveAt(when time.Time) (router.LiveSource, func(firsts []int64) int64)
	// Ride is a pattern's shape between two of its stops.
	Ride(tripID string, from, to int) ([]models.Shape, bool)
}

type Service struct {
	loc    *time.Location
	store  *database.Store
	data   *transit.Data
	timing Timing
	// Lines the city ticket does not cover.
	OwnTicket func(routeShortName string) bool

	mu       sync.Mutex
	stopsKey uint64
	streets  *router.Streets
	period   int64
	networks map[string]*router.Network

	// Read without mu, which the walk graph's read holds for seconds.
	places atomic.Pointer[router.Places]
}

func New(loc *time.Location, store *database.Store, data *transit.Data, timing Timing) *Service {
	return &Service{loc: loc, store: store, data: data, timing: timing}
}

func (s *Service) Register(api fiber.Router) {
	api.Get("/plan", s.plan)
	api.Get("/places", s.searchPlaces)
	api.Get("/areas", s.listAreas)
}

// Forget drops the built networks, for a calendar that now reads differently.
func (s *Service) Forget() {
	s.mu.Lock()
	s.networks = nil
	s.mu.Unlock()
}
