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
	var timestamp int64
	var lifespan int64

	err := DB.QueryRow(`SELECT timestamp, lifespan FROM cache_times WHERE id = ?`, cacheId).Scan(&timestamp, &lifespan)
	if err != nil {
		return false
	}

	return time.Now().UnixMilli() < (timestamp + lifespan)
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
