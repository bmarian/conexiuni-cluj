package pages

import (
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/models"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestAHiddenStopSendsItsPageToTheNearestStop(t *testing.T) {
	city, _ := testCity(t)
	err := city.StoreNetwork(&feeds.Snapshot{
		Routes:    []models.Route{{RouteID: 1, RouteShortName: "1"}},
		Stops:     []models.Stop{{StopID: 1, StopName: "Far", StopLat: 46, StopLon: 24}, {StopID: 2, StopName: "Hidden", StopLat: 46, StopLon: 24.1}, {StopID: 3, StopName: "Near", StopLat: 46, StopLon: 24.11}},
		Trips:     []models.Trip{{TripID: "1_0", RouteID: 1}},
		TripStops: []models.TripStop{{TripID: "1_0", StopID: 1, StopSequence: 1}, {TripID: "1_0", StopID: 2, StopSequence: 2}, {TripID: "1_0", StopID: 3, StopSequence: 3}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := city.HideStops([]int{2}, true); err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	app.Get("/stop", func(c fiber.Ctx) error { return stopPage(city, c, 2) })
	res, err := app.Test(httptest.NewRequest("GET", "/stop", nil))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != fiber.StatusFound || res.Header.Get("Location") != "/a/stop/3" {
		t.Fatalf("got %d to %q", res.StatusCode, res.Header.Get("Location"))
	}
}
