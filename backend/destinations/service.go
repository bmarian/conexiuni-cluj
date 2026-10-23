// Package destinations answers which lines reach each area of a city, and from where.
package destinations

import (
	"conexiuni-cluj/models"
	"conexiuni-cluj/router"
	"conexiuni-cluj/web"
	"sync"

	"github.com/gofiber/fiber/v3"
)

type Network interface {
	Current() (*models.NetworkBundle, string, error)
	Version() (string, error)
}

type Service struct {
	network Network
	places  func() *router.Places

	mu      sync.Mutex
	version string
	from    *router.Places
	index   *Index
}

func New(network Network, places func() *router.Places) *Service {
	return &Service{network: network, places: places}
}

func (s *Service) Register(api fiber.Router) {
	api.Get("/destinations", s.list)
	api.Get("/destinations/:slug", s.get)
}

func (s *Service) Index() (*Index, error) {
	version, err := s.network.Version()
	if err != nil {
		return nil, err
	}
	places := s.places()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.index != nil && s.version == version && s.from == places {
		return s.index, nil
	}
	bundle, version, err := s.network.Current()
	if err != nil {
		return nil, err
	}
	var areas []router.Area
	if places != nil {
		areas = places.Areas()
	}
	s.index, s.version, s.from = build(bundle, areas), version, places
	return s.index, nil
}

func (s *Service) Areas() (*Index, bool, error) {
	x, err := s.Index()
	if err != nil {
		return nil, false, err
	}
	return x, s.places() != nil, nil
}

func loading(c fiber.Ctx) error {
	c.Set("Retry-After", "10")
	return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "destinations still loading", "retry_after_seconds": 10})
}

func (s *Service) list(c fiber.Ctx) error {
	x, ready, err := s.Areas()
	if err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": err.Error()})
	}
	if !ready {
		return loading(c)
	}
	c.Set("Cache-Control", web.Revalidate)
	return c.JSON(x.Summaries())
}

func (s *Service) get(c fiber.Ctx) error {
	x, ready, err := s.Areas()
	if err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": err.Error()})
	}
	if !ready {
		return loading(c)
	}
	dest := x.Destination(c.Params("slug"))
	if dest == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "no such destination"})
	}
	c.Set("Cache-Control", web.Revalidate)
	return c.JSON(dest)
}
