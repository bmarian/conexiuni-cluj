package handlers

import (
	"conexiuni-cluj/database"
	"time"

	"github.com/gofiber/fiber/v3"
)

type CacheOpts[T any] struct {
	Optimize    bool
	PostProcess func(T) T
}

func HandleCached[T any](
	c fiber.Ctx,
	cacheID string,
	shelfLife time.Duration,
	dbFetcher func() (T, error),
	apiFetcher func() (T, error),
	dbStorer func(T) error,
	opts CacheOpts[T],
) error {
	if database.IsCacheValid(cacheID) {
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
		_ = database.UpdateCache(cacheID, shelfLife.Milliseconds())
		if opts.Optimize {
			_ = database.Optimize()
		}
	}()

	if opts.PostProcess != nil {
		return c.JSON(opts.PostProcess(data))
	}
	return c.JSON(data)
}
