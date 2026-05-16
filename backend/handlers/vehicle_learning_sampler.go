package handlers

import (
	"conexiuni-cluj/database"
	"conexiuni-cluj/services/tranzy"
	"log"
	"math"
	"sync"
	"time"
)

const (
	vehicleLearningNoBudgetCheckInterval = time.Hour
	vehicleLearningQuotaName             = "vehicle_learning"
)

type VehicleLearningSamplerConfig struct {
	Enabled                bool
	MaxDailyQuota          int
	UsesDedicatedTranzyKey bool
}

type vehicleLearningPlan struct {
	Interval       time.Duration
	ShelfLife      time.Duration
	DailyBudget    int
	CallsRemaining int
	QuotaRemaining int
	LearnerUsed    int
	VehiclesUsed   int
	Ready          bool
}

type vehicleLearningDailyCounter struct {
	mu      sync.Mutex
	name    string
	loc     *time.Location
	count   int
	resetAt time.Time
}

var vehicleLearningSamplerRuntime = struct {
	sync.RWMutex
	client         *tranzy.Client
	cfg            VehicleLearningSamplerConfig
	counter        *vehicleLearningDailyCounter
	initialized    bool
	disabledReason string
}{}

func StartVehicleLearningSampler(tranzyClient *tranzy.Client, cfg VehicleLearningSamplerConfig) {
	if !cfg.Enabled {
		rememberVehicleLearningSamplerRuntime(tranzyClient, cfg, nil, "disabled by VEHICLE_LEARNING_ENABLED")
		log.Printf("vehicle learner: disabled")
		return
	}
	if cfg.MaxDailyQuota <= 0 {
		rememberVehicleLearningSamplerRuntime(tranzyClient, cfg, nil, "max_daily_quota <= 0")
		log.Printf("vehicle learner: disabled because max_daily_quota=%d", cfg.MaxDailyQuota)
		return
	}
	log.Printf("vehicle learner: enabled max_daily_quota=%d dedicated_tranzy_key=%t",
		cfg.MaxDailyQuota,
		cfg.UsesDedicatedTranzyKey,
	)
	counter := newVehicleLearningDailyCounter(vehicleLearningQuotaName, tranzyClient.Location())
	rememberVehicleLearningSamplerRuntime(tranzyClient, cfg, counter, "")
	go func() {
		for {
			plan := currentVehicleLearningPlan(tranzyClient, cfg, counter)
			delay := plan.Interval
			if delay <= 0 {
				delay = vehicleLearningNoBudgetCheckInterval
			}
			time.Sleep(delay)
			runVehicleLearningSample(tranzyClient, cfg, counter)
		}
	}()
}

func rememberVehicleLearningSamplerRuntime(tranzyClient *tranzy.Client, cfg VehicleLearningSamplerConfig, counter *vehicleLearningDailyCounter, disabledReason string) {
	vehicleLearningSamplerRuntime.Lock()
	vehicleLearningSamplerRuntime.client = tranzyClient
	vehicleLearningSamplerRuntime.cfg = cfg
	vehicleLearningSamplerRuntime.counter = counter
	vehicleLearningSamplerRuntime.initialized = true
	vehicleLearningSamplerRuntime.disabledReason = disabledReason
	vehicleLearningSamplerRuntime.Unlock()
}

func currentVehicleLearningQuotaSnapshot() vehicleLearningQuotaSnapshot {
	vehicleLearningSamplerRuntime.RLock()
	initialized := vehicleLearningSamplerRuntime.initialized
	tranzyClient := vehicleLearningSamplerRuntime.client
	cfg := vehicleLearningSamplerRuntime.cfg
	counter := vehicleLearningSamplerRuntime.counter
	disabledReason := vehicleLearningSamplerRuntime.disabledReason
	vehicleLearningSamplerRuntime.RUnlock()

	snapshot := vehicleLearningQuotaSnapshot{
		Enabled:                initialized && cfg.Enabled && cfg.MaxDailyQuota > 0,
		DisabledReason:         disabledReason,
		UsesDedicatedTranzyKey: cfg.UsesDedicatedTranzyKey,
		MaxDailyQuota:          cfg.MaxDailyQuota,
	}
	if tranzyClient == nil {
		return snapshot
	}
	now := time.Now()
	plan := currentVehicleLearningPlan(tranzyClient, cfg, counter)
	snapshot.DailyBudget = plan.DailyBudget
	snapshot.CallsUsed = plan.LearnerUsed
	snapshot.CallsRemaining = plan.CallsRemaining
	snapshot.VehiclesUsed = plan.VehiclesUsed
	snapshot.VehiclesRemaining = plan.QuotaRemaining
	snapshot.VehiclesLimit = tranzyClient.VehiclesQuotaLimit()
	snapshot.Ready = plan.Ready
	if !snapshot.Enabled {
		snapshot.CallsRemaining = 0
		snapshot.Ready = false
	}
	snapshot.IntervalMs = plan.Interval.Milliseconds()
	snapshot.ShelfLifeMs = plan.ShelfLife.Milliseconds()
	if counter != nil {
		_, resetAt := counter.Snapshot(now)
		if !resetAt.IsZero() {
			snapshot.ResetAt = resetAt.UTC().Format(time.RFC3339)
		}
	}
	return snapshot
}

