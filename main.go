package main

import (
	"conexiuni-cluj/config"
	"conexiuni-cluj/frontend"
	"log"

	"github.com/a-h/templ"
	"github.com/gofiber/fiber/v3"
)

func main() {
	conf := config.Load()
	app := fiber.New()

	app.Get("/:name?", func(c fiber.Ctx) error {
		name := c.Params("name")
		return Render(c, frontend.Index(name))
	})

	app.Use(NotFoundMiddleware)
	log.Fatal(app.Listen(":" + conf.Port))
}

func NotFoundMiddleware(c fiber.Ctx) error {
	c.Status(fiber.StatusNotFound)
	return Render(c, frontend.NotFound())
}

func Render(c fiber.Ctx, component templ.Component) error {
	c.Set("Content-Type", "text/html")
	return component.Render(c.Context(), c.Response().BodyWriter())
}
