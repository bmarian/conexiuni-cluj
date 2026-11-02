package admin

import (
	"bytes"
	"conexiuni-cluj/cities"
	"conexiuni-cluj/database"
	"conexiuni-cluj/devicesync"
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/live"
	"conexiuni-cluj/models"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
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

func adminApp(t *testing.T, path string, all ...*cities.City) *fiber.App {
	t.Helper()
	registry, err := cities.NewRegistry(all, path)
	if err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	Register(app.Group("/api"), registry, func() (devicesync.Stats, error) { return devicesync.Stats{}, nil }, "secret", false)
	return app
}

func putAs(t *testing.T, app *fiber.App, url, body string, signedIn bool) int {
	t.Helper()
	req := httptest.NewRequest("PUT", url, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	if signedIn {
		req.AddCookie(&http.Cookie{Name: adminSessionCookie, Value: newAdminSession("secret", time.Now())})
	}
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode
}

func TestAdminSwitchesACity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "disabled-cities.json")
	app := adminApp(t, path, testCity(t, "a"), testCity(t, "b"))
	put := func(slug, body string) int { return putAs(t, app, "/api/admin/cities/"+slug, body, true) }
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

func TestAdminHidesAndBringsBackStops(t *testing.T) {
	city := testCity(t, "a")
	app := adminApp(t, filepath.Join(t.TempDir(), "disabled-cities.json"), city)
	put := func(body string, signedIn bool) int { return putAs(t, app, "/api/admin/hidden-stops?city=a", body, signedIn) }
	if got := put(`{"stop_ids":[7],"hidden":true}`, false); got != http.StatusUnauthorized {
		t.Fatalf("signed out: %d, want 401", got)
	}
	if got := put(`{"stop_ids":[],"hidden":true}`, true); got != http.StatusBadRequest {
		t.Fatalf("no stops: %d, want 400", got)
	}
	if got := put(`{"stop_ids":[7]}`, true); got != http.StatusBadRequest {
		t.Fatalf("no value: %d, want 400", got)
	}
	if got := put(`{"stop_ids":[7],"hidden":true}`, true); got != http.StatusOK || !city.StopHidden(7) {
		t.Fatalf("hide: %d, hidden %v", got, city.StopHidden(7))
	}
	if got := put(`{"stop_ids":[7],"hidden":false}`, true); got != http.StatusOK || city.StopHidden(7) {
		t.Fatalf("bring back: %d, hidden %v", got, city.StopHidden(7))
	}
}

func TestAdminHidesLinesAndListsTheIdleOnes(t *testing.T) {
	city := testCity(t, "a")
	snap := &feeds.Snapshot{Stops: []models.Stop{{StopID: 1, StopName: "A", StopLat: 46, StopLon: 24}, {StopID: 2, StopName: "B", StopLat: 46, StopLon: 24.01}}}
	for id := 1; id <= 3; id++ {
		trip := fmt.Sprintf("%d_0", id)
		snap.Routes = append(snap.Routes, models.Route{RouteID: id, RouteShortName: fmt.Sprint(id)})
		snap.Trips = append(snap.Trips, models.Trip{TripID: trip, RouteID: id})
		snap.TripStops = append(snap.TripStops, models.TripStop{TripID: trip, StopID: 1, StopSequence: 1}, models.TripStop{TripID: trip, StopID: 2, StopSequence: 2})
	}
	if err := city.StoreNetwork(snap); err != nil {
		t.Fatal(err)
	}
	if err := city.Availability().Refresh(city.Data); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	ran := func(route int, at time.Time) {
		t.Helper()
		if _, err := city.Archive.Exec(`INSERT INTO segment_travel_time_samples VALUES (?, 0, 1, 2, 'weekday', 480, 60, ?)`, route, at.Unix()); err != nil {
			t.Fatal(err)
		}
	}
	names := func(lines []adminLine) (out []string) {
		for _, l := range lines {
			out = append(out, l.Name)
		}
		return out
	}

	ran(2, now.Add(-10*24*time.Hour))
	if got, err := hiddenLines(city, now); err != nil || len(got.Idle) != 0 {
		t.Fatalf("no bus this week, idle %v (%v)", names(got.Idle), err)
	}
	ran(1, now.Add(-time.Hour))
	got, err := hiddenLines(city, now)
	if err != nil || !slices.Equal(names(got.Idle), []string{"3", "2"}) || got.Idle[1].LastRun != now.Add(-10*24*time.Hour).Unix() {
		t.Fatalf("idle %+v (%v)", got.Idle, err)
	}

	app := adminApp(t, filepath.Join(t.TempDir(), "disabled-cities.json"), city)
	put := func(body string, signedIn bool) int { return putAs(t, app, "/api/admin/hidden-lines?city=a", body, signedIn) }
	if got := put(`{"names":["3"],"hidden":true}`, false); got != http.StatusUnauthorized {
		t.Fatalf("signed out: %d, want 401", got)
	}
	if got := put(`{"names":[],"hidden":true}`, true); got != http.StatusBadRequest {
		t.Fatalf("no lines: %d, want 400", got)
	}
	if got := put(`{"names":["3"],"hidden":true}`, true); got != http.StatusOK || !city.LineHidden("3") {
		t.Fatalf("hide: %d, hidden %v", got, city.LineHidden("3"))
	}
	if got, _ := hiddenLines(city, now); !slices.Equal(names(got.Hidden), []string{"3"}) || !slices.Equal(names(got.Idle), []string{"2"}) {
		t.Fatalf("hidden %v, idle %v", names(got.Hidden), names(got.Idle))
	}
	if got := put(`{"names":["3"],"hidden":false}`, true); got != http.StatusOK || city.LineHidden("3") {
		t.Fatalf("bring back: %d, hidden %v", got, city.LineHidden("3"))
	}
}
