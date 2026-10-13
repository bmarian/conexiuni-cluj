package handlers

import (
	"strconv"
	"strings"

	"conexiuni-cluj/router"

	"github.com/gofiber/fiber/v3"
)

// How many places a search lists at most: a short list to pick from while typing.
const placesLimit = 10

// handlePlaces serves GET /api/places: `q` searches the area's addresses, named places and streets;
// `lat` and `lon` name the place at a position.
func handlePlaces(c fiber.Ctx) error {
	places := plannerPlaces()
	if places == nil {
		c.Set("Retry-After", "10")
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "places still loading", "retry_after_seconds": 10})
	}
	c.Set("Cache-Control", revalidateCacheControl)
	if q := strings.TrimSpace(c.Query("q")); q != "" {
		limit, err := strconv.Atoi(c.Query("limit"))
		if err != nil || limit < 1 || limit > placesLimit {
			limit = placesLimit
		}
		found := places.Search(q, limit)
		if found == nil {
			found = []router.Place{}
		}
		return c.JSON(found)
	}
	lat, errLat := strconv.ParseFloat(c.Query("lat"), 64)
	lon, errLon := strconv.ParseFloat(c.Query("lon"), 64)
	if errLat != nil || errLon != nil || !isValidLat(lat) || !isValidLng(lon) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "q, or lat and lon, are required"})
	}
	if place, ok := places.Nearby(lat, lon); ok {
		return c.JSON([]router.Place{place})
	}
	return c.JSON([]router.Place{})
}
