package handlers

import (
	"conexiuni-cluj/database"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func newTestCity(t *testing.T) *City {
	t.Helper()
	dir := t.TempDir()
	store, err := database.Open(filepath.Join(dir, "test.db"), filepath.Join(dir, "archive.db"), 5)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	city := NewCity("test", "Test", store, nil, nil, nil, nil, Options{})
	city.resetProfileSnapshots()
	t.Cleanup(city.Close)
	return city
}

func (city *City) withoutArchive(fn func()) {
	archive := city.Archive
	city.Archive = nil
	defer func() { city.Archive = archive }()
	fn()
}

func TestLegacyPagesRedirectToCluj(t *testing.T) {
	app := fiber.New()
	RegisterLegacyPages(app, "cluj")
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
