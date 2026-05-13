package handlers

import (
	"conexiuni-cluj/services/tranzy"
	"log"
	"time"
)

type VehicleLearningSamplerConfig struct {
	Enabled           bool
	Interval          time.Duration
	MinQuotaRemaining int
}

func StartVehicleLearningSampler(tranzyClient *tranzy.Client, cfg VehicleLearningSamplerConfig) {
	if !cfg.Enabled {
		log.Printf("vehicle learner: disabled")
		return
	}
	if cfg.Interval <= 0 {
		log.Printf("vehicle learner: disabled because interval=%s", cfg.Interval)
		return
	}
	if cfg.MinQuotaRemaining < 0 {
		cfg.MinQuotaRemaining = 0
	}

	initialDelay := cfg.Interval
	if initialDelay > time.Minute {
		initialDelay = time.Minute
	}

	log.Printf("vehicle learner: enabled interval=%s quota_reserve=%d", cfg.Interval, cfg.MinQuotaRemaining)
	go func() {
		timer := time.NewTimer(initialDelay)
		defer timer.Stop()

		for {
			<-timer.C
			runVehicleLearningSample(tranzyClient, cfg)
			timer.Reset(cfg.Interval)
		}
	}()
}

func runVehicleLearningSample(tranzyClient *tranzy.Client, cfg VehicleLearningSamplerConfig) {
	if VehicleHub != nil && VehicleHub.SubscriberCount() > 0 {
		return
	}
	remaining := tranzyClient.VehiclesQuotaRemaining()
	if remaining <= cfg.MinQuotaRemaining {
		log.Printf("vehicle learner: skip, quota remaining=%d reserve=%d", remaining, cfg.MinQuotaRemaining)
		return
	}
	shelfLife := cfg.Interval
	if shelfLife > time.Second {
		shelfLife -= time.Second
	}
	if _, err := GetVehicles(tranzyClient, shelfLife, VehicleFilter{}); err != nil {
		log.Printf("vehicle learner: sample failed: %v", err)
		return
	}
}
