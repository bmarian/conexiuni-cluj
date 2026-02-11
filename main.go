package main

import (
	"conexiuni-cluj/config"
	"conexiuni-cluj/frontend"
	"log"

	"github.com/a-h/templ"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/adaptor"
)

func main() {
	conf := config.Load()
	app := fiber.New()

	component := frontend.Hello("Fuck Life")
	app.Get("/", adaptor.HTTPHandler(templ.Handler(component)))
	log.Fatal(app.Listen(":" + conf.Port))
}
