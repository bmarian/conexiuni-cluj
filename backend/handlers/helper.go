package handlers

import (
	"conexiuni-cluj/database"
	"conexiuni-cluj/services/tranzy"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"golang.org/x/sync/singleflight"
)

var cacheSingleflight singleflight.Group

type CacheOpts[T any] struct {
	Optimize    bool
	PostProcess func(T) T
}

func HandleCached[T any](
	cacheID string,
	shelfLife time.Duration,
	dbFetcher func() (T, error),
	apiFetcher func() (T, error),
	dbStorer func(T) error,
	opts CacheOpts[T],
) (T, error) {
	if data, hit, err := readFromCache(cacheID, dbFetcher); hit {
		if err != nil {
			var zero T
			return zero, err
		}
		return data, nil
	}

	// Deduplicate concurrent cache-miss callers so only one API request goes out;
	// everyone else waits for the shared result.
	raw, err, _ := cacheSingleflight.Do(cacheID, func() (any, error) {
		// Re-check the cache — an earlier flight may have just populated it.
		if data, hit, err := readFromCache(cacheID, dbFetcher); hit {
			if err != nil {
				return nil, err
			}
			return data, nil
		}

		data, err := apiFetcher()
		if err != nil {
			return nil, err
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

		return data, nil
	})
	if err != nil {
		var zero T
		return zero, err
	}
	data := raw.(T)

	if opts.PostProcess != nil {
		return opts.PostProcess(data), nil
	}
	return data, nil
}

func NormalizeTripID(tripID string) string {
	parts := strings.Split(tripID, "_")
	if len(parts) >= 2 {
		return parts[0] + "_" + parts[1]
	}
	return tripID
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

// tranzyFetch fetches endpoint from the Tranzy API and unmarshals the response into T.
func tranzyFetch[T any](client *tranzy.Client, endpoint string) (T, error) {
	data, err := client.DoRequest(endpoint, nil)
	if err != nil {
		var zero T
		return zero, err
	}
	var result T
	if err := json.Unmarshal(data, &result); err != nil {
		var zero T
		return zero, fmt.Errorf("failed to unmarshal %s response: %w", endpoint, err)
	}
	return result, nil
}
