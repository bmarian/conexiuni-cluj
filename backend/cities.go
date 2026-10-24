package main

import (
	"conexiuni-cluj/cities"
	"conexiuni-cluj/cities/botosani"
	"conexiuni-cluj/cities/bucuresti"
	"conexiuni-cluj/cities/cluj"
	"conexiuni-cluj/cities/constanta"
	"conexiuni-cluj/cities/craiova"
	"conexiuni-cluj/cities/iasi"
	"conexiuni-cluj/cities/oradea"
	"conexiuni-cluj/cities/ploiesti"
	"conexiuni-cluj/cities/sibiu"
	"conexiuni-cluj/cities/suceava"
	"conexiuni-cluj/cities/targumures"
	"conexiuni-cluj/cities/timisoara"
	"conexiuni-cluj/config"
	"conexiuni-cluj/database"
	"conexiuni-cluj/live"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// Every city the server runs; a new one is a folder under cities/ and a line here.
var allCities = []cities.Spec{
	cluj.City, iasi.City, botosani.City, oradea.City, constanta.City,
	sibiu.City, bucuresti.City, timisoara.City, suceava.City,
	ploiesti.City, craiova.City, targumures.City,
}

func buildCities(cfg *config.Config) ([]*cities.City, error) {
	opts, err := cityOptions(cfg)
	if err != nil {
		return nil, err
	}
	var built []*cities.City
	for _, spec := range allCities {
		city, err := buildCity(cfg, spec, opts)
		if errors.Is(err, cities.ErrNoKey) {
			log.Printf("%s: %v, city left out", spec.Slug, err)
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", spec.Slug, err)
		}
		built = append(built, city)
	}
	if len(built) == 0 {
		return nil, fmt.Errorf("no city has a key")
	}
	return built, nil
}

func buildCity(cfg *config.Config, spec cities.Spec, opts live.Options) (*cities.City, error) {
	env := cities.NewEnv(spec.Slug, cfg, func() (*database.Store, error) { return openStore(cfg, spec.Slug) })
	feeds, err := spec.Feeds(env)
	if err != nil {
		return nil, err
	}
	store, err := env.Store()
	if err != nil {
		return nil, err
	}
	opts.LearningEnabled = config.Bool("VEHICLE_LEARNING_ENABLED_"+strings.ToUpper(spec.Slug), opts.LearningEnabled)
	city := cities.New(spec.Slug, spec.Name, store, feeds.Static, feeds.Timetables, feeds.Realtime, feeds.Learning, opts)
	city.Operator, city.OperatorURL, city.News, city.Planner.OwnTicket = spec.Operator, spec.OperatorURL, feeds.News, spec.Rules.OwnTicket
	env.Place(city.Live)
	return city, nil
}

func openStore(cfg *config.Config, slug string) (*database.Store, error) {
	for _, dir := range []string{cfg.DBDir, cfg.ArchiveDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	return database.Open(filepath.Join(cfg.DBDir, slug+".db"), filepath.Join(cfg.ArchiveDir, slug+".db"), cfg.LogRetentionDays)
}

func cityOptions(cfg *config.Config) (live.Options, error) {
	weekday, err := live.ParseSchedule(cfg.VehicleSchedule)
	if err != nil {
		return live.Options{}, fmt.Errorf("VEHICLE_SCHEDULE: %w", err)
	}
	weekend, err := live.ParseSchedule(cfg.VehicleScheduleWeekend)
	if err != nil {
		return live.Options{}, fmt.Errorf("VEHICLE_SCHEDULE_WEEKEND: %w", err)
	}
	return live.Options{
		VehicleInterval: live.VehicleIntervalConfig{
			Weekday: weekday, Weekend: weekend,
			MinInterval: cfg.VehicleMinInterval, MaxInterval: cfg.VehicleMaxInterval,
		},
		LearningEnabled:  cfg.VehicleLearningEnabled,
		LearningMaxQuota: cfg.VehicleLearningMaxQuota,
		// A restart in development costs no quota.
		Refresh: cfg.Environment == "production",
	}, nil
}

func offlineCity(cfg *config.Config, slug string) (*cities.City, error) {
	store, err := openStore(cfg, slug)
	if err != nil {
		return nil, err
	}
	return cities.New(slug, slug, store, nil, nil, nil, nil, live.Options{}), nil
}