func runVehicleLearningSample(tranzyClient *tranzy.Client, cfg VehicleLearningSamplerConfig, counter *vehicleLearningDailyCounter) {
	subscribers := 0
	if VehicleHub != nil {
		subscribers = VehicleHub.SubscriberCount()
	}
	if subscribers > 0 {
		return
	}

	plan := currentVehicleLearningPlan(tranzyClient, cfg, counter)
	if plan.CallsRemaining <= 0 || !plan.Ready {
		log.Printf("vehicle learner: skip, ready=%t interval=%s learner_used=%d vehicles_used=%d calls_remaining=%d daily_budget=%d quota_remaining=%d",
			plan.Ready,
			plan.Interval,
			plan.LearnerUsed,
			plan.VehiclesUsed,
			plan.CallsRemaining,
			plan.DailyBudget,
			plan.QuotaRemaining,
		)
		return
	}

	counter.Record(time.Now())
	_, err := GetVehicles(tranzyClient, plan.ShelfLife, VehicleFilter{})
	if err != nil {
		log.Printf("vehicle learner: sample failed interval=%s learner_used=%d vehicles_used=%d daily_budget=%d calls_remaining=%d quota_remaining=%d err=%v",
			plan.Interval,
			plan.LearnerUsed+1,
			plan.VehiclesUsed,
			plan.DailyBudget,
			plan.CallsRemaining,
			plan.QuotaRemaining,
			err,
		)
		return
	}
}

func currentVehicleLearningPlan(tranzyClient *tranzy.Client, cfg VehicleLearningSamplerConfig, counter *vehicleLearningDailyCounter) vehicleLearningPlan {
	now := time.Now()
	learnerUsed := 0
	if counter != nil {
		learnerUsed = counter.Count(now)
	}
	return computeVehicleLearningPlan(
		tranzyClient.VehiclesQuotaRemaining(),
		tranzyClient.VehiclesQuotaLimit(),
		learnerUsed,
		cfg.MaxDailyQuota,
		now,
		tranzyClient.Location(),
	)
}

func computeVehicleLearningPlan(
	quotaRemaining int,
	quotaLimit int,
	learnerUsed int,
	maxDailyQuota int,
	now time.Time,
	loc *time.Location,
) vehicleLearningPlan {
	dailyBudget := maxDailyQuota
	if quotaLimit > 0 && dailyBudget > quotaLimit {
		dailyBudget = quotaLimit
	}
	if quotaRemaining < 0 {
		quotaRemaining = 0
	}
	if learnerUsed < 0 {
		learnerUsed = 0
	}
	vehiclesUsed := 0
	if quotaLimit > 0 {
		vehiclesUsed = quotaLimit - quotaRemaining
		if vehiclesUsed < 0 {
			vehiclesUsed = 0
		}
	}

	dayStart, dayDuration, untilReset := vehicleLearningDayWindow(now, loc)
	learnerRemaining := dailyBudget - learnerUsed
	if learnerRemaining < 0 {
		learnerRemaining = 0
	}
	callsRemaining := learnerRemaining
	if callsRemaining > quotaRemaining {
		callsRemaining = quotaRemaining
	}

	learnerAllowed := vehicleLearningCallsAllowedByClock(dailyBudget, dayStart, dayDuration, now)
	quotaAllowed := vehicleLearningCallsAllowedByClock(quotaLimit, dayStart, dayDuration, now)
	learnerReady := learnerUsed < learnerAllowed
	quotaReady := quotaLimit <= 0 || vehiclesUsed < quotaAllowed
	ready := callsRemaining > 0 && untilReset > 0 && learnerReady && quotaReady

	interval := time.Duration(0)
	if callsRemaining > 0 && untilReset > 0 && dailyBudget > 0 {
		interval = vehicleLearningFairInterval(dailyBudget, dayDuration)
		if !learnerReady {
			interval = maxDuration(interval, vehicleLearningDelayUntilAllowed(learnerUsed, dailyBudget, dayStart, dayDuration, now))
		}
		if quotaLimit > 0 && !quotaReady {
			interval = maxDuration(interval, vehicleLearningDelayUntilAllowed(vehiclesUsed, quotaLimit, dayStart, dayDuration, now))
		}
		if interval > untilReset {
			interval = untilReset
		}
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
		LearnerUsed:    learnerUsed,
		VehiclesUsed:   vehiclesUsed,
		Ready:          ready,
	}
}

