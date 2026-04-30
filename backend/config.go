package main

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Environment               string
	LogDir                    string
	LogRetentionDays          int
	LogIPHashSalt             string
	TranzyBaseUrl             string
	ClujAgencyId              string
	CtpCsvBaseUrl             string
	Port                      string
	DatabasePath              string
	TranzyApiKey              string
	ShapeCacheShelfLife       time.Duration
	RouteCacheShelfLife       time.Duration
	TripCacheShelfLife        time.Duration
	StopCacheShelfLife        time.Duration
	TimetableCacheShelfLife   time.Duration
	StopTimeCacheShelfLife    time.Duration
	APIStopTimeCacheShelfLife time.Duration
	CtpCjRateLimit            time.Duration
	TranzyRateLimit           time.Duration
	StopInfoCacheShelfLife    time.Duration
	DirectionsCacheShelfLife  time.Duration
	TranzyVehiclesDailyQuota  int
	TranzyDefaultDailyQuota   int
	VehicleSchedule           string
	VehicleScheduleWeekend    string
	VehicleMinInterval        time.Duration
	VehicleMaxInterval        time.Duration
	ORSApiKey                 string
	ORSBaseURL                string
	ORSDailyQuota             int
	ORSMinuteQuota            int
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value != "" {
		return value
	}
	return defaultValue
}

func getInt(key string, defaultValue int) int {
	value := os.Getenv(key)
	if value != "" {
		n, err := strconv.Atoi(value)
		if err != nil {
			log.Printf("Invalid %s, using default %d: %v", key, defaultValue, err)
			return defaultValue
		}
		return n
	}
	return defaultValue
}

func getDuration(key string, defaultValue time.Duration) time.Duration {
	value := os.Getenv(key)
	if value != "" {
		duration, err := time.ParseDuration(value)
		if err != nil {
			log.Printf("Invalid %s, using default %v: %v", key, defaultValue, err)
			return defaultValue
		}
		return duration
	}
	return defaultValue
}

func Load() *Config {
	loaded := false
	if err := godotenv.Load(".env", "keys.env"); err == nil {
		loaded = true
	}
	if err := godotenv.Load("../.env", "../keys.env"); err == nil {
		loaded = true
	}
	if !loaded {
		log.Fatal("No .env files found")
	}

	return &Config{
		Environment:               getEnv("ENV", "development"),
		LogDir:                    getEnv("LOG_DIR", "../logs"),
		LogRetentionDays:          getInt("LOG_RETENTION_DAYS", 5),
		LogIPHashSalt:             getEnv("LOG_IP_HASH_SALT", ""),
		TranzyBaseUrl:             getEnv("TRANZY_BASE_URL", "https://api.tranzy.ai/v1/opendata"),
		ClujAgencyId:              getEnv("CLUJ_AGENCY_ID", "2"),
		CtpCsvBaseUrl:             getEnv("CTP_CSV_BASE_URL", "https://ctpcj.ro/orare/csv"),
		Port:                      getEnv("PORT", "6698"),
		DatabasePath:              getEnv("DATABASE_PATH", "../conexiuni-cluj.db"),
		TranzyApiKey:              getEnv("TRANZY_API_KEY", ""),
		ShapeCacheShelfLife:       getDuration("SHAPE_CACHE_SHELF_LIFE", 7*24*time.Hour),
		RouteCacheShelfLife:       getDuration("ROUTE_CACHE_SHELF_LIFE", 7*24*time.Hour),
		TripCacheShelfLife:        getDuration("TRIP_CACHE_SHELF_LIFE", 7*24*time.Hour),
		StopCacheShelfLife:        getDuration("STOP_CACHE_SHELF_LIFE", 7*24*time.Hour),
		TimetableCacheShelfLife:   getDuration("TIMETABLE_CACHE_SHELF_LIFE", 7*24*time.Hour),
		StopTimeCacheShelfLife:    getDuration("STOP_TIME_CACHE_SHELF_LIFE", 7*24*time.Hour),
		APIStopTimeCacheShelfLife: getDuration("API_STOP_TIME_CACHE_SHELF_LIFE", 7*24*time.Hour),
		StopInfoCacheShelfLife:    getDuration("STOP_INFO_CACHE_SHELF_LIFE", 7*24*time.Hour),
		DirectionsCacheShelfLife:  getDuration("DIRECTIONS_CACHE_SHELF_LIFE", 7*24*time.Hour),
		CtpCjRateLimit:            getDuration("CTP_CJ_RATE_LIMIT", time.Second),
		TranzyRateLimit:           getDuration("TRANZY_RATE_LIMIT", 200*time.Millisecond),
		TranzyVehiclesDailyQuota:  getInt("TRANZY_VEHICLES_DAILY_QUOTA", 4500),
		TranzyDefaultDailyQuota:   getInt("TRANZY_DEFAULT_DAILY_QUOTA", 500),
		VehicleSchedule:           getEnv("VEHICLE_SCHEDULE", "00:00-06:00;30s;60s@20, 06:00-07:00;20s, 07:00-09:00;10s, 09:00-16:00;20s, 16:00-18:30;10s, 18:30-22:00;20s, 22:00-24:00;30s;60s@20"),
		VehicleScheduleWeekend:    getEnv("VEHICLE_SCHEDULE_WEEKEND", "00:00-06:00;30s;60s@20, 06:00-22:00;20s, 22:00-24:00;30s;60s@20"),
		VehicleMinInterval:        getDuration("VEHICLE_MIN_INTERVAL", 5*time.Second),
		VehicleMaxInterval:        getDuration("VEHICLE_MAX_INTERVAL", 60*time.Second),
		ORSApiKey:                 getEnv("OPEN_ROUTE_SERVICE_API_KEY", ""),
		ORSBaseURL:                getEnv("ORS_BASE_URL", "https://api.openrouteservice.org"),
		ORSDailyQuota:             getInt("ORS_DAILY_QUOTA", 2000),
		ORSMinuteQuota:            getInt("ORS_MINUTE_QUOTA", 40),
	}
}
