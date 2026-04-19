package handlers

import "time"

type VehicleIntervalConfig struct {
	Baseline             time.Duration
	Busy                 time.Duration
	Reserve              time.Duration
	SubscribersThreshold int
	RushMorningStart     int
	RushMorningEnd       int
	RushEveningStart     int
	RushEveningEnd       int
}

func (c VehicleIntervalConfig) isRushHour(now time.Time) bool {
	h := now.Hour()
	return (h >= c.RushMorningStart && h < c.RushMorningEnd) ||
		(h >= c.RushEveningStart && h < c.RushEveningEnd)
}

func (c VehicleIntervalConfig) rushSecondsRemaining(localNow, midnight time.Time) int {
	windows := [2][2]int{
		{c.RushMorningStart, c.RushMorningEnd},
		{c.RushEveningStart, c.RushEveningEnd},
	}
	y, m, d := localNow.Date()
	loc := localNow.Location()
	total := 0
	for _, w := range windows {
		start := time.Date(y, m, d, w[0], 0, 0, 0, loc)
		end := time.Date(y, m, d, w[1], 0, 0, 0, loc)
		if !end.After(localNow) {
			continue
		}
		if start.Before(localNow) {
			start = localNow
		}
		if end.After(midnight) {
			end = midnight
		}
		total += int(end.Sub(start).Seconds())
	}
	return total
}

// Keeps a 1-per-Reserve budget reserved until midnight so we never fully drain the daily quota.
func ComputeVehicleInterval(cfg VehicleIntervalConfig, subscribers, quotaRemaining int, now time.Time, loc *time.Location) time.Duration {
	localNow := now.In(loc)
	busyNow := cfg.isRushHour(localNow) || subscribers >= cfg.SubscribersThreshold

	y, m, d := localNow.Date()
	midnight := time.Date(y, m, d+1, 0, 0, 0, 0, loc)
	secondsLeft := int(midnight.Sub(localNow).Seconds())
	if secondsLeft <= 0 {
		return cfg.Reserve
	}

	reserveSeconds := int(cfg.Reserve.Seconds())
	if reserveSeconds < 1 {
		reserveSeconds = 1
	}
	reserveRequests := secondsLeft / reserveSeconds
	if reserveRequests < 1 {
		reserveRequests = 1
	}

	spendable := quotaRemaining - reserveRequests
	if spendable <= 0 {
		return cfg.Reserve
	}

	busyS := int(cfg.Busy.Seconds())
	baseS := int(cfg.Baseline.Seconds())
	if busyS < 1 {
		busyS = 1
	}
	if baseS < 1 {
		baseS = 1
	}

	rushSeconds := cfg.rushSecondsRemaining(localNow, midnight)
	baselineSeconds := secondsLeft - rushSeconds
	if baselineSeconds < 0 {
		baselineSeconds = 0
	}
	wantedBudget := rushSeconds/busyS + baselineSeconds/baseS
	if wantedBudget < 1 {
		wantedBudget = 1
	}

	var interval time.Duration
	switch {
	case wantedBudget <= spendable:
		if busyNow {
			interval = cfg.Busy
		} else {
			interval = cfg.Baseline
		}
	case busyNow:
		interval = time.Duration(int64(cfg.Busy) * int64(wantedBudget) / int64(spendable))
	default:
		interval = time.Duration(int64(cfg.Baseline) * int64(wantedBudget) / int64(spendable))
	}

	if interval > cfg.Reserve {
		interval = cfg.Reserve
	}
	if interval < cfg.Busy {
		interval = cfg.Busy
	}
	return interval
}
