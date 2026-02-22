package handlers

import (
	"conexiuni-cluj/database"
	"log"
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
	if data, hit, err := readFromCache(cacheID, dbFetcher); hit {
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
		mu := database.GetCacheRWMutex(cacheID)
		mu.Lock()
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Warning: panic in cache write goroutine for %s: %v", cacheID, r)
			}
			mu.Unlock()
		}()

		// If 2 goroutines are writing to the same cache,
		// the first one to finish will update the cache
		if database.IsCacheValid(cacheID) {
			return
		}

		if err := dbStorer(data); err != nil {
			log.Printf("Warning: failed to store %s in database: %v", cacheID, err)
			return
		}
		if err := database.UpdateCache(cacheID, shelfLife.Milliseconds()); err != nil {
			log.Printf("Warning: failed to update cache for %s: %v", cacheID, err)
			return
		}
		if opts.Optimize {
			if err := database.Optimize(); err != nil {
				log.Printf("Warning: failed to optimize database: %v", err)
			}
		}
	}()

	if opts.PostProcess != nil {
		return c.JSON(opts.PostProcess(data))
	}
	return c.JSON(data)
}

func readFromCache[T any](cacheID string, dbFetcher func() (T, error)) (T, bool, error) {
	mu := database.GetCacheRWMutex(cacheID)
	mu.RLock()
	defer mu.RUnlock()

	if !database.IsCacheValid(cacheID) {
		var zero T
		return zero, false, nil
	}

	data, err := dbFetcher()
	return data, true, err
}
