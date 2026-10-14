package handlers

import (
	"conexiuni-cluj/models"
	"slices"
	"sync"
	"time"
)

type feedStatus struct {
	FetchedAt time.Time
	Reported  int
	Valid     int
	// Median age of the GPS fixes when they were fetched.
	FixAge time.Duration
	Err    string
	ErrAt  time.Time
}

var feed = struct {
	sync.Mutex
	status feedStatus
}{}

func noteFeed(reported int, vehicles []models.Vehicle) {
	now := time.Now()
	ages := make([]time.Duration, 0, len(vehicles))
	for _, v := range vehicles {
		if at, err := time.Parse(time.RFC3339, v.Timestamp); err == nil {
			ages = append(ages, now.Sub(at))
		}
	}
	slices.Sort(ages)
	feed.Lock()
	feed.status.FetchedAt, feed.status.Reported, feed.status.Valid = now, reported, len(vehicles)
	feed.status.FixAge = 0
	if len(ages) > 0 {
		feed.status.FixAge = ages[len(ages)/2]
	}
	feed.Unlock()
}

func noteFeedError(err error) {
	feed.Lock()
	feed.status.Err, feed.status.ErrAt = err.Error(), time.Now()
	feed.Unlock()
}

func currentFeedStatus() feedStatus {
	feed.Lock()
	defer feed.Unlock()
	return feed.status
}
