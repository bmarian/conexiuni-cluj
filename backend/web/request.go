package web

import (
	"net/url"
	"strings"

	"github.com/gofiber/fiber/v3"
)

// Always revalidate, so fixes reach clients on the next request.
const Revalidate = "no-cache"

// SameOrigin is false for a request another site's page made.
func SameOrigin(c fiber.Ctx) bool {
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

// Canonical even when a page is served from the old host.
const SiteOrigin = "https://ceautobuziau.ro"
