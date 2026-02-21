package handlers

import (
	"conexiuni-cluj/database"
	"time"

	"github.com/gofiber/fiber/v3"
)

func HandleCachedData[T any](
	c fiber.Ctx,
	cacheID string,
	cacheShelfLife time.Duration,
	dbFetcher func() (T, error),
	apiFetcher func() (T, error),
	dbStorer func(T) error,
	optimize bool,
) error {
	isCacheValid := database.IsCacheValid(cacheID)
	if isCacheValid {
		data, err := dbFetcher()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": err.Error(),
			})
		}
		return c.JSON(data)
	}

	data, err := apiFetcher()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	go func() {
		_ = dbStorer(data)
		_ = database.UpdateCache(cacheID, cacheShelfLife.Milliseconds())
		if optimize {
			_ = database.Optimize()
		}
	}()
	return c.JSON(data)
}
