package handlers

import (
	"bufio"
	"conexiuni-cluj/models"
	ctpcj "conexiuni-cluj/services/ctp-cj"
	"conexiuni-cluj/services/ors"
	"conexiuni-cluj/services/tranzy"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
)

const staticCacheControl = "max-age=3600, stale-while-revalidate=86400"

func RegisterAPIRoutes(api fiber.Router, tranzyClient *tranzy.Client, ctpCjClient *ctpcj.Client, orsClient *ors.Client, cacheTimes models.CacheTimes) {
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
		data, err := GetRoutes(tranzyClient, cacheTimes.RouteCacheShelfLife, filter)
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
		c.Set("Cache-Control", staticCacheControl)
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
		data, err := GetStops(tranzyClient, cacheTimes.StopCacheShelfLife, filter)
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
		c.Set("Cache-Control", staticCacheControl)
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
		data, err := GetShapes(tranzyClient, cacheTimes.ShapeCacheShelfLife, filter)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		c.Set("Cache-Control", staticCacheControl)
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
		data, err := GetTrips(tranzyClient, cacheTimes.TripCacheShelfLife, filter)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
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
		c.Set("Cache-Control", staticCacheControl)
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
		c.Set("Cache-Control", staticCacheControl)
		return c.JSON(data)
	})

	api.Get("/timetable", func(c fiber.Ctx) error {
		routeShortName := c.Query("route_short_name")
		data, err := GetTimetable(ctpCjClient, cacheTimes.TimetableCacheShelfLife, routeShortName)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		c.Set("Cache-Control", staticCacheControl)
		return c.JSON(data)
	})

	api.Get("/directions", func(c fiber.Ctx) error {
		if orsClient == nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "directions service not configured (ORS_API_KEY missing)"})
		}

		fromLatS := c.Query("from_lat")
		fromLngS := c.Query("from_lng")
		toLatS := c.Query("to_lat")
		toLngS := c.Query("to_lng")
		if fromLatS == "" || fromLngS == "" || toLatS == "" || toLngS == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "from_lat, from_lng, to_lat, to_lng are required"})
		}

		fromLat, err := strconv.ParseFloat(fromLatS, 64)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid from_lat"})
		}
		fromLng, err := strconv.ParseFloat(fromLngS, 64)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid from_lng"})
		}
		toLat, err := strconv.ParseFloat(toLatS, 64)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid to_lat"})
		}
		toLng, err := strconv.ParseFloat(toLngS, 64)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid to_lng"})
		}

		data, err := GetDirections(orsClient, fromLat, fromLng, toLat, toLng)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		if len(data) == 0 {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "directions unavailable"})
		}
		c.Set("Cache-Control", "no-store")
		return c.Send(data)
	})
}
