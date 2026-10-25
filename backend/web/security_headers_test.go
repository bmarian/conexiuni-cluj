package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestCORSAllowsOnlyOurSites(t *testing.T) {
	app := fiber.New()
	app.Use(CORS)
	app.Get("/", func(c fiber.Ctx) error { return c.SendString("ok") })
	for origin, want := range map[string]string{
		"https://ceautobuziau.ro":    "https://ceautobuziau.ro",
		"https://bus.bmarian.online": "https://bus.bmarian.online",
		"https://example.com":        "",
		"":                           "",
	} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		if origin != "" {
			req.Header.Set(fiber.HeaderOrigin, origin)
		}
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		if got := resp.Header.Get(fiber.HeaderAccessControlAllowOrigin); got != want {
			t.Errorf("origin %q: allowed %q, want %q", origin, got, want)
		}
		if resp.StatusCode != fiber.StatusOK {
			t.Errorf("origin %q: status %d", origin, resp.StatusCode)
		}
	}
}

func TestSecurityHeaders(t *testing.T) {
	app := fiber.New()
	app.Use(SecurityHeaders)
	app.Get("/", func(c fiber.Ctx) error { return c.SendString("ok") })
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range []string{"Strict-Transport-Security", "X-Content-Type-Options", "X-Frame-Options", "Content-Security-Policy"} {
		if resp.Header.Get(h) == "" {
			t.Errorf("missing %s", h)
		}
	}
}
