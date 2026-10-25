package cities

import (
	"conexiuni-cluj/web"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
)

func responseStatus(c fiber.Ctx, err error) int {
	if err == nil {
		return c.Response().StatusCode()
	}
	var fe *fiber.Error
	if errors.As(err, &fe) {
		return fe.Code
	}
	return fiber.StatusInternalServerError
}

func errorText(c fiber.Ctx, err error) string {
	if err != nil {
		return err.Error()
	}
	body, err := c.Response().BodyUncompressed()
	if err != nil {
		return ""
	}
	var reply struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(body, &reply) == nil && reply.Error != "" {
		return reply.Error
	}
	return string(body)
}

// Must run before the access logger.
func StatsMiddleware(cities map[string]*City, salt string) fiber.Handler {
	return func(c fiber.Ctx) error {
		start := time.Now()
		hash, kind := web.Identify(c, salt)

		err := c.Next()
		route := c.Route().Path
		slug, rest, _ := strings.Cut(strings.TrimPrefix(route, "/api/"), "/")
		city := cities[slug]
		if !strings.HasPrefix(route, "/api/") || city == nil || !city.Enabled() || strings.HasPrefix(rest, "admin") {
			return err
		}
		route = "/" + rest
		key := c.Method() + " " + route
		status := responseStatus(c, err)
		warmingUp := status == fiber.StatusServiceUnavailable && len(c.Response().Header.Peek(fiber.HeaderRetryAfter)) > 0
		if status >= fiber.StatusInternalServerError && !warmingUp {
			city.RecordRequestError(key+"|"+strconv.Itoa(status)+"|"+errorText(c, err), c.Path(), time.Now())
		}
		if status >= fiber.StatusBadRequest {
			return err
		}
		if route == "/live" && c.Query("once") == "" {
			key += " stream"
			city.RecordEvent("endpoint_call", key)
		} else {
			city.RecordEndpointTiming(key, time.Since(start))
		}
		if kind != "" {
			city.RecordVisit(hash, kind)
			city.RecordEvent("person_call", key)
		}
		return err
	}
}
