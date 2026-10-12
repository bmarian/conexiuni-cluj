package handlers

import (
	"conexiuni-cluj/database"
	"conexiuni-cluj/models"
	ctpcj "conexiuni-cluj/services/ctp-cj"
	"conexiuni-cluj/services/tranzy"
	"net/url"
	"strings"

	"github.com/gofiber/fiber/v3"
)

// "no-cache" forces ETag revalidation on every request so backend fixes
// reach clients on the next round-trip. Unchanged bodies short-circuit to
// 304 via the etag middleware on the /api group.
const revalidateCacheControl = "no-cache"

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
		c.Set("Cache-Control", revalidateCacheControl)
		if c.Query("format") == "polyline" {
			return c.JSON(shapePolylines(data))
		}
		return c.JSON(data)
	})

	api.Get("/live", ServeLive)

	api.Get("/plan", func(c fiber.Ctx) error {
		return handlePlan(c, tranzyClient, ctpCjClient, cacheTimes)
	})

	api.Get("/network", NetworkHandler(tranzyClient, ctpCjClient, cacheTimes))
	api.Get("/network/version", NetworkVersionHandler(tranzyClient, ctpCjClient, cacheTimes))

	api.Get("/news", func(c fiber.Ctx) error { return GetNews(c, cacheTimes.NewsCacheShelfLife) })

	api.Get("/route-changes", GetRouteChanges)

	api.Get("/push/key", GetPushKey)
	api.Post("/push/subscribe", SubscribePush)
	api.Post("/push/unsubscribe", UnsubscribePush)
	api.Post("/push/reminders", SyncReminders)

	api.Post("/transfer", CreateTransferLink)
	api.Put("/transfer/:code", SendToTransferLink)
	api.Get("/transfer/:code", ReadTransferLink)
	api.Delete("/transfer/:code", DeleteTransferLink)

}
