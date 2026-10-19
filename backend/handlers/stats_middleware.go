package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
)

const (
	clientHashLocalKey = "client_hash"
	clientKindLocalKey = "client_kind"
)

// Same scheme as the access log id; IPv6 clients by /64.
func ComputeClientHash(c fiber.Ctx, salt string) string {
	ip := ClientIP(c)
	if ip == "" {
		return ""
	}
	h := hmac.New(sha256.New, []byte(salt))
	_, _ = h.Write([]byte(clientKey(ip)))
	return hex.EncodeToString(h.Sum(nil)[:8])
}

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

// clientKind is "web", "android/1.4.0" or "wear/1.0.2"; empty for robots and scripts.
func clientKind(ua, fetchSite string) string {
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
	// Browsers mark the site's own fetches; scripts that borrow a browser user agent don't.
	if strings.HasPrefix(ua, "Mozilla/") && fetchSite == "same-origin" {
		return "web"
	}
	return ""
}

// IsPerson tells the website and the apps apart from robots and scripts.
func IsPerson(c fiber.Ctx) bool {
	kind, _ := c.Locals(clientKindLocalKey).(string)
	return kind != ""
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

// Must run before the access logger.
func StatsMiddleware(cities map[string]*City, salt string) fiber.Handler {
	return func(c fiber.Ctx) error {
		start := time.Now()
		hash := ComputeClientHash(c, salt)
		kind := clientKind(c.Get(fiber.HeaderUserAgent), c.Get("Sec-Fetch-Site"))
		c.Locals(clientHashLocalKey, hash)
		c.Locals(clientKindLocalKey, kind)

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
		if status >= fiber.StatusInternalServerError {
			city.RecordEvent("endpoint_error", key)
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
