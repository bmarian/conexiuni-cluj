package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"conexiuni-cluj/database"

	"github.com/gofiber/fiber/v3"
)

const clientHashLocalKey = "client_hash"

var streamedAPIPaths = map[string]struct{}{
	"/api/vehicles/stream": {},
}

// ComputeClientHash returns an 8-byte hex HMAC-SHA256 of the request's client IP
// using the provided salt. Same scheme as the access log identifier.
func ComputeClientHash(c fiber.Ctx, salt string) string {
	ip := ""
	for forwardedIP := range strings.SplitSeq(c.Get(fiber.HeaderXForwardedFor), ",") {
		candidate := strings.TrimSpace(forwardedIP)
		if candidate != "" {
			ip = candidate
			break
		}
	}
	if ip == "" {
		ip = c.IP()
	}
	if ip == "" {
		return ""
	}
	h := hmac.New(sha256.New, []byte(salt))
	_, _ = h.Write([]byte(ip))
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

func skipStatsPath(path string) bool {
	if strings.HasPrefix(path, "/api/admin") || path == "/admin" || strings.HasPrefix(path, "/admin/") {
		return true
	}
	return false
}

// classifyURL maps a request to a stats metric/key pair. Empty metric means
// the request is not tracked.
func classifyURL(method, path string) (metric, key string) {
	if method != "GET" || skipStatsPath(path) {
		return "", ""
	}
	if key := database.CanonicalAPIStatsKey(path); key != "" {
		return "api_call", key
	}
	return "", ""
}

func isSuccessfulStatus(status int) bool {
	return status >= fiber.StatusOK && status < fiber.StatusBadRequest
}

// StatsMiddleware computes the client hash once and records a stats event for
// the request. Must be registered before the access logger so the logger can
// reuse the cached hash via ClientHashFromLocals.
func StatsMiddleware(salt string) fiber.Handler {
	return func(c fiber.Ctx) error {
		start := time.Now()
		hash := ComputeClientHash(c, salt)
		c.Locals(clientHashLocalKey, hash)

		err := c.Next()
		if err == nil && isSuccessfulStatus(c.Response().StatusCode()) {
			if skipStatsPath(c.Path()) {
				return err
			}
			metric, key := classifyURL(c.Method(), c.Path())
			if metric != "" {
				if _, streamed := streamedAPIPaths[key]; streamed {
					database.RecordEvent(hash, metric, key)
				} else {
					database.RecordEndpointTiming(hash, key, time.Since(start))
				}
			}
		}
		return err
	}
}
