package handlers

import (
	"conexiuni-cluj/database"
	"conexiuni-cluj/services/tranzy"
	"log"
	"math"
	"time"
)

const (
	vehicleLearningAverageDays           = 14
	vehicleLearningNoBudgetCheckInterval = time.Hour
	vehicleLearningQuotaStatsKey         = "vehicles"
)

type VehicleLearningSamplerConfig struct {
	Enabled           bool
	MaxDailyQuota     int
	MinQuotaRemaining int
}

type vehicleLearningPlan struct {
	Interval                time.Duration
	ShelfLife               time.Duration
	DailyBudget             int
	AverageDailyUsage       float64
	AverageDays             int
	CallsRemaining          int
	QuotaRemaining          int
	SpendableQuotaRemaining int
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
	if cfg.MinQuotaRemaining < 0 {
		cfg.MinQuotaRemaining = 0
	}

	log.Printf("vehicle learner: enabled max_daily_quota=%d quota_reserve=%d average_days=%d",
		cfg.MaxDailyQuota,
		cfg.MinQuotaRemaining,
		vehicleLearningAverageDays,
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
		log.Printf("vehicle learner: skip, active_sse_subscribers=%d", subscribers)
		return
	}

	plan := currentVehicleLearningPlan(tranzyClient, cfg)
	if plan.CallsRemaining <= 0 {
		log.Printf("vehicle learner: skip, calls_remaining=%d daily_budget=%d avg_vehicle_usage=%.1f avg_days=%d quota_remaining=%d spendable_quota=%d reserve=%d",
			plan.CallsRemaining,
			plan.DailyBudget,
			plan.AverageDailyUsage,
			plan.AverageDays,
			plan.QuotaRemaining,
			plan.SpendableQuotaRemaining,
			cfg.MinQuotaRemaining,
		)
		return
	}

	log.Printf("vehicle learner: sampling vehicles interval=%s cache_shelf=%s daily_budget=%d calls_remaining=%d avg_vehicle_usage=%.1f avg_days=%d quota_remaining=%d reserve=%d",
		plan.Interval,
		plan.ShelfLife,
		plan.DailyBudget,
		plan.CallsRemaining,
		plan.AverageDailyUsage,
		plan.AverageDays,
		plan.QuotaRemaining,
		cfg.MinQuotaRemaining,
	)
	vehicles, err := GetVehicles(tranzyClient, plan.ShelfLife, VehicleFilter{})
	if err != nil {
		log.Printf("vehicle learner: sample failed: %v", err)
		return
	}
	log.Printf("vehicle learner: sample complete vehicles=%d quota_remaining=%d", len(vehicles), tranzyClient.VehiclesQuotaRemaining())
}

func currentVehicleLearningPlan(tranzyClient *tranzy.Client, cfg VehicleLearningSamplerConfig) vehicleLearningPlan {
	avgUsage, avgDays, err := database.GetAverageDailyTranzyQuotaUsage(vehicleLearningQuotaStatsKey, vehicleLearningAverageDays)
	if err != nil {
		log.Printf("vehicle learner: failed to read quota usage average: %v", err)
	}

	return computeVehicleLearningPlan(
		tranzyClient.VehiclesQuotaLimit(),
		tranzyClient.VehiclesQuotaRemaining(),
		cfg.MaxDailyQuota,
		cfg.MinQuotaRemaining,
		avgUsage,
		avgDays,
		time.Now(),
		tranzyClient.Location(),
	)
}

func computeVehicleLearningPlan(
	quotaLimit int,
	quotaRemaining int,
	maxDailyQuota int,
	minQuotaRemaining int,
	averageDailyUsage float64,
	averageDays int,
	now time.Time,
	loc *time.Location,
) vehicleLearningPlan {
	dailyBudget := computeVehicleLearningDailyBudget(quotaLimit, maxDailyQuota, averageDailyUsage)
	callsByClock, untilReset := vehicleLearningCallsRemainingByClock(dailyBudget, now, loc)
	spendableQuota := quotaRemaining - minQuotaRemaining
	if spendableQuota < 0 {
		spendableQuota = 0
	}
	callsRemaining := callsByClock
	if callsRemaining > spendableQuota {
		callsRemaining = spendableQuota
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
		Interval:                interval,
		ShelfLife:               shelfLife,
		DailyBudget:             dailyBudget,
		AverageDailyUsage:       averageDailyUsage,
		AverageDays:             averageDays,
		CallsRemaining:          callsRemaining,
		QuotaRemaining:          quotaRemaining,
		SpendableQuotaRemaining: spendableQuota,
	}
}

func computeVehicleLearningDailyBudget(quotaLimit int, maxDailyQuota int, averageDailyUsage float64) int {
	if quotaLimit <= 0 || maxDailyQuota <= 0 {
		return 0
	}

	quotaLeftOnAverage := float64(quotaLimit) - averageDailyUsage
	if quotaLeftOnAverage <= 0 {
		return 0
	}

	budget := int(math.Floor(quotaLeftOnAverage / 2))
	if budget > maxDailyQuota {
		return maxDailyQuota
	}
	if budget < 0 {
		return 0
	}
	return budget
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
