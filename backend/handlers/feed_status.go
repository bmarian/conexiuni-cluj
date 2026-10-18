package handlers

import (
	"conexiuni-cluj/models"
	"slices"
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

func (city *City) noteFeed(reported int, vehicles []models.Vehicle) {
	now := time.Now()
	ages := make([]time.Duration, 0, len(vehicles))
	for _, v := range vehicles {
		if at, err := time.Parse(time.RFC3339, v.Timestamp); err == nil {
			ages = append(ages, now.Sub(at))
		}
	}
	slices.Sort(ages)
	city.feed.Lock()
	city.feed.status.FetchedAt, city.feed.status.Reported, city.feed.status.Valid = now, reported, len(vehicles)
	city.feed.status.FixAge = 0
	if len(ages) > 0 {
		city.feed.status.FixAge = ages[len(ages)/2]
	}
	city.feed.Unlock()
}

func (city *City) noteFeedError(err error) {
	city.feed.Lock()
	city.feed.status.Err, city.feed.status.ErrAt = err.Error(), time.Now()
	city.feed.Unlock()
}

func (city *City) currentFeedStatus() feedStatus {
	city.feed.Lock()
	defer city.feed.Unlock()
	return city.feed.status
}
