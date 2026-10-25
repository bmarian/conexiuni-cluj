package cities

import (
	"conexiuni-cluj/database"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
)

func TestWarmUpAnswersAreNotServerErrors(t *testing.T) {
	city := newNamedTestCity(t, "a", nil)
	app := fiber.New()
	app.Use(StatsMiddleware(map[string]*City{"a": city}, "salt"))
	app.Get("/api/a/warm", func(c fiber.Ctx) error {
		c.Set(fiber.HeaderRetryAfter, "10")
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "still loading"})
	})
	app.Get("/api/a/down", func(c fiber.Ctx) error {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "database gone"})
	})
	for _, path := range []string{"/api/a/warm", "/api/a/down"} {
		if _, err := app.Test(httptest.NewRequest("GET", path, nil)); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := city.MetricRows("error_detail", database.StatsDate(time.Now()))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Key != "GET /down|503|database gone" || rows[0].Count != 1 {
		t.Fatalf("error details %+v", rows)
	}
}
