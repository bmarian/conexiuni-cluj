package pages

import (
	"conexiuni-cluj/cities"
	"conexiuni-cluj/database"
	"conexiuni-cluj/live"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func testCity(t *testing.T) (*cities.City, string) {
	t.Helper()
	dir := t.TempDir()
	store, err := database.Open(filepath.Join(dir, "test.db"), filepath.Join(dir, "archive.db"), 5)
	if err != nil {
		t.Fatal(err)
	}
	city := cities.New("a", "A", store, nil, nil, nil, nil, live.Options{})
	t.Cleanup(city.Close)
	return city, dir
}

func TestSitemapWaitsForDestinations(t *testing.T) {
	city, dir := testCity(t)
	registry, err := cities.NewRegistry([]*cities.City{city}, filepath.Join(dir, "disabled-cities.json"))
	if err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	app.Get("/sitemap.xml", Sitemap(registry))
	res, err := app.Test(httptest.NewRequest("GET", "/sitemap.xml", nil))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != 200 || res.Header.Get("Cache-Control") != "no-cache" {
		t.Errorf("a city without destinations got %d, Cache-Control %q", res.StatusCode, res.Header.Get("Cache-Control"))
	}
}
