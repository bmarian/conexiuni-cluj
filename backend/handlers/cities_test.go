package handlers

import (
	"bytes"
	"conexiuni-cluj/database"
	"conexiuni-cluj/sources"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
)

func testCities(t *testing.T, cities ...*City) *Cities {
	t.Helper()
	cs, err := NewCities(cities, filepath.Join(t.TempDir(), "disabled-cities.json"))
	if err != nil {
		t.Fatalf("cities: %v", err)
	}
	return cs
}

func newNamedTestCity(t *testing.T, slug string, static sources.Static) *City {
	t.Helper()
	dir := t.TempDir()
	store, err := database.Open(filepath.Join(dir, "test.db"), filepath.Join(dir, "archive.db"), 5)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	city := NewCity(slug, slug, store, static, nil, nil, nil, Options{})
	t.Cleanup(city.Close)
	return city
}

type countingStatic struct{ calls atomic.Int32 }

func (s *countingStatic) Snapshot() (*sources.Snapshot, error) {
	s.calls.Add(1)
	return nil, errors.New("offline")
}
func (s *countingStatic) Cadence() time.Duration { return time.Hour }

func TestDisabledCityIsGoneAndStaysGone(t *testing.T) {
	a, b := newNamedTestCity(t, "a", nil), newNamedTestCity(t, "b", nil)
	path := filepath.Join(t.TempDir(), "disabled-cities.json")
	cities, err := NewCities([]*City{a, b}, path)
	if err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	missing := func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusNotFound) }
	for _, city := range cities.All() {
		city.Register(app.Group("/api/"+city.Slug, city.OnlyEnabled(missing)))
	}
	status := func(url string) int {
		resp, err := app.Test(httptest.NewRequest("GET", url, nil))
		if err != nil {
			t.Fatal(err)
		}
		return resp.StatusCode
	}
	if got := status("/api/b/route-changes"); got != http.StatusOK {
		t.Fatalf("enabled city answered %d", got)
	}

	if err := cities.SetEnabled("b", false); err != nil {
		t.Fatal(err)
	}
	if got := status("/api/b/route-changes"); got != http.StatusNotFound {
		t.Fatalf("disabled city answered %d, want 404", got)
	}
	if cities.Find("b") != nil || len(cities.Enabled()) != 1 {
		t.Fatalf("disabled city still listed")
	}
	if err := cities.SetEnabled("a", false); !errors.Is(err, ErrLastCity) {
		t.Fatalf("turning off the last city: %v", err)
	}

	again, err := NewCities([]*City{newNamedTestCity(t, "a", nil), newNamedTestCity(t, "b", nil)}, path)
	if err != nil {
		t.Fatal(err)
	}
	if again.Find("b") != nil || again.Find("a") == nil {
		t.Fatalf("restart forgot the switch")
	}
}

func TestStoppedCityFetchesNothingUntilEnabled(t *testing.T) {
	static := &countingStatic{}
	city := newNamedTestCity(t, "a", static)
	cities := testCities(t, city, newNamedTestCity(t, "b", nil))
	waitFor := func(what string, ok func() bool) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for !ok() {
			if time.Now().After(deadline) {
				t.Fatalf("timed out waiting for %s", what)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}

	city.Start()
	waitFor("the first fetch", func() bool { return static.calls.Load() == 1 })

	if err := cities.SetEnabled("a", false); err != nil {
		t.Fatal(err)
	}
	waitFor("the stop", func() bool { return city.running().stopped() })
	stopped := city.running()
	done := make(chan struct{})
	go func() { stopped.tasks.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("background work still running after the stop")
	}

	if err := cities.SetEnabled("a", true); err != nil {
		t.Fatal(err)
	}
	waitFor("the fetch after enabling", func() bool { return static.calls.Load() == 2 })
	city.run.enabled.Store(false)
	city.Stop()
}

func TestAdminSwitchesACity(t *testing.T) {
	cities := testCities(t, newNamedTestCity(t, "a", nil), newNamedTestCity(t, "b", nil))
	app := fiber.New()
	RegisterAdminRoutes(app.Group("/api"), cities, "secret", false)
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
	data, err := os.ReadFile(cities.path)
	if err != nil || string(data) != `["b"]` {
		t.Fatalf("saved %q, %v", data, err)
	}
}
