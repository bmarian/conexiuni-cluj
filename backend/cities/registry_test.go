package cities

import (
	"conexiuni-cluj/database"
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/live"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
)

func testCities(t *testing.T, all ...*City) *Registry {
	t.Helper()
	cs, err := NewRegistry(all, filepath.Join(t.TempDir(), "disabled-cities.json"))
	if err != nil {
		t.Fatalf("cities: %v", err)
	}
	return cs
}

func newNamedTestCity(t *testing.T, slug string, static feeds.Static) *City {
	t.Helper()
	dir := t.TempDir()
	store, err := database.Open(filepath.Join(dir, "test.db"), filepath.Join(dir, "archive.db"), 5)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	eng := New(slug, slug, store, static, nil, nil, nil, live.Options{})
	t.Cleanup(eng.Close)
	return eng
}

type countingStatic struct{ calls atomic.Int32 }

func (s *countingStatic) Snapshot() (*feeds.Snapshot, error) {
	s.calls.Add(1)
	return nil, errors.New("offline")
}
func (s *countingStatic) Cadence() time.Duration { return time.Hour }

func TestDisabledCityIsGoneAndStaysGone(t *testing.T) {
	a, b := newNamedTestCity(t, "a", nil), newNamedTestCity(t, "b", nil)
	path := filepath.Join(t.TempDir(), "disabled-cities.json")
	cities, err := NewRegistry([]*City{a, b}, path)
	if err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	missing := func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusNotFound) }
	for _, eng := range cities.All() {
		eng.Register(app.Group("/api/"+eng.Slug, eng.OnlyEnabled(missing)))
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

	again, err := NewRegistry([]*City{newNamedTestCity(t, "a", nil), newNamedTestCity(t, "b", nil)}, path)
	if err != nil {
		t.Fatal(err)
	}
	if again.Find("b") != nil || again.Find("a") == nil {
		t.Fatalf("restart forgot the switch")
	}
}

func TestStoppedCityFetchesNothingUntilEnabled(t *testing.T) {
	static := &countingStatic{}
	eng := newNamedTestCity(t, "a", static)
	cities := testCities(t, eng, newNamedTestCity(t, "b", nil))
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

	eng.Start()
	waitFor("the first fetch", func() bool { return static.calls.Load() == 1 })

	if err := cities.SetEnabled("a", false); err != nil {
		t.Fatal(err)
	}
	waitFor("the stop", func() bool { return eng.running().Stopped() })
	stopped := eng.running()
	done := make(chan struct{})
	go func() { stopped.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("background work still running after the stop")
	}

	if err := cities.SetEnabled("a", true); err != nil {
		t.Fatal(err)
	}
	waitFor("the fetch after enabling", func() bool { return static.calls.Load() == 2 })
	eng.run.enabled.Store(false)
	eng.Stop()
}
