package admin

import (
	"conexiuni-cluj/cities"
	"conexiuni-cluj/models"
	"conexiuni-cluj/transit"
	"sort"

	"github.com/gofiber/fiber/v3"
)

type hiddenStation struct {
	StopIDs []int        `json:"stop_ids"`
	Name    string       `json:"name"`
	Lat     float64      `json:"lat"`
	Lon     float64      `json:"lon"`
	Lines   []namedCount `json:"lines"`
}

func hiddenStations(city *cities.City) ([]*hiddenStation, error) {
	stops, err := city.Stops(transit.StopFilter{})
	if err != nil {
		return nil, err
	}
	byID := make(map[int]models.Stop, len(stops))
	for _, s := range stops {
		byID[s.StopID] = s
	}
	ids := city.HiddenStops()
	routes, err := city.RoutesAtStops(ids)
	if err != nil {
		return nil, err
	}
	var allRoutes []int
	for _, at := range routes {
		allRoutes = append(allRoutes, at...)
	}
	lines, err := routeNames(city, allRoutes)
	if err != nil {
		return nil, err
	}

	stations := map[int]*hiddenStation{}
	out := []*hiddenStation{}
	for _, id := range ids {
		stop, ok := byID[id]
		if !ok {
			continue
		}
		station := stations[stop.Station()]
		if station == nil {
			station = &hiddenStation{Name: stop.StopName, Lat: stop.StopLat, Lon: stop.StopLon, Lines: []namedCount{}}
			stations[stop.Station()] = station
			out = append(out, station)
		}
		station.StopIDs = append(station.StopIDs, id)
		for _, route := range routes[id] {
			if line := lines[route]; city.Availability().RouteHasTimetable(line.Name) {
				station.Lines = append(station.Lines, line)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func registerHiddenStops(admin fiber.Router, cityOf func(fiber.Ctx) *cities.City, session fiber.Handler) {
	list := func(c fiber.Ctx, city *cities.City) error {
		stations, err := hiddenStations(city)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		c.Set("Cache-Control", "no-store")
		return c.JSON(stations)
	}
	admin.Get("/hidden-stops", session, func(c fiber.Ctx) error { return list(c, cityOf(c)) })
	admin.Put("/hidden-stops", session, func(c fiber.Ctx) error {
		var req struct {
			StopIDs []int `json:"stop_ids"`
			Hidden  *bool `json:"hidden"`
		}
		if err := c.Bind().Body(&req); err != nil || len(req.StopIDs) == 0 || req.Hidden == nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
		}
		city := cityOf(c)
		if err := city.HideStops(req.StopIDs, *req.Hidden); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return list(c, city)
	})
}
