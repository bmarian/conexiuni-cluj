package cities

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
)

func TestShapesAsksForNoMoreThanTheBusiestStationShows(t *testing.T) {
	city := newNamedTestCity(t, "t", &swapStatic{snap: oneLineSnapshot(24.01)})
	if err := city.Ingest.Run(time.Now()); err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	city.Register(app.Group("/api/t"))
	cases := map[string]int{
		"/api/t/shapes":                   fiber.StatusBadRequest,
		"/api/t/shapes?shape_ids=,":       fiber.StatusBadRequest,
		"/api/t/shapes?shape_ids=1_0":     fiber.StatusOK,
		"/api/t/shapes?shape_ids=1_0,1_0": fiber.StatusOK,
		"/api/t/shapes?shape_ids=1_0,1_1": fiber.StatusBadRequest,
	}
	for url, want := range cases {
		resp, err := app.Test(httptest.NewRequest("GET", url, nil))
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != want {
			t.Errorf("%s: status %d, want %d", url, resp.StatusCode, want)
		}
	}
}
