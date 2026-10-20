package pages

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestLegacyPagesRedirectToCluj(t *testing.T) {
	app := fiber.New()
	RegisterLegacy(app, "cluj")
	for path, want := range map[string]string{
		"/stop/391":                       "/cluj/stop/391",
		"/route/12/0?change=4":            "/cluj/route/12/0?change=4",
		"/plan?lat=46.77&lon=23.6&name=x": "/cluj/plan?lat=46.77&lon=23.6&name=x",
		"/settings":                       "/cluj/settings",
		"/stop//evil.example":             "/cluj/stop//evil.example",
	} {
		resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, path, nil))
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if resp.StatusCode != fiber.StatusMovedPermanently || resp.Header.Get("Location") != want {
			t.Errorf("%s: got %d %q, want 301 %q", path, resp.StatusCode, resp.Header.Get("Location"), want)
		}
	}
}
