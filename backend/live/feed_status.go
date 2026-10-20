package live

import (
	"conexiuni-cluj/models"
	"slices"
	"sync"
	"time"
)

type FeedStatus struct {
	FetchedAt time.Time
	Reported  int
	Valid     int
	// Median age of the GPS fixes when they were fetched.
	FixAge time.Duration
	Err    string
	ErrAt  time.Time
}

func (eng *Engine) noteFeed(reported int, vehicles []models.Vehicle) {
	now := time.Now()
	ages := make([]time.Duration, 0, len(vehicles))
	for _, v := range vehicles {
		if at, err := time.Parse(time.RFC3339, v.Timestamp); err == nil {
			ages = append(ages, now.Sub(at))
		}
	}
	slices.Sort(ages)
	eng.feed.Lock()
	eng.feed.status.FetchedAt, eng.feed.status.Reported, eng.feed.status.Valid = now, reported, len(vehicles)
	eng.feed.status.FixAge = 0
	if len(ages) > 0 {
		eng.feed.status.FixAge = ages[len(ages)/2]
	}
	eng.feed.Unlock()
}

func (eng *Engine) noteFeedError(err error) {
	eng.feed.Lock()
	eng.feed.status.Err, eng.feed.status.ErrAt = err.Error(), time.Now()
	eng.feed.Unlock()
}

func (eng *Engine) currentFeedStatus() FeedStatus {
	eng.feed.Lock()
	defer eng.feed.Unlock()
	return eng.feed.status
}

type feedState struct {
	sync.Mutex
	status FeedStatus
}
