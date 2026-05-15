package handlers

import (
	"conexiuni-cluj/services/tranzy"
	"log"
	"math"
	"time"
)

const (
	vehicleLearningNoBudgetCheckInterval = time.Hour
)

type VehicleLearningSamplerConfig struct {
	Enabled       bool
	MaxDailyQuota int
}

type vehicleLearningPlan struct {
	Interval       time.Duration
	ShelfLife      time.Duration
	DailyBudget    int
	CallsRemaining int
	QuotaRemaining int
}

func StartVehicleLearningSampler(tranzyClient *tranzy.Client, cfg VehicleLearningSamplerConfig) {
	if !cfg.Enabled {
		log.Printf("vehicle learner: disabled")
		return
	}
	if cfg.MaxDailyQuota <= 0 {
		log.Printf("vehicle learner: disabled because max_daily_quota=%d", cfg.MaxDailyQuota)
		return
	}
	log.Printf("vehicle learner: enabled max_daily_quota=%d",
		cfg.MaxDailyQuota,
	)
	go func() {
		for {
			plan := currentVehicleLearningPlan(tranzyClient, cfg)
			delay := plan.Interval
			if delay <= 0 {
				delay = vehicleLearningNoBudgetCheckInterval
			}
			time.Sleep(delay)
			runVehicleLearningSample(tranzyClient, cfg)
		}
	}()
}

func runVehicleLearningSample(tranzyClient *tranzy.Client, cfg VehicleLearningSamplerConfig) {
	subscribers := 0
	if VehicleHub != nil {
		subscribers = VehicleHub.SubscriberCount()
	}
	if subscribers > 0 {
		return
	}

	plan := currentVehicleLearningPlan(tranzyClient, cfg)
	if plan.CallsRemaining <= 0 {
		log.Printf("vehicle learner: skip, calls_remaining=%d daily_budget=%d quota_remaining=%d",
			plan.CallsRemaining,
			plan.DailyBudget,
			plan.QuotaRemaining,
		)
		return
	}

	_, err := GetVehicles(tranzyClient, plan.ShelfLife, VehicleFilter{})
	if err != nil {
		log.Printf("vehicle learner: sample failed interval=%s daily_budget=%d calls_remaining=%d quota_remaining=%d err=%v",
			plan.Interval,
			plan.DailyBudget,
			plan.CallsRemaining,
			plan.QuotaRemaining,
			err,
		)
		return
	}
}

func currentVehicleLearningPlan(tranzyClient *tranzy.Client, cfg VehicleLearningSamplerConfig) vehicleLearningPlan {
	return computeVehicleLearningPlan(
		tranzyClient.VehiclesQuotaRemaining(),
		cfg.MaxDailyQuota,
		time.Now(),
		tranzyClient.Location(),
	)
}

func computeVehicleLearningPlan(
	quotaRemaining int,
	maxDailyQuota int,
	now time.Time,
	loc *time.Location,
) vehicleLearningPlan {
	dailyBudget := maxDailyQuota
	callsByClock, untilReset := vehicleLearningCallsRemainingByClock(dailyBudget, now, loc)
	callsRemaining := callsByClock
	if quotaRemaining < 0 {
		quotaRemaining = 0
	}
	if callsRemaining > quotaRemaining {
		callsRemaining = quotaRemaining
	}

	interval := time.Duration(0)
	if callsRemaining > 0 && untilReset > 0 {
		interval = untilReset / time.Duration(callsRemaining)
	}
	shelfLife := interval
	if shelfLife > time.Second {
		shelfLife -= time.Second
	}

	return vehicleLearningPlan{
		Interval:       interval,
		ShelfLife:      shelfLife,
		DailyBudget:    dailyBudget,
		CallsRemaining: callsRemaining,
		QuotaRemaining: quotaRemaining,
	}
}

func vehicleLearningCallsRemainingByClock(dailyBudget int, now time.Time, loc *time.Location) (int, time.Duration) {
	if dailyBudget <= 0 {
		return 0, 0
	}
	if loc == nil {
		loc = time.Local
	}

	localNow := now.In(loc)
	y, m, d := localNow.Date()
	dayStart := time.Date(y, m, d, 0, 0, 0, 0, loc)
	nextReset := dayStart.AddDate(0, 0, 1)
	dayDuration := nextReset.Sub(dayStart)
	untilReset := nextReset.Sub(localNow)
	if dayDuration <= 0 || untilReset <= 0 {
		return 0, 0
	}

	remainingRatio := untilReset.Seconds() / dayDuration.Seconds()
	callsRemaining := int(math.Ceil(float64(dailyBudget) * remainingRatio))
	if callsRemaining < 1 {
		callsRemaining = 1
	}
	return callsRemaining, untilReset
}
