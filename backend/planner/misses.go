package planner

import (
	"fmt"
	"strings"

	"conexiuni-cluj/router"
	"conexiuni-cluj/transit"
	"conexiuni-cluj/web"

	"github.com/gofiber/fiber/v3"
)

const (
	PlaceMissMetric = "place_miss"
	PlanMissMetric  = "plan_miss"
)

// The search screen finds lines and stops itself.
func (s *Service) notePlaceMiss(c fiber.Ctx, q string) {
	if !web.IsPerson(c) {
		return
	}
	slug := router.Slug(q)
	if slug == "" {
		return
	}
	if found, err := s.namedElsewhere(slug); err != nil || found {
		return
	}
	s.store.RecordEvent(PlaceMissMetric, strings.ReplaceAll(slug, "-", " "))
}

func (s *Service) namedElsewhere(slug string) (bool, error) {
	routes, err := s.data.Routes(transit.RouteFilter{})
	if err != nil {
		return false, err
	}
	for _, r := range routes {
		if strings.Contains(router.Slug(r.RouteShortName), slug) || strings.Contains(router.Slug(r.RouteLongName), slug) {
			return true, nil
		}
	}
	stops, err := s.data.Stops(transit.StopFilter{})
	if err != nil {
		return false, err
	}
	for _, st := range stops {
		if strings.Contains(router.Slug(st.StopName), slug) {
			return true, nil
		}
	}
	return false, nil
}

// Keys "reason|from|to"; reason "outside" the map, or "walk": only a walk longer than any walk to a stop.
func (s *Service) notePlanMiss(c fiber.Ctx, n *router.Network, q router.Query, options []router.Option, covered bool) {
	if !web.IsPerson(c) || q.Last {
		return
	}
	reason := "outside"
	if covered {
		for _, o := range options {
			if o.Rides > 0 || o.Runs[0].Arrive-o.Runs[0].Depart <= int64(plannerMaxWalk.Seconds()) {
				return
			}
		}
		if !n.InService(q.Time.Unix()) {
			return
		}
		reason = "walk"
	}
	s.store.RecordEvent(PlanMissMetric, reason+"|"+s.placeName(q.From)+"|"+s.placeName(q.To))
}

// Coarse on purpose: no exact origin or destination is kept.
func (s *Service) placeName(p router.Point) string {
	if places := s.Places(); places != nil {
		if place, ok := places.Nearby(p.Lat, p.Lon); ok {
			return strings.ReplaceAll(place.Label, "|", " ")
		}
	}
	return fmt.Sprintf("%.2f, %.2f", p.Lat, p.Lon)
}
