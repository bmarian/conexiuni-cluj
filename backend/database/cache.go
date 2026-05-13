package database

import (
	"sync"
	"time"
)

var cacheMutexes sync.Map

func GetCacheRWMutex(cacheId string) *sync.RWMutex {
	v, _ := cacheMutexes.LoadOrStore(cacheId, &sync.RWMutex{})
	return v.(*sync.RWMutex)
}

func IsCacheValid(cacheId string) bool {
	return GetCacheState(cacheId) == CacheStateFresh
}

// CacheState distinguishes "never populated" from "populated but past TTL",
// enabling stale-while-revalidate: stale rows can be served immediately while
// a background refresh runs.
type CacheState int

const (
	CacheStateMissing CacheState = iota
	CacheStateFresh
	CacheStateStale
)

func GetCacheState(cacheId string) CacheState {
	var timestamp, lifespan int64
	err := DB.QueryRow(`SELECT timestamp, lifespan FROM cache_times WHERE id = ?`, cacheId).Scan(&timestamp, &lifespan)
	if err != nil {
		return CacheStateMissing
	}
	if time.Now().UnixMilli() < (timestamp + lifespan) {
		return CacheStateFresh
	}
	return CacheStateStale
}

func UpdateCache(cacheId string, lifespan int64) error {
	timestamp := time.Now().UnixMilli()

	_, err := DB.Exec(`
		INSERT OR REPLACE INTO cache_times (id, timestamp, lifespan)
		VALUES (?, ?, ?)
	`, cacheId, timestamp, lifespan)

	return err
}

func InvalidateCache(cacheId string) error {
	_, err := DB.Exec(`DELETE FROM cache_times WHERE id = ?`, cacheId)
	return err
}

func InvalidateAllCaches() error {
	_, err := DB.Exec(`DELETE FROM cache_times`)
	return err
}

type CacheEntry struct {
	ID        string
	Timestamp int64
	Lifespan  int64
}

func ListCacheEntries() ([]CacheEntry, error) {
	rows, err := DB.Query(`SELECT id, timestamp, lifespan FROM cache_times ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []CacheEntry
	for rows.Next() {
		var e CacheEntry
		if err := rows.Scan(&e.ID, &e.Timestamp, &e.Lifespan); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