func newVehicleLearningDailyCounter(name string, loc *time.Location) *vehicleLearningDailyCounter {
	if loc == nil {
		loc = time.Local
	}
	counter := &vehicleLearningDailyCounter{name: name, loc: loc}
	count, resetAt, err := database.LoadTranzyQuota(name)
	if err != nil {
		log.Printf("vehicle learner: failed to load persisted quota %q: %v", name, err)
	} else {
		counter.count = count
		counter.resetAt = resetAt
	}
	counter.mu.Lock()
	counter.rolloverLocked(time.Now())
	counter.mu.Unlock()
	return counter
}

func (counter *vehicleLearningDailyCounter) Count(now time.Time) int {
	counter.mu.Lock()
	defer counter.mu.Unlock()
	counter.rolloverLocked(now)
	return counter.count
}

func (counter *vehicleLearningDailyCounter) Snapshot(now time.Time) (int, time.Time) {
	counter.mu.Lock()
	defer counter.mu.Unlock()
	counter.rolloverLocked(now)
	return counter.count, counter.resetAt
}

func (counter *vehicleLearningDailyCounter) Record(now time.Time) {
	counter.mu.Lock()
	defer counter.mu.Unlock()
	counter.rolloverLocked(now)
	counter.count++
	counter.persistLocked()
}

func (counter *vehicleLearningDailyCounter) rolloverLocked(now time.Time) {
	if counter.loc == nil {
		counter.loc = time.Local
	}
	localNow := now.In(counter.loc)
	if counter.resetAt.IsZero() || !localNow.Before(counter.resetAt) {
		counter.count = 0
		y, m, d := localNow.Date()
		counter.resetAt = time.Date(y, m, d+1, 0, 0, 0, 0, counter.loc)
		counter.persistLocked()
	}
}

func (counter *vehicleLearningDailyCounter) persistLocked() {
	if err := database.SaveTranzyQuota(counter.name, counter.count, counter.resetAt); err != nil {
		log.Printf("vehicle learner: failed to persist quota %q: %v", counter.name, err)
	}
}

func vehicleLearningDayWindow(now time.Time, loc *time.Location) (time.Time, time.Duration, time.Duration) {
	if loc == nil {
		loc = time.Local
	}
	localNow := now.In(loc)
	y, m, d := localNow.Date()
	dayStart := time.Date(y, m, d, 0, 0, 0, 0, loc)
	nextReset := dayStart.AddDate(0, 0, 1)
	dayDuration := nextReset.Sub(dayStart)
	untilReset := nextReset.Sub(localNow)
	if dayDuration <= 0 {
		dayDuration = 24 * time.Hour
	}
	if untilReset < 0 {
		untilReset = 0
	}
	return dayStart, dayDuration, untilReset
}

func vehicleLearningCallsAllowedByClock(dailyBudget int, dayStart time.Time, dayDuration time.Duration, now time.Time) int {
	if dailyBudget <= 0 || dayDuration <= 0 {
		return 0
	}
	elapsed := now.Sub(dayStart)
	if elapsed <= 0 {
		return 0
	}
	if elapsed >= dayDuration {
		return dailyBudget
	}
	return int(math.Floor(float64(dailyBudget) * elapsed.Seconds() / dayDuration.Seconds()))
}

func vehicleLearningDelayUntilAllowed(used int, dailyBudget int, dayStart time.Time, dayDuration time.Duration, now time.Time) time.Duration {
	if dailyBudget <= 0 || dayDuration <= 0 {
		return 0
	}
	if used < 0 {
		used = 0
	}
	if used >= dailyBudget {
		return dayStart.Add(dayDuration).Sub(now)
	}
	next := dayStart.Add(time.Duration(float64(dayDuration) * float64(used+1) / float64(dailyBudget)))
	delay := next.Sub(now)
	if delay < 0 {
		return 0
	}
	return delay
}

func vehicleLearningFairInterval(dailyBudget int, dayDuration time.Duration) time.Duration {
	if dailyBudget <= 0 || dayDuration <= 0 {
		return 0
	}
	return time.Duration(float64(dayDuration) / float64(dailyBudget))
}

func maxDuration(a, b time.Duration) time.Duration {
	if a > b {
		return a
	}
	return b
}
