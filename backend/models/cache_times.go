package models

import "time"

type CacheTimes struct {
	ShapeCacheShelfLife       time.Duration
	RouteCacheShelfLife       time.Duration
	TripCacheShelfLife        time.Duration
	StopCacheShelfLife        time.Duration
	StopTimeCacheShelfLife    time.Duration
	APIStopTimeCacheShelfLife time.Duration
	StopInfoCacheShelfLife    time.Duration
	TimetableCacheShelfLife   time.Duration
	DirectionsCacheShelfLife  time.Duration
}
