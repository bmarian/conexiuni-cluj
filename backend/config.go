package main

import (
	"log"
	"math"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Environment              string
	LogDir                   string
	LogRetentionDays         int
	LogIPHashSalt            string
	AdminToken               string
	TranzyBaseUrl            string
	ClujAgencyId             string
	CtpCsvBaseUrl            string
	Port                     string
	DatabasePath             string
	TranzyApiKey             string
	TranzyLearningApiKey     string
	TranzyCacheShelfLife     time.Duration
	TimetableCacheShelfLife  time.Duration
	NewsCacheShelfLife       time.Duration
	CtpCjRateLimit           time.Duration
	TranzyRateLimit          time.Duration
	TranzyVehiclesDailyQuota int
	TranzyDefaultDailyQuota  int
	VehicleSchedule          string
	VehicleScheduleWeekend   string
	VehicleMinInterval       time.Duration
	VehicleMaxInterval       time.Duration
	VehicleLearningEnabled   bool
	VehicleLearningMaxQuota  int
	OtpMaxMemory             string
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

func getBool(key string, defaultValue bool) bool {
	value := os.Getenv(key)
	if value != "" {
		b, err := strconv.ParseBool(value)
		if err != nil {
			log.Printf("Invalid %s, using default %t: %v", key, defaultValue, err)
			return defaultValue
		}
		return b
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

	tranzyDefaultDailyQuota := getInt("TRANZY_DEFAULT_DAILY_QUOTA", 144)
	tranzyVehiclesDailyQuota := getInt("TRANZY_VEHICLES_DAILY_QUOTA", 4500)
	// 6 distinct Tranzy endpoint caches share the quota equally
	tranzyCacheShelfLife := 24 * time.Hour * 6 / time.Duration(tranzyDefaultDailyQuota)

	cfg := &Config{
		Environment:              getEnv("ENV", "development"),
		LogDir:                   getEnv("LOG_DIR", "../logs"),
		LogRetentionDays:         getInt("LOG_RETENTION_DAYS", 5),
		LogIPHashSalt:            getEnv("LOG_IP_HASH_SALT", ""),
		AdminToken:               getEnv("ADMIN_TOKEN", ""),
		TranzyBaseUrl:            getEnv("TRANZY_BASE_URL", "https://api.tranzy.ai/v1/opendata"),
		ClujAgencyId:             getEnv("CLUJ_AGENCY_ID", "2"),
		CtpCsvBaseUrl:            getEnv("CTP_CSV_BASE_URL", "https://ctpcj.ro/orare/csv"),
		Port:                     getEnv("PORT", "6698"),
		DatabasePath:             getEnv("DATABASE_PATH", "../conexiuni-cluj.db"),
		TranzyApiKey:             getEnv("TRANZY_API_KEY", ""),
		TranzyLearningApiKey:     getEnv("TRANZY_API_KEY_LEARNING", ""),
		TranzyCacheShelfLife:     tranzyCacheShelfLife,
		TimetableCacheShelfLife:  getDuration("TIMETABLE_CACHE_SHELF_LIFE", 24*time.Hour),
		NewsCacheShelfLife:       getDuration("NEWS_CACHE_SHELF_LIFE", 4*time.Hour),
		CtpCjRateLimit:           getDuration("CTP_CJ_RATE_LIMIT", time.Second),
		TranzyRateLimit:          getDuration("TRANZY_RATE_LIMIT", 200*time.Millisecond),
		TranzyVehiclesDailyQuota: tranzyVehiclesDailyQuota,
		TranzyDefaultDailyQuota:  tranzyDefaultDailyQuota,
		VehicleSchedule:          getEnv("VEHICLE_SCHEDULE", "00:00-06:00;30s;60s@20, 06:00-07:00;20s, 07:00-09:00;10s, 09:00-16:00;20s, 16:00-18:30;10s, 18:30-22:00;20s, 22:00-24:00;30s;60s@20"),
		VehicleScheduleWeekend:   getEnv("VEHICLE_SCHEDULE_WEEKEND", "00:00-06:00;30s;60s@20, 06:00-22:00;20s, 22:00-24:00;30s;60s@20"),
		VehicleMinInterval:       getDuration("VEHICLE_MIN_INTERVAL", 5*time.Second),
		VehicleMaxInterval:       getDuration("VEHICLE_MAX_INTERVAL", 60*time.Second),
		VehicleLearningEnabled:   getBool("VEHICLE_LEARNING_ENABLED", true),
		VehicleLearningMaxQuota:  getInt("VEHICLE_LEARNING_DAILY_QUOTA_MAX", 3000),
		OtpMaxMemory:             getEnv("OTP_MX", "2G"),
	}

	if cfg.Environment == "development" {
		neverExpire := time.Duration(math.MaxInt64)
		cfg.TranzyCacheShelfLife = neverExpire
		cfg.TimetableCacheShelfLife = neverExpire
	}

	return cfg
}
