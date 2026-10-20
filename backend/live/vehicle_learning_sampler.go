package live

import (
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/jobs"
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
	eng     *Engine
	mu      sync.Mutex
	name    string
	loc     *time.Location
	count   int
	resetAt time.Time
}

type learnerState struct {
	sync.RWMutex
	source      feeds.Realtime
	cfg         VehicleLearningSamplerConfig
	counter     *vehicleLearningDailyCounter
	initialized bool
}

func (eng *Engine) StartVehicleLearningSampler(r *jobs.Runner) {
	cfg := VehicleLearningSamplerConfig{
		Enabled:                eng.Options.LearningEnabled && eng.Learning != nil,
		MaxDailyQuota:          eng.Options.LearningMaxQuota,
		UsesDedicatedTranzyKey: eng.Learning != nil && eng.Learning != eng.Realtime,
	}
	if !cfg.Enabled {
		eng.rememberVehicleLearningSamplerRuntime(cfg, nil)
		eng.log("vehicle learner: disabled")
		return
	}
	if cfg.MaxDailyQuota <= 0 {
		eng.rememberVehicleLearningSamplerRuntime(cfg, nil)
		eng.log("vehicle learner: disabled because max_daily_quota=%d", cfg.MaxDailyQuota)
		return
	}
	eng.log("vehicle learner: enabled max_daily_quota=%d dedicated_tranzy_key=%t",
		cfg.MaxDailyQuota,
		cfg.UsesDedicatedTranzyKey,
	)
	counter := eng.newVehicleLearningDailyCounter(vehicleLearningQuotaName, eng.Location)
	eng.rememberVehicleLearningSamplerRuntime(cfg, counter)
	r.Go(func() {
		for {
			plan := eng.currentVehicleLearningPlan(cfg, counter)
			delay := plan.Interval
			if delay <= 0 {
				delay = vehicleLearningNoBudgetCheckInterval
			}
			if !r.Sleep(delay) {
				return
			}
			eng.runVehicleLearningSample(cfg, counter)
		}
	})
}

func (eng *Engine) rememberVehicleLearningSamplerRuntime(cfg VehicleLearningSamplerConfig, counter *vehicleLearningDailyCounter) {
	eng.learner.Lock()
	eng.learner.source = eng.Learning
	eng.learner.cfg = cfg
	eng.learner.counter = counter
	eng.learner.initialized = true
	eng.learner.Unlock()
}

type LearnerStatus struct {
	Enabled   bool
	Dedicated bool
	Used      int
	Budget    int
	Interval  time.Duration
}

func (eng *Engine) currentVehicleLearningStatus() LearnerStatus {
	eng.learner.RLock()
	initialized := eng.learner.initialized
	source := eng.learner.source
	cfg := eng.learner.cfg
	counter := eng.learner.counter
	eng.learner.RUnlock()

	status := LearnerStatus{
		Enabled:   initialized && cfg.Enabled && cfg.MaxDailyQuota > 0,
		Dedicated: cfg.UsesDedicatedTranzyKey,
	}
	if source == nil || !status.Enabled {
		return status
	}
	plan := eng.currentVehicleLearningPlan(cfg, counter)
	status.Used, status.Budget, status.Interval = plan.LearnerUsed, plan.DailyBudget, plan.Interval
	return status
}

func (eng *Engine) runVehicleLearningSample(cfg VehicleLearningSamplerConfig, counter *vehicleLearningDailyCounter) {
	subscribers := 0
	if eng.hub != nil {
		subscribers = eng.hub.SubscriberCount()
	}
	if subscribers > 0 {
		return
	}

	plan := eng.currentVehicleLearningPlan(cfg, counter)
	if plan.CallsRemaining <= 0 || !plan.Ready {
		eng.log("vehicle learner: skip, ready=%t interval=%s learner_used=%d vehicles_used=%d calls_remaining=%d daily_budget=%d quota_remaining=%d",
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
	_, err := eng.observeVehicles(eng.learner.source)
	if err != nil {
		eng.log("vehicle learner: sample failed interval=%s learner_used=%d vehicles_used=%d daily_budget=%d calls_remaining=%d quota_remaining=%d err=%v",
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

func (eng *Engine) currentVehicleLearningPlan(cfg VehicleLearningSamplerConfig, counter *vehicleLearningDailyCounter) vehicleLearningPlan {
	now := time.Now()
	learnerUsed := 0
	if counter != nil {
		learnerUsed = counter.Count(now)
	}
	budget := eng.learner.source.Budget()
	return computeVehicleLearningPlan(
		budget.Remaining,
		budget.Limit,
		learnerUsed,
		cfg.MaxDailyQuota,
		now,
		eng.Location,
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
	// A zero limit is unmetered.
	if quotaLimit > 0 && callsRemaining > quotaRemaining {
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
			interval = max(interval, vehicleLearningDelayUntilAllowed(learnerUsed, dailyBudget, dayStart, dayDuration, now))
		}
		if quotaLimit > 0 && !quotaReady {
			interval = max(interval, vehicleLearningDelayUntilAllowed(vehiclesUsed, quotaLimit, dayStart, dayDuration, now))
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

func (eng *Engine) newVehicleLearningDailyCounter(name string, loc *time.Location) *vehicleLearningDailyCounter {
	if loc == nil {
		loc = time.Local
	}
	counter := &vehicleLearningDailyCounter{eng: eng, name: name, loc: loc}
	count, resetAt, err := eng.LoadQuota(name)
	if err != nil {
		eng.log("vehicle learner: failed to load persisted quota %q: %v", name, err)
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
	if err := counter.eng.SaveQuota(counter.name, counter.count, counter.resetAt); err != nil {
		counter.eng.log("vehicle learner: failed to persist quota %q: %v", counter.name, err)
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
