package handlers

import (
	"conexiuni-cluj/models"
	"conexiuni-cluj/services/tranzy"
	"fmt"
)

const (
	StopInfoCacheId = "STOP_INFO"
)

func GetStopInfo(tranzyClient *tranzy.Client, cacheTimes models.CacheTimes, filter StopFilter) (*models.StopInfo, error) {
	opts := CacheOpts[*models.StopInfo]{
		Optimize: true,
	}

	cacheID := fmt.Sprintf("%s_%d", StopInfoCacheId, *filter.StopID)
	return HandleCached(cacheID, cacheTimes.StopInfoCacheShelfLife,
		func() (*models.StopInfo, error) { return getStopInfoFromDB(filter) },
		func() (*models.StopInfo, error) { return requestStopInfo(tranzyClient, filter) },
		storeStopInfoInDB,
		opts)
}

func getStopInfoFromDB(filter StopFilter) (*models.StopInfo, error) {
	return nil, nil
}

func requestStopInfo(tranzyClient *tranzy.Client, filter StopFilter) (*models.StopInfo, error) {
	return nil, nil
}

func storeStopInfoInDB(stopInfo *models.StopInfo) error {
	return nil
}
