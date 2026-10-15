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

// Stale rows are served while a background refresh runs.
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
