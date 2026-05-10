package models

import "time"

type CacheTimes struct {
	TranzyCacheShelfLife    time.Duration
	TimetableCacheShelfLife time.Duration
	StopTimeCacheShelfLife  time.Duration
	NewsCacheShelfLife      time.Duration
}
