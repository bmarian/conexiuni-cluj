package handlers

import "github.com/gofiber/fiber/v3"

// Every origin the web app talks to: map tiles, place search, weather, the Android release check.
const contentSecurityPolicy = "default-src 'self'; " +
	"script-src 'self'; " +
	"style-src 'self' 'unsafe-inline'; " +
	"img-src 'self' data: blob: https://*.basemaps.cartocdn.com; " +
	"connect-src 'self' https://*.basemaps.cartocdn.com https://photon.komoot.io https://api.open-meteo.com https://api.github.com; " +
	"font-src 'self'; " +
	"object-src 'none'; " +
	"base-uri 'self'; " +
	"form-action 'self'; " +
	"frame-ancestors 'none'"

// Browsers ignore HSTS over plain HTTP, so it can be sent from behind nginx.
func SecurityHeaders(c fiber.Ctx) error {
	c.Set(fiber.HeaderStrictTransportSecurity, "max-age=31536000")
	c.Set(fiber.HeaderXContentTypeOptions, "nosniff")
	c.Set(fiber.HeaderXFrameOptions, "DENY")
	c.Set(fiber.HeaderContentSecurityPolicy, contentSecurityPolicy)
	return c.Next()
}
