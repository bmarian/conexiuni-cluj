package pages

import (
	"conexiuni-cluj/cities"
	"conexiuni-cluj/slugs"
	"conexiuni-cluj/transit"
	"fmt"
	"strconv"

	"github.com/gofiber/fiber/v3"
)

const destinationsPath = "/destinatii"

func areaPath(slug string) string { return "/spre/" + slug }

func linePath(city *cities.City, routeID, dir int) string {
	slug := city.Slugs.Index().Of(slugs.Line, routeID)
	switch {
	case slug == "":
		return fmt.Sprintf("/route/%d/%d", routeID, dir)
	case dir == 1:
		return "/linia/" + slug + "/intors"
	}
	return "/linia/" + slug
}

func lineCanonicalPath(city *cities.City, routeID, dir int) string {
	if city.Slugs.Index().Of(slugs.Line, routeID) != "" {
		dir = 0
	}
	return linePath(city, routeID, dir)
}

func stationPath(city *cities.City, stationID int) string {
	if slug := city.Slugs.Index().Of(slugs.Station, stationID); slug != "" {
		return "/statia/" + slug
	}
	return "/stop/" + strconv.Itoa(stationID)
}

func moved(c fiber.Ctx, city *cities.City, path string) error {
	to := "/" + city.Slug + path
	if q := string(c.Request().URI().QueryString()); q != "" {
		to += "?" + q
	}
	return c.Redirect().Status(fiber.StatusMovedPermanently).To(to)
}

// The app still loads, to suggest similar names.
func missing(c fiber.Ctx) error {
	if indexHTMLContent == "" {
		return c.Next()
	}
	c.Set("Content-Type", "text/html; charset=utf-8")
	return c.Status(fiber.StatusNotFound).SendString(indexHTMLContent)
}

func linePage(city *cities.City, c fiber.Ctx) error {
	dir, auto := 0, false
	switch c.Params("direction") {
	case "":
	case "intors":
		dir = 1
	case "auto":
		auto = true
	default:
		return missing(c)
	}
	slug := c.Params("line")
	addresses := city.Slugs.Index()
	routeID, ok := addresses.Find(slugs.Line, slug)
	if !ok {
		return missing(c)
	}
	if addresses.Of(slugs.Line, routeID) != slug {
		return moved(c, city, autoPath(linePath(city, routeID, dir), auto))
	}
	return routePage(city, c, routeID, dir)
}

func autoPath(path string, auto bool) string {
	if auto {
		return path + "/auto"
	}
	return path
}

func stationPage(city *cities.City, c fiber.Ctx) error {
	slug := c.Params("station")
	addresses := city.Slugs.Index()
	stationID, ok := addresses.Find(slugs.Station, slug)
	if !ok {
		return missing(c)
	}
	if addresses.Of(slugs.Station, stationID) != slug {
		return moved(c, city, stationPath(city, stationID))
	}
	return stopPage(city, c, stationID)
}

func idRoutePage(city *cities.City, c fiber.Ctx) error {
	routeID, err := strconv.Atoi(c.Params("routeId"))
	if err != nil {
		return missing(c)
	}
	param := c.Params("direction")
	dir, err := strconv.Atoi(param)
	if city.Slugs.Index().Of(slugs.Line, routeID) != "" && (err == nil || param == "auto") {
		return moved(c, city, autoPath(linePath(city, routeID, dir), param == "auto"))
	}
	if param == "auto" {
		return c.Next()
	}
	if err != nil {
		return missing(c)
	}
	return routePage(city, c, routeID, dir)
}

func idStopPage(city *cities.City, c fiber.Ctx) error {
	stopID, err := strconv.Atoi(c.Params("stopId"))
	if err != nil {
		return missing(c)
	}
	stops, err := city.Stops(transit.StopFilter{StopID: &stopID})
	if err != nil {
		return c.Next()
	}
	if len(stops) == 0 {
		return missing(c)
	}
	if station := stops[0].StationID; city.Slugs.Index().Of(slugs.Station, station) != "" {
		return moved(c, city, stationPath(city, station))
	}
	return stopPage(city, c, stopID)
}
