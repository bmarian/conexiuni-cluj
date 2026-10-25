package pages

import (
	"conexiuni-cluj/cities"

	"github.com/gofiber/fiber/v3"
)

// Register serves a city's pages: home, line, stop, planner and destinations.
func Register(app fiber.Router, city *cities.City) {
	page := func(serve func(*cities.City, fiber.Ctx) error) fiber.Handler {
		return func(c fiber.Ctx) error { return serve(city, c) }
	}
	app.Get("/", page(homePage))
	app.Get("/linia/:line", page(linePage))
	app.Get("/linia/:line/:direction", page(linePage))
	app.Get("/statia/:station", page(stationPage))
	app.Get("/route/:routeId/:direction", page(idRoutePage))
	app.Get("/stop/:stopId", page(idStopPage))
	app.Get("/plan", page(planPage))
	app.Get(destinationsPath, page(destinationsPage))
	app.Get(areaPath(":slug"), page(destinationPage))
	app.Get("/destinations", page(func(city *cities.City, c fiber.Ctx) error { return moved(c, city, destinationsPath) }))
	app.Get("/to/:slug", page(func(city *cities.City, c fiber.Ctx) error { return moved(c, city, areaPath(c.Params("slug"))) }))
}
