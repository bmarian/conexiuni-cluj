package pages

import (
	"conexiuni-cluj/cities"

	"github.com/gofiber/fiber/v3"
)

// Register serves a city's pages: home, line, stop and planner.
func Register(app fiber.Router, city *cities.City) {
	page := func(serve func(*cities.City, fiber.Ctx) error) fiber.Handler {
		return func(c fiber.Ctx) error { return serve(city, c) }
	}
	app.Get("/", page(homePage))
	app.Get("/route/:routeId/:direction", page(routePage))
	app.Get("/stop/:stopId", page(stopPage))
	app.Get("/plan", page(planPage))
}
