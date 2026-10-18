package main

import (
	"conexiuni-cluj/database"
	"conexiuni-cluj/handlers"
	"conexiuni-cluj/sources"
	"conexiuni-cluj/sources/ctpcsv"
	"conexiuni-cluj/sources/tranzy"
	"fmt"
	"path/filepath"
	"time"
)

// quotaPersister keeps Tranzy's daily counts across restarts and notes each call in the stats.
type quotaPersister struct {
	store *database.Store
	stats bool
}

func (p quotaPersister) Load(name string) (int, time.Time, error) {
	return p.store.LoadTranzyQuota(name)
}
func (p quotaPersister) Save(name string, count int, resetAt time.Time) error {
	return p.store.SaveTranzyQuota(name, count, resetAt)
}
func (p quotaPersister) RecordUsage(name string) {
	if p.stats {
		p.store.RecordTranzyQuotaUsage(name)
	}
}

func openStore(cfg *Config, slug string) (*database.Store, error) {
	return database.Open(filepath.Join(cfg.DataDir, slug+".db"), filepath.Join(cfg.DataDir, slug+"-archive.db"), cfg.LogRetentionDays)
}

func cityOptions(cfg *Config) (handlers.Options, error) {
	weekday, err := handlers.ParseSchedule(cfg.VehicleSchedule)
	if err != nil {
		return handlers.Options{}, fmt.Errorf("VEHICLE_SCHEDULE: %w", err)
	}
	weekend, err := handlers.ParseSchedule(cfg.VehicleScheduleWeekend)
	if err != nil {
		return handlers.Options{}, fmt.Errorf("VEHICLE_SCHEDULE_WEEKEND: %w", err)
	}
	return handlers.Options{
		VehicleInterval: handlers.VehicleIntervalConfig{
			Weekday: weekday, Weekend: weekend,
			MinInterval: cfg.VehicleMinInterval, MaxInterval: cfg.VehicleMaxInterval,
		},
		LearningEnabled:  cfg.VehicleLearningEnabled,
		LearningMaxQuota: cfg.VehicleLearningMaxQuota,
		// Development reuses whatever the database holds, so a restart costs no quota.
		Refresh: cfg.Environment == "production",
	}, nil
}

// A Tranzy agency, with a second key for learning samples when one is configured.
func tranzyCity(cfg *Config, slug, name, operator, agencyID string, opts handlers.Options) (*handlers.City, error) {
	store, err := openStore(cfg, slug)
	if err != nil {
		return nil, err
	}
	client := tranzy.NewClient(cfg.TranzyBaseUrl, cfg.TranzyApiKey, agencyID, cfg.TranzyRateLimit, cfg.TranzyVehiclesDailyQuota, cfg.TranzyDefaultDailyQuota, quotaPersister{store, true})
	static := tranzy.New(client)
	var learning sources.Realtime
	if cfg.TranzyLearningApiKey != "" && cfg.TranzyLearningApiKey != cfg.TranzyApiKey {
		learningClient := tranzy.NewClientWithQuotaNames(cfg.TranzyBaseUrl, cfg.TranzyLearningApiKey, agencyID, cfg.TranzyRateLimit, cfg.TranzyVehiclesDailyQuota, cfg.TranzyDefaultDailyQuota,
			tranzy.QuotaNames{Vehicles: "vehicle_learning_vehicles", Default: "vehicle_learning_default"}, quotaPersister{store, false})
		learning = tranzy.New(learningClient)
	}
	city := handlers.NewCity(slug, name, store, static, nil, static, learning, opts)
	city.Operator = operator
	return city, nil
}

// Every city served, in the order shown to people.
func buildCities(cfg *Config) ([]*handlers.City, error) {
	opts, err := cityOptions(cfg)
	if err != nil {
		return nil, err
	}
	cluj, err := tranzyCity(cfg, "cluj", "Cluj-Napoca", "CTP Cluj-Napoca", "2", opts)
	if err != nil {
		return nil, fmt.Errorf("cluj: %w", err)
	}
	// Tranzy carries no departure times for Cluj; CTP's own CSVs do.
	cluj.Timetables = ctpcsv.New(cfg.CtpCsvBaseUrl, cfg.CtpCjRateLimit, cfg.TimetableRefresh)
	return []*handlers.City{cluj}, nil
}

// The city a maintenance command works on, with its database but no sources.
func offlineCity(cfg *Config, slug string) (*handlers.City, error) {
	store, err := openStore(cfg, slug)
	if err != nil {
		return nil, err
	}
	return handlers.NewCity(slug, slug, store, nil, nil, nil, nil, handlers.Options{}), nil
}
