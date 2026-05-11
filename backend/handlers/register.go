package handlers

import (
	"bufio"
	"conexiuni-cluj/database"
	"conexiuni-cluj/models"
	ctpcj "conexiuni-cluj/services/ctp-cj"
	"conexiuni-cluj/services/tranzy"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
)

// Cache-Control headers, set at registration time so they track the
// configured shelf lives (which derive from TRANZY_DEFAULT_DAILY_QUOTA).
// No stale-while-revalidate: the whole point of the hourly refresh is that
// when Tranzy ships bad data we recover within the shelf life, not 24h later.
var (
	tranzyCacheControl    string
	timetableCacheControl string
)

type statsEventRequest struct {
	Metric string `json:"metric"`
	Key    string `json:"key"`
}

func sameOriginRequest(c fiber.Ctx) bool {
	switch c.Get("Sec-Fetch-Site") {
	case "same-origin", "same-site", "none":
		return true
	}
	origin := c.Get(fiber.HeaderOrigin)
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	return err == nil && strings.EqualFold(u.Host, c.Hostname())
}

func validStatsEvent(metric, key string) bool {
	return metric == "pwa_install" && key == "appinstalled"
}

func RegisterAPIRoutes(api fiber.Router, tranzyClient *tranzy.Client, ctpCjClient *ctpcj.Client, cacheTimes models.CacheTimes) {
	tranzyCacheControl = fmt.Sprintf("max-age=%d", int(cacheTimes.TranzyCacheShelfLife.Seconds()))
	timetableCacheControl = fmt.Sprintf("max-age=%d", int(cacheTimes.TimetableCacheShelfLife.Seconds()))
	api.Post("/stats/event", func(c fiber.Ctx) error {
		if !sameOriginRequest(c) {
			return c.SendStatus(fiber.StatusNoContent)
		}
		var req statsEventRequest
		if err := c.Bind().Body(&req); err != nil {
			return c.SendStatus(fiber.StatusNoContent)
		}
		req.Metric = strings.TrimSpace(req.Metric)
		req.Key = strings.TrimSpace(req.Key)
		if !validStatsEvent(req.Metric, req.Key) {
			return c.SendStatus(fiber.StatusNoContent)
		}
		database.RecordEvent(ClientHashFromLocals(c), req.Metric, req.Key)
		c.Set("Cache-Control", "no-store")
		return c.SendStatus(fiber.StatusNoContent)
	})

	api.Get("/routes", func(c fiber.Ctx) error {
		filter := RouteFilter{}
		if s := c.Query("route_id"); s != "" {
			id, err := strconv.Atoi(s)
			if err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid route_id"})
			}
			filter.RouteID = &id
		}
		if s := c.Query("route_short_name"); s != "" {
			filter.RouteShortName = &s
		}
		data, err := GetRoutes(tranzyClient, cacheTimes.TranzyCacheShelfLife, filter)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		if filter.RouteID == nil && filter.RouteShortName == nil && Availability.IsReady() {
			filtered := make([]models.Route, 0, len(data))
			for _, r := range data {
				if Availability.RouteHasTimetable(r.RouteShortName) {
					filtered = append(filtered, r)
				}
			}
			data = filtered
		}
		c.Set("Cache-Control", tranzyCacheControl)
		return c.JSON(data)
	})

	api.Get("/stops", func(c fiber.Ctx) error {
		filter := StopFilter{}
		if s := c.Query("stop_id"); s != "" {
			id, err := strconv.Atoi(s)
			if err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid stop_id"})
			}
			filter.StopID = &id
		}
		data, err := GetStops(tranzyClient, cacheTimes.TranzyCacheShelfLife, filter)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		if filter.StopID == nil && Availability.IsReady() {
			filtered := make([]models.Stop, 0, len(data))
			for _, s := range data {
				if Availability.StopHasBuses(s.StopID) {
					filtered = append(filtered, s)
				}
			}
			data = filtered
		}
		c.Set("Cache-Control", tranzyCacheControl)
		return c.JSON(data)
	})

	api.Get("/shapes", func(c fiber.Ctx) error {
		filter := ShapeFilter{}
		if s := c.Query("shape_id"); s != "" {
			filter.ShapeID = &s
		}
		if s := c.Query("shape_ids"); s != "" {
			for _, id := range strings.Split(s, ",") {
				if id = strings.TrimSpace(id); id != "" {
					filter.ShapeIDs = append(filter.ShapeIDs, id)
				}
			}
		}
		data, err := GetShapes(tranzyClient, cacheTimes.TranzyCacheShelfLife, filter)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		c.Set("Cache-Control", tranzyCacheControl)
		return c.JSON(data)
	})

	api.Get("/vehicles", func(c fiber.Ctx) error {
		filter := VehicleFilter{}
		if s := c.Query("route_id"); s != "" {
			id, err := strconv.Atoi(s)
			if err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid route_id"})
			}
			filter.RouteID = &id
		}
		if s := c.Query("trip_id"); s != "" {
			filter.TripID = &s
		}
		if s := c.Query("trip_ids"); s != "" {
			for _, id := range strings.Split(s, ",") {
				if id = strings.TrimSpace(id); id != "" {
					filter.TripIDs = append(filter.TripIDs, id)
				}
			}
		}
		data, err := GetVehicles(tranzyClient, VehicleHub.CurrentInterval(), filter)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		c.Set("Cache-Control", "no-store")
		return c.JSON(data)
	})

	api.Get("/vehicles/stream", func(c fiber.Ctx) error {
		var tripIDs []string
		if s := c.Query("trip_ids"); s != "" {
			for _, id := range strings.Split(s, ",") {
				if id = strings.TrimSpace(id); id != "" {
					tripIDs = append(tripIDs, id)
				}
			}
		}
		if len(tripIDs) == 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "trip_ids required"})
		}
		c.Set("Content-Type", "text/event-stream")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")
		c.Set("X-Accel-Buffering", "no")
		sub, id := VehicleHub.Subscribe(tripIDs)
		return c.SendStreamWriter(func(w *bufio.Writer) {
			defer VehicleHub.Unsubscribe(id)
			keep := time.NewTicker(25 * time.Second)
			defer keep.Stop()
			for {
				select {
				case batch, ok := <-sub.Ch():
					if !ok {
						return
					}
					data, err := json.Marshal(batch)
					if err != nil {
						continue
					}
					if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
						return
					}
					if err := w.Flush(); err != nil {
						return
					}
				case <-keep.C:
					if _, err := w.WriteString(": ping\n\n"); err != nil {
						return
					}
					if err := w.Flush(); err != nil {
						return
					}
				}
			}
		})
	})

	api.Get("/trips", func(c fiber.Ctx) error {
		filter := TripFilter{}
		if s := c.Query("route_id"); s != "" {
			id, err := strconv.Atoi(s)
			if err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid route_id"})
			}
			filter.RouteID = &id
		}
		if s := c.Query("trip_id"); s != "" {
			filter.TripID = &s
		}
		data, err := GetTrips(tranzyClient, cacheTimes.TranzyCacheShelfLife, filter)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		c.Set("Cache-Control", tranzyCacheControl)
		return c.JSON(data)
	})

	api.Get("/stop_times", func(c fiber.Ctx) error {
		rsn := c.Query("route_short_name")
		if rsn == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "route_short_name is required"})
		}
		data, err := GetStopTimes(tranzyClient, cacheTimes, StopTimeFilter{RouteShortName: &rsn})
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		c.Set("Cache-Control", tranzyCacheControl)
		return c.JSON(data)
	})

	api.Get("/stop_info", func(c fiber.Ctx) error {
		s := c.Query("stop_id")
		if s == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "stop_id is required"})
		}
		id, err := strconv.Atoi(s)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid stop_id"})
		}
		data, err := GetStopInfo(tranzyClient, ctpCjClient, cacheTimes, StopFilter{StopID: &id})
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		c.Set("Cache-Control", tranzyCacheControl)
		return c.JSON(data)
	})

	api.Get("/timetable", func(c fiber.Ctx) error {
		routeShortName := c.Query("route_short_name")
		data, err := GetTimetable(ctpCjClient, cacheTimes.TimetableCacheShelfLife, routeShortName)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		c.Set("Cache-Control", timetableCacheControl)
		return c.JSON(data)
	})

	api.Get("/plan_routes", func(c fiber.Ctx) error {
		return handlePlanRoutes(c, tranzyClient, ctpCjClient, cacheTimes)
	})

	api.Get("/news", func(c fiber.Ctx) error { return GetNews(c, cacheTimes.NewsCacheShelfLife) })
}
