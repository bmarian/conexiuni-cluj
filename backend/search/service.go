// Package search answers /search: lines, stops, places, buses, cities and trips in one ranked answer.
package search

import (
	"slices"
	"strings"
	"sync"

	"conexiuni-cluj/database"
	"conexiuni-cluj/destinations"
	"conexiuni-cluj/live"
	"conexiuni-cluj/models"
	"conexiuni-cluj/router"
	"conexiuni-cluj/web"

	"github.com/gofiber/fiber/v3"
)

type Network interface {
	Current() (*models.NetworkBundle, string, error)
}

type Service struct {
	Here    City
	network Network
	places  func() *router.Places
	streets func() *router.Streets
	lines   func() (*destinations.Index, error)
	buses   func() []live.Bus
	store   *database.Store
	Cities  func() []City
	// Empty until read.
	Towns func() []router.Town

	mu      sync.Mutex
	version string
	named   *names
}

func New(here City, network Network, places func() *router.Places, streets func() *router.Streets,
	lines func() (*destinations.Index, error), buses func() []live.Bus, store *database.Store) *Service {
	return &Service{Here: here, network: network, places: places, streets: streets, lines: lines, buses: buses, store: store,
		Cities: func() []City { return nil }, Towns: func() []router.Town { return nil }}
}

func (s *Service) Register(api fiber.Router) {
	api.Get("/search", s.serve)
}

type namedStop struct {
	stop *models.Stop
	name string
}

type namedRoute struct {
	route *models.NetworkRoute
	name  string
}

// Folded names of the network, one stop per station.
type names struct {
	bundle *models.NetworkBundle
	stops  []namedStop
	routes []namedRoute
}

func (s *Service) names() (*names, error) {
	bundle, version, err := s.network.Current()
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.named != nil && s.version == version {
		return s.named, nil
	}
	n := &names{bundle: bundle}
	seen := map[int]bool{}
	for i := range bundle.Stops {
		st := &bundle.Stops[i]
		if !seen[st.Station()] {
			seen[st.Station()] = true
			n.stops = append(n.stops, namedStop{st, router.Fold(st.StopName)})
		}
	}
	slices.SortFunc(n.stops, func(a, b namedStop) int { return strings.Compare(a.name, b.name) })
	for i := range bundle.Routes {
		r := &bundle.Routes[i]
		n.routes = append(n.routes, namedRoute{r, router.Fold(r.RouteShortName + " " + r.RouteLongName)})
	}
	s.named, s.version = n, version
	return n, nil
}

func (s *Service) serve(c fiber.Ctx) error {
	q := strings.TrimSpace(c.Query("q"))
	if q == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "q is required"})
	}
	n, err := s.names()
	if err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": err.Error()})
	}
	a := s.find(n, q)
	if web.IsPerson(c) {
		s.noteMiss(q, a)
	}
	c.Set("Cache-Control", web.Revalidate)
	return c.JSON(a)
}
