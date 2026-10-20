// Package push notifies riders on their phones: line changes for the lines they follow, and reminders for the buses they take.
package push

import (
	"conexiuni-cluj/database"
	"time"

	"github.com/gofiber/fiber/v3"
)

// Live is what reminders read: when a bus is due at a stop, and which timetable a day runs.
type Live interface {
	ArrivalAt(tripID string, stopID int, slot time.Time) (time.Time, bool)
	ScheduleDay(t time.Time) string
}

type Service struct {
	slug  string
	loc   *time.Location
	store *database.Store
	live  Live
	wake  chan struct{}
}

func New(slug string, loc *time.Location, store *database.Store, live Live) *Service {
	return &Service{slug: slug, loc: loc, store: store, live: live}
}

// Enabled is false until Configure got the keys.
func Enabled() bool { return pushCfg != nil }

func (s *Service) Register(api fiber.Router) {
	api.Get("/push/key", key)
	api.Post("/push/subscribe", s.subscribe)
	api.Post("/push/unsubscribe", s.unsubscribe)
	api.Post("/push/reminders", s.syncReminders)
}

// Counts is how many follow lines, how many reminders there are and how many people set them.
func (s *Service) Counts() (followers, reminders, reminderUsers int, err error) {
	err = s.store.DB.QueryRow(`
		SELECT (SELECT COUNT(DISTINCT endpoint) FROM push_subscription_routes),
		       (SELECT COUNT(*) FROM push_reminders),
		       (SELECT COUNT(DISTINCT endpoint) FROM push_reminders)`).Scan(&followers, &reminders, &reminderUsers)
	return followers, reminders, reminderUsers, err
}

// SubscribedElsewhere counts the subscriptions made on another host than host.
func (s *Service) SubscribedElsewhere(host string) (int, error) {
	var n int
	err := s.store.DB.QueryRow(`SELECT COUNT(*) FROM push_subscriptions WHERE host <> ?`, host).Scan(&n)
	return n, err
}
