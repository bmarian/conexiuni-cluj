package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"conexiuni-cluj/database"

	"github.com/gofiber/fiber/v3"
)

const clientHashLocalKey = "client_hash"

// ComputeClientHash returns an 8-byte hex HMAC-SHA256 of the request's client (its IP, or the /64 for IPv6)
// using the provided salt. Same scheme as the access log identifier.
func ComputeClientHash(c fiber.Ctx, salt string) string {
	ip := ClientIP(c)
	if ip == "" {
		return ""
	}
	h := hmac.New(sha256.New, []byte(salt))
	_, _ = h.Write([]byte(clientKey(ip)))
	return hex.EncodeToString(h.Sum(nil)[:8])
}

// ClientHashFromLocals returns the previously-computed client hash stored by
// StatsMiddleware. Empty if the middleware did not run for this request.
func ClientHashFromLocals(c fiber.Ctx) string {
	if v, ok := c.Locals(clientHashLocalKey).(string); ok {
		return v
	}
	return ""
}

var appUserAgents = map[string]string{
	"ConexiuniClujMobile/": "android",
	"ConexiuniClujWear/":   "wear",
}

var botUserAgentWords = []string{"bot", "crawl", "spider", "slurp", "preview", "lighthouse", "headless"}

// clientKind is "web", "android/1.4.0", "wear/1.0.2" or "other"; empty for crawlers.
func clientKind(ua string) string {
	for prefix, platform := range appUserAgents {
		if version, ok := strings.CutPrefix(ua, prefix); ok {
			if validAppVersion(version) {
				return platform + "/" + version
			}
			return platform
		}
	}
	lower := strings.ToLower(ua)
	for _, w := range botUserAgentWords {
		if strings.Contains(lower, w) {
			return ""
		}
	}
	if strings.HasPrefix(ua, "Mozilla/") {
		return "web"
	}
	return "other"
}

func validAppVersion(v string) bool {
	if v == "" || len(v) > 16 {
		return false
	}
	for _, r := range v {
		if (r < '0' || r > '9') && r != '.' {
			return false
		}
	}
	return true
}

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

// StatsMiddleware computes the client hash once, then counts every API call by its route,
// the people behind them and the server errors. Must run before the access logger.
func StatsMiddleware(salt string) fiber.Handler {
	return func(c fiber.Ctx) error {
		start := time.Now()
		hash := ComputeClientHash(c, salt)
		c.Locals(clientHashLocalKey, hash)

		err := c.Next()
		route := c.Route().Path
		if !strings.HasPrefix(route, "/api/") || strings.HasPrefix(route, "/api/admin") {
			return err
		}
		key := c.Method() + " " + route
		status := responseStatus(c, err)
		if status >= fiber.StatusInternalServerError {
			database.RecordEvent("endpoint_error", key)
		}
		if status >= fiber.StatusBadRequest {
			return err
		}
		database.RecordVisit(hash, clientKind(c.Get(fiber.HeaderUserAgent)))
		if route == "/api/live" && c.Query("once") == "" {
			database.RecordEvent("endpoint_call", key+" stream")
		} else {
			database.RecordEndpointTiming(key, time.Since(start))
		}
		return err
	}
}
