package admin

import (
	"bytes"
	"conexiuni-cluj/cities"
	"conexiuni-cluj/database"
	"conexiuni-cluj/devicesync"
	"conexiuni-cluj/live"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
)

func testCity(t *testing.T, slug string) *cities.City {
	t.Helper()
	dir := t.TempDir()
	store, err := database.Open(filepath.Join(dir, "test.db"), filepath.Join(dir, "archive.db"), 5)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	eng := cities.New(slug, slug, store, nil, nil, nil, nil, live.Options{})
	t.Cleanup(eng.Close)
	return eng
}

func TestAdminSwitchesACity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "disabled-cities.json")
	cities, err := cities.NewRegistry([]*cities.City{testCity(t, "a"), testCity(t, "b")}, path)
	if err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	Register(app.Group("/api"), cities, func() (devicesync.Stats, error) { return devicesync.Stats{}, nil }, "secret", false)
	put := func(slug, body string) int {
		req := httptest.NewRequest("PUT", "/api/admin/cities/"+slug, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{Name: adminSessionCookie, Value: newAdminSession("secret", time.Now())})
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		return resp.StatusCode
	}
	if got := put("b", `{"enabled":false}`); got != http.StatusOK {
		t.Fatalf("switch off: %d", got)
	}
	if got := put("a", `{"enabled":false}`); got != http.StatusConflict {
		t.Fatalf("last city off: %d, want 409", got)
	}
	if got := put("nowhere", `{"enabled":true}`); got != http.StatusNotFound {
		t.Fatalf("unknown city: %d, want 404", got)
	}
	if got := put("b", `{}`); got != http.StatusBadRequest {
		t.Fatalf("no value: %d, want 400", got)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != `["b"]` {
		t.Fatalf("saved %q, %v", data, err)
	}
}

func TestSiteListsEnabledCitiesByName(t *testing.T) {
	registry, err := cities.NewRegistry([]*cities.City{testCity(t, "b"), testCity(t, "a"), testCity(t, "c")}, filepath.Join(t.TempDir(), "disabled-cities.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.SetEnabled("c", false); err != nil {
		t.Fatal(err)
	}
	site := buildSite(registry, func() (devicesync.Stats, error) { return devicesync.Stats{}, errors.New("no sync") }, time.Now())
	if len(site.Cities) != 2 || site.Cities[0].Slug != "a" || site.Cities[1].Slug != "b" {
		t.Fatalf("cities %+v", site.Cities)
	}
	if site.Server.Sync != nil {
		t.Errorf("sync stats from a failing source: %+v", site.Server.Sync)
	}
}
