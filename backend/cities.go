package main

import (
	"conexiuni-cluj/database"
	"conexiuni-cluj/handlers"
	"conexiuni-cluj/sources"
	"conexiuni-cluj/sources/ctpcsv"
	"conexiuni-cluj/sources/eltrans"
	"conexiuni-cluj/sources/gtfs"
	"conexiuni-cluj/sources/gtfsrt"
	"conexiuni-cluj/sources/stpt"
	"conexiuni-cluj/sources/tranzy"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type quotaPersister struct {
	slug  string
	store *database.Store
	stats bool
}

func (p quotaPersister) Load(name string) (int, time.Time, error) {
	count, resetAt, err := p.store.LoadTranzyQuota(name)
	if err != nil {
		err = fmt.Errorf("%s: %w", p.slug, err)
	}
	return count, resetAt, err
}
func (p quotaPersister) Save(name string, count int, resetAt time.Time) error {
	if err := p.store.SaveTranzyQuota(name, count, resetAt); err != nil {
		return fmt.Errorf("%s: %w", p.slug, err)
	}
	return nil
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
		// A restart in development costs no quota.
		Refresh: cfg.Environment == "production",
	}, nil
}

type tranzyAgency struct {
	slug, name, operator, agencyID string
	// Tranzy has no departure times; nil leaves only learned ones.
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
	client := tranzy.NewClient(cfg.TranzyBaseUrl, key, a.agencyID, cfg.TranzyRateLimit, cfg.TranzyVehiclesDailyQuota, cfg.TranzyDefaultDailyQuota, quotaPersister{a.slug, store, true})
	static := tranzy.New(client)
	var learning sources.Realtime
	if learningKey != "" && learningKey != key {
		learningClient := tranzy.NewClientWithQuotaNames(cfg.TranzyBaseUrl, learningKey, a.agencyID, cfg.TranzyRateLimit, cfg.TranzyVehiclesDailyQuota, cfg.TranzyDefaultDailyQuota,
			tranzy.QuotaNames{Vehicles: "vehicle_learning_vehicles", Default: "vehicle_learning_default"}, quotaPersister{a.slug, store, false})
		learning = tranzy.New(learningClient)
	}
	city := handlers.NewCity(a.slug, a.name, store, static, a.timetables, static, learning, learningFor(a.slug, opts))
	city.Operator = a.operator
	return city, nil
}

func learningFor(slug string, opts handlers.Options) handlers.Options {
	opts.LearningEnabled = getBool("VEHICLE_LEARNING_ENABLED_"+strings.ToUpper(slug), opts.LearningEnabled)
	return opts
}

func buildCities(cfg *Config) ([]*handlers.City, error) {
	opts, err := cityOptions(cfg)
	if err != nil {
		return nil, err
	}
	feed := func(city string) sources.Timetables {
		return gtfs.New("https://external.gtfs.ro/"+city+"/"+strings.ToUpper(city)+".zip", cfg.TimetableRefresh, nil)
	}
	agencies := []tranzyAgency{
		{"cluj", "Cluj-Napoca", "CTP Cluj-Napoca", "2", ctpcsv.New(cfg.CtpCsvBaseUrl, cfg.CtpCjRateLimit, cfg.TimetableRefresh)},
		{"iasi", "Iași", "CTP Iași", "1", feed("iasi")},
		{"botosani", "Botoșani", "Eltrans Botoșani", "6", eltrans.New("https://primariabt.ro/storage/2020/07/microbusautobus.pdf", cfg.TimetableRefresh)},
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
	gtfsRealtime := func(url string) func(sources.IDMap) sources.Realtime {
		return func(ids sources.IDMap) sources.Realtime { return gtfsrt.New(url, ids) }
	}
	stptMap := func(url string) func(sources.IDMap) sources.Realtime {
		return func(ids sources.IDMap) sources.Realtime { return stpt.New(url, ids) }
	}
	feeds := []gtfsCity{
		{"sibiu", "Sibiu", "Tursib", "https://tursib.ro/trasee/gtfs", nil},
		{"bucuresti", "București", "TPBI", "https://gtfs.tpbi.ro/regional/BUCHAREST-REGION.zip", gtfsRealtime("https://gtfs.tpbi.ro/api/gtfs-rt/vehiclePositions")},
		{"timisoara", "Timișoara", "STPT Timișoara", "https://smtt.ro/transit/gtfs/maps/smtt-timisoara-ro.zip", stptMap("https://live.stpt.ro/gtfs-vehicles.php")},
	}
	for _, g := range feeds {
		city, err := g.city(cfg, opts)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", g.slug, err)
		}
		cities = append(cities, city)
	}
	if len(cities) == 0 {
		return nil, fmt.Errorf("no city has a key")
	}
	return cities, nil
}

type gtfsCity struct {
	slug, name, operator, url string
	// nil means schedule-only.
	realtime func(sources.IDMap) sources.Realtime
}

func (g gtfsCity) city(cfg *Config, opts handlers.Options) (*handlers.City, error) {
	store, err := openStore(cfg, g.slug)
	if err != nil {
		return nil, err
	}
	feed := gtfs.New(g.url, cfg.TimetableRefresh, store)
	var realtime sources.Realtime
	if g.realtime != nil {
		realtime = g.realtime(store)
	}
	city := handlers.NewCity(g.slug, g.name, store, feed, nil, realtime, nil, learningFor(g.slug, opts))
	city.Operator = g.operator
	if rt, ok := realtime.(*gtfsrt.Source); ok {
		rt.Placement = city
	}
	return city, nil
}

func offlineCity(cfg *Config, slug string) (*handlers.City, error) {
	store, err := openStore(cfg, slug)
	if err != nil {
		return nil, err
	}
	return handlers.NewCity(slug, slug, store, nil, nil, nil, nil, handlers.Options{}), nil
}
