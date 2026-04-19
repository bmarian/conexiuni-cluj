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

func (c VehicleIntervalConfig) currentRushEnd(localNow time.Time) time.Time {
	y, m, d := localNow.Date()
	loc := localNow.Location()
	for _, w := range [2][2]int{
		{c.RushMorningStart, c.RushMorningEnd},
		{c.RushEveningStart, c.RushEveningEnd},
	} {
		start := time.Date(y, m, d, w[0], 0, 0, 0, loc)
		end := time.Date(y, m, d, w[1], 0, 0, 0, loc)
		if !localNow.Before(start) && localNow.Before(end) {
			return end
		}
	}
	return time.Time{}
}

func (c VehicleIntervalConfig) nextRushStart(localNow time.Time) time.Time {
	y, m, d := localNow.Date()
	loc := localNow.Location()
	var next time.Time
	for _, w := range [2][2]int{
		{c.RushMorningStart, c.RushMorningEnd},
		{c.RushEveningStart, c.RushEveningEnd},
	} {
		start := time.Date(y, m, d, w[0], 0, 0, 0, loc)
		if start.After(localNow) && (next.IsZero() || start.Before(next)) {
			next = start
		}
	}
	return next
}

// ComputeVehicleInterval Greedy: spend current window at desired rate; keep only a 1/Reserve floor for the rest of today.
func ComputeVehicleInterval(cfg VehicleIntervalConfig, subscribers, quotaRemaining int, now time.Time, loc *time.Location) time.Duration {
	localNow := now.In(loc)
	y, m, d := localNow.Date()
	midnight := time.Date(y, m, d+1, 0, 0, 0, 0, loc)
	secondsLeft := int(midnight.Sub(localNow).Seconds())
	if secondsLeft <= 0 {
		return cfg.Reserve
	}

	busyS := int(cfg.Busy.Seconds())
	baseS := int(cfg.Baseline.Seconds())
	reserveS := int(cfg.Reserve.Seconds())
	if busyS < 1 {
		busyS = 1
	}
	if baseS < 1 {
		baseS = 1
	}
	if reserveS < 1 {
		reserveS = 1
	}

	inRush := cfg.isRushHour(localNow)
	busyNow := inRush || subscribers >= cfg.SubscribersThreshold

	var windowEnd time.Time
	desiredS := baseS
	if busyNow {
		desiredS = busyS
		if inRush {
			windowEnd = cfg.currentRushEnd(localNow)
		} else {
			windowEnd = localNow.Add(cfg.Baseline)
		}
	} else {
		windowEnd = cfg.nextRushStart(localNow)
	}
	if windowEnd.IsZero() || windowEnd.After(midnight) {
		windowEnd = midnight
	}

	windowSeconds := int(windowEnd.Sub(localNow).Seconds())
	if windowSeconds < 1 {
		windowSeconds = 1
	}
	afterSeconds := secondsLeft - windowSeconds
	if afterSeconds < 0 {
		afterSeconds = 0
	}
	reserveAfter := afterSeconds / reserveS

	spendable := quotaRemaining - reserveAfter
	if spendable <= 0 {
		return cfg.Reserve
	}

	needed := windowSeconds / desiredS
	if needed < 1 {
		needed = 1
	}

	var interval time.Duration
	if needed <= spendable {
		interval = time.Duration(desiredS) * time.Second
	} else {
		interval = time.Duration(windowSeconds) * time.Second / time.Duration(spendable)
	}

	if interval > cfg.Reserve {
		interval = cfg.Reserve
	}
	if interval < cfg.Busy {
		interval = cfg.Busy
	}
	return interval
}
