// Package updates tells riders what changed: the lines whose timetable or stops changed at an import, and the operator's news.
package updates

import (
	"conexiuni-cluj/database"
	"conexiuni-cluj/transit"
	"time"

	"github.com/gofiber/fiber/v3"
)

type Service struct {
	store *database.Store
	data  *transit.Data
	loc   *time.Location
	// Whether recorded changes wait for the push sender, and how to wake it.
	PushPending func() bool
	Notify      func()
}

func New(store *database.Store, data *transit.Data, loc *time.Location) *Service {
	return &Service{store: store, data: data, loc: loc}
}

func (s *Service) Register(api fiber.Router) {
	api.Get("/route-changes", s.routeChangesHandler)
	api.Get("/updates", s.updatesHandler)
}
