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
	data, state, err := readFromCache(cacheID, dbFetcher)
	if err != nil {
		var zero T
		return zero, err
	}
	switch state {
	case database.CacheStateFresh:
		return applyPostProcess(data, opts), nil
	case database.CacheStateStale:
		// Stale-while-revalidate: serve the existing row immediately and refresh
		// from the API in the background. Avoids the 8h-idle cascade where every
		// cache miss serialized behind Tranzy + CTP-CJ rate limiters.
		go backgroundRefresh(cacheID, shelfLife, apiFetcher, dbStorer, opts)
		return applyPostProcess(data, opts), nil
	}

	// CacheStateMissing: deduplicate concurrent cache-miss callers via
	// singleflight so only one API request goes out; everyone else waits.
	raw, err, _ := cacheSingleflight.Do(cacheID, func() (any, error) {
		if data, state, err := readFromCache(cacheID, dbFetcher); err == nil && state != database.CacheStateMissing {
			return data, nil
		}

		fetched, err := apiFetcher()
		if err != nil {
			return nil, err
		}

		go writeCache(cacheID, shelfLife, fetched, dbStorer, opts)
		return fetched, nil
	})
	if err != nil {
		var zero T
		return zero, err
	}
	return applyPostProcess(raw.(T), opts), nil
}

func applyPostProcess[T any](data T, opts CacheOpts[T]) T {
	if opts.PostProcess != nil {
		return opts.PostProcess(data)
	}
	return data
}

func backgroundRefresh[T any](
	cacheID string,
	shelfLife time.Duration,
	apiFetcher func() (T, error),
	dbStorer func(T) error,
	opts CacheOpts[T],
) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Warning: panic in background refresh for %s: %v", cacheID, r)
		}
	}()

	_, _, _ = cacheSingleflight.Do(cacheID, func() (any, error) {
		data, err := apiFetcher()
		if err != nil {
			log.Printf("Warning: background refresh of %s failed: %v", cacheID, err)
			return nil, err
		}
		writeCache(cacheID, shelfLife, data, dbStorer, opts)
		return data, nil
	})
}

func writeCache[T any](
	cacheID string,
	shelfLife time.Duration,
	data T,
	dbStorer func(T) error,
	opts CacheOpts[T],
) {
	mu := database.GetCacheRWMutex(cacheID)
	mu.Lock()
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Warning: panic in cache write for %s: %v", cacheID, r)
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
}

func NormalizeTripID(tripID string) string {
	parts := strings.Split(tripID, "_")
	if len(parts) >= 2 {
		return parts[0] + "_" + parts[1]
	}
	return tripID
}

func readFromCache[T any](cacheID string, dbFetcher func() (T, error)) (T, database.CacheState, error) {
	mu := database.GetCacheRWMutex(cacheID)
	mu.RLock()
	defer mu.RUnlock()

	state := database.GetCacheState(cacheID)
	if state == database.CacheStateMissing {
		var zero T
		return zero, state, nil
	}

	data, err := dbFetcher()
	return data, state, err
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
