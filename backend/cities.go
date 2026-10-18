package main

import (
	"conexiuni-cluj/database"
	"conexiuni-cluj/handlers"
	"conexiuni-cluj/sources"
	"conexiuni-cluj/sources/ctpcsv"
	"conexiuni-cluj/sources/gtfs"
	"conexiuni-cluj/sources/tranzy"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
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
	for _, dir := range []string{cfg.DBDir, cfg.ArchiveDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	return database.Open(filepath.Join(cfg.DBDir, slug+".db"), filepath.Join(cfg.ArchiveDir, slug+".db"), cfg.LogRetentionDays)
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

// A Tranzy agency. Its keys are TRANZY_API_KEY_{SLUG} and, for learning samples, TRANZY_API_KEY_{SLUG}_LEARNING.
type tranzyAgency struct {
	slug, name, operator, agencyID string
	// Tranzy carries no departure times; nil leaves the city with live buses and learned departures only.
	timetables sources.Timetables
}

func (a tranzyAgency) keys() (key, learning string) {
	name := "TRANZY_API_KEY_" + strings.ToUpper(a.slug)
	return os.Getenv(name), os.Getenv(name + "_LEARNING")
}

func (a tranzyAgency) city(cfg *Config, opts handlers.Options) (*handlers.City, error) {
	key, learningKey := a.keys()
	store, err := openStore(cfg, a.slug)
	if err != nil {
		return nil, err
	}
	client := tranzy.NewClient(cfg.TranzyBaseUrl, key, a.agencyID, cfg.TranzyRateLimit, cfg.TranzyVehiclesDailyQuota, cfg.TranzyDefaultDailyQuota, quotaPersister{store, true})
	static := tranzy.New(client)
	var learning sources.Realtime
	if learningKey != "" && learningKey != key {
		learningClient := tranzy.NewClientWithQuotaNames(cfg.TranzyBaseUrl, learningKey, a.agencyID, cfg.TranzyRateLimit, cfg.TranzyVehiclesDailyQuota, cfg.TranzyDefaultDailyQuota,
			tranzy.QuotaNames{Vehicles: "vehicle_learning_vehicles", Default: "vehicle_learning_default"}, quotaPersister{store, false})
		learning = tranzy.New(learningClient)
	}
	city := handlers.NewCity(a.slug, a.name, store, static, a.timetables, static, learning, opts)
	city.Operator = a.operator
	return city, nil
}

// Every city served, in the order shown to people. A Tranzy city without a key is left out with a note.
func buildCities(cfg *Config) ([]*handlers.City, error) {
	opts, err := cityOptions(cfg)
	if err != nil {
		return nil, err
	}
	feed := func(city string) sources.Timetables {
		return gtfs.New("https://external.gtfs.ro/"+city+"/"+strings.ToUpper(city)+".zip", cfg.TimetableRefresh)
	}
	agencies := []tranzyAgency{
		// CTP's own CSVs are the better timetable for Cluj.
		{"cluj", "Cluj-Napoca", "CTP Cluj-Napoca", "2", ctpcsv.New(cfg.CtpCsvBaseUrl, cfg.CtpCjRateLimit, cfg.TimetableRefresh)},
		{"iasi", "Iași", "CTP Iași", "1", feed("iasi")},
		// Eltrans publishes no feed with times.
		{"botosani", "Botoșani", "Eltrans Botoșani", "6", nil},
		{"oradea", "Oradea", "OTL Oradea", "9", feed("oradea")},
		{"constanta", "Constanța", "CT BUS Constanța", "10", feed("constanta")},
	}
	var cities []*handlers.City
	for _, a := range agencies {
		if key, _ := a.keys(); key == "" {
			log.Printf("%s: no TRANZY_API_KEY_%s, city left out", a.slug, strings.ToUpper(a.slug))
			continue
		}
		city, err := a.city(cfg, opts)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", a.slug, err)
		}
		cities = append(cities, city)
	}
	if len(cities) == 0 {
		return nil, fmt.Errorf("no city has a key")
	}
	return cities, nil
}

// The city a maintenance command works on, with its database but no sources.
func offlineCity(cfg *Config, slug string) (*handlers.City, error) {
	store, err := openStore(cfg, slug)
	if err != nil {
		return nil, err
	}
	return handlers.NewCity(slug, slug, store, nil, nil, nil, nil, handlers.Options{}), nil
}
