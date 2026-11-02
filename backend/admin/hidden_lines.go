package admin

import (
	"conexiuni-cluj/cities"
	"conexiuni-cluj/transit"
	"sort"
	"time"

	"github.com/gofiber/fiber/v3"
)

const idleAfter = 7 * 24 * time.Hour

type adminLine struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Color    string `json:"color,omitempty"`
	LongName string `json:"long_name"`
	// Unix seconds, 0 for never.
	LastRun int64 `json:"last_run"`
}

type hiddenLinesAnswer struct {
	Hidden []adminLine `json:"hidden"`
	Idle   []adminLine `json:"idle"`
}

func hiddenLines(city *cities.City, now time.Time) (hiddenLinesAnswer, error) {
	out := hiddenLinesAnswer{Hidden: []adminLine{}, Idle: []adminLine{}}
	routes, err := city.Routes(transit.RouteFilter{})
	if err != nil {
		return out, err
	}
	runs, err := city.LastRuns()
	if err != nil {
		return out, err
	}
	ids := make([]int, len(routes))
	for i, r := range routes {
		ids[i] = r.RouteID
	}
	labels, err := routeNames(city, ids)
	if err != nil {
		return out, err
	}

	byName := map[string]*adminLine{}
	var names []string
	running := false
	for _, r := range routes {
		line := byName[r.RouteShortName]
		if line == nil {
			line = &adminLine{ID: r.RouteID, Name: r.RouteShortName, Color: labels[r.RouteID].Color, LongName: r.RouteLongName}
			byName[r.RouteShortName] = line
			names = append(names, r.RouteShortName)
		}
		if at, ok := runs[r.RouteID]; ok {
			line.LastRun = max(line.LastRun, at.Unix())
			running = running || now.Sub(at) < idleAfter
		}
	}
	for _, name := range city.HiddenLines() {
		if line := byName[name]; line != nil {
			out.Hidden = append(out.Hidden, *line)
		} else {
			out.Hidden = append(out.Hidden, adminLine{Name: name})
		}
	}
	// No bus at all this week means no live data to judge by.
	if running {
		offered := city.Availability()
		for _, name := range names {
			line := byName[name]
			if offered.RouteHasTimetable(name) && now.Sub(time.Unix(line.LastRun, 0)) >= idleAfter {
				out.Idle = append(out.Idle, *line)
			}
		}
	}
	sort.Slice(out.Idle, func(i, j int) bool { return out.Idle[i].LastRun < out.Idle[j].LastRun })
	return out, nil
}

func registerHiddenLines(admin fiber.Router, cityOf func(fiber.Ctx) *cities.City, session fiber.Handler) {
	list := func(c fiber.Ctx, city *cities.City) error {
		answer, err := hiddenLines(city, time.Now())
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		c.Set("Cache-Control", "no-store")
		return c.JSON(answer)
	}
	admin.Get("/hidden-lines", session, func(c fiber.Ctx) error { return list(c, cityOf(c)) })
	admin.Put("/hidden-lines", session, func(c fiber.Ctx) error {
		var req struct {
			Names  []string `json:"names"`
			Hidden *bool    `json:"hidden"`
		}
		if err := c.Bind().Body(&req); err != nil || len(req.Names) == 0 || req.Hidden == nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
		}
		city := cityOf(c)
		if err := city.HideLines(req.Names, *req.Hidden); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return list(c, city)
	})
}
