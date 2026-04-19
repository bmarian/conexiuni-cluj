package main

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Environment                 string
	LogFilePath                 string
	TranzyBaseUrl               string
	ClujAgencyId                string
	CtpCsvBaseUrl               string
	Port                        string
	DatabasePath                string
	TranzyApiKey                string
	ShapeCacheShelfLife         time.Duration
	RouteCacheShelfLife         time.Duration
	TripCacheShelfLife          time.Duration
	StopCacheShelfLife          time.Duration
	TimetableCacheShelfLife     time.Duration
	StopTimeCacheShelfLife      time.Duration
	APIStopTimeCacheShelfLife   time.Duration
	CtpCjRateLimit              time.Duration
	TranzyRateLimit             time.Duration
	StopInfoCacheShelfLife      time.Duration
	TranzyVehiclesDailyQuota    int
	TranzyDefaultDailyQuota     int
	VehicleSubscribersThreshold int
	VehicleBaselineInterval     time.Duration
	VehicleBusyInterval         time.Duration
	VehicleReserveInterval      time.Duration
	VehicleRushMorningStart     int
	VehicleRushMorningEnd       int
	VehicleRushEveningStart     int
	VehicleRushEveningEnd       int
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
	err := godotenv.Load("../.env", "../keys.env")
	if err != nil {
		log.Fatal("No .env files found, using environment variables")
	}

	return &Config{
		Environment:                 getEnv("ENV", "development"),
		LogFilePath:                 getEnv("LOG_FILE_PATH", "../conexiuni-cluj.log"),
		TranzyBaseUrl:               getEnv("TRANZY_BASE_URL", "https://api.tranzy.ai/v1/opendata"),
		ClujAgencyId:                getEnv("CLUJ_AGENCY_ID", "2"),
		CtpCsvBaseUrl:               getEnv("CTP_CSV_BASE_URL", "https://ctpcj.ro/orare/csv"),
		Port:                        getEnv("PORT", "6698"),
		DatabasePath:                getEnv("DATABASE_PATH", "../conexiuni-cluj.db"),
		TranzyApiKey:                getEnv("TRANZY_API_KEY", ""),
		ShapeCacheShelfLife:         getDuration("SHAPE_CACHE_SHELF_LIFE", 7*24*time.Hour),
		RouteCacheShelfLife:         getDuration("ROUTE_CACHE_SHELF_LIFE", 7*24*time.Hour),
		TripCacheShelfLife:          getDuration("TRIP_CACHE_SHELF_LIFE", 7*24*time.Hour),
		StopCacheShelfLife:          getDuration("STOP_CACHE_SHELF_LIFE", 7*24*time.Hour),
		TimetableCacheShelfLife:     getDuration("TIMETABLE_CACHE_SHELF_LIFE", 7*24*time.Hour),
		StopTimeCacheShelfLife:      getDuration("STOP_TIME_CACHE_SHELF_LIFE", 7*24*time.Hour),
		APIStopTimeCacheShelfLife:   getDuration("API_STOP_TIME_CACHE_SHELF_LIFE", 7*24*time.Hour),
		StopInfoCacheShelfLife:      getDuration("STOP_INFO_CACHE_SHELF_LIFE", 7*24*time.Hour),
		CtpCjRateLimit:              getDuration("CTP_CJ_RATE_LIMIT", time.Second),
		TranzyRateLimit:             getDuration("TRANZY_RATE_LIMIT", 200*time.Millisecond),
		TranzyVehiclesDailyQuota:    getInt("TRANZY_VEHICLES_DAILY_QUOTA", 4500),
		TranzyDefaultDailyQuota:     getInt("TRANZY_DEFAULT_DAILY_QUOTA", 500),
		VehicleSubscribersThreshold: getInt("VEHICLE_SUBSCRIBERS_THRESHOLD", 20),
		VehicleBaselineInterval:     getDuration("VEHICLE_BASELINE_INTERVAL", 20*time.Second),
		VehicleBusyInterval:         getDuration("VEHICLE_BUSY_INTERVAL", 5*time.Second),
		VehicleReserveInterval:      getDuration("VEHICLE_RESERVE_INTERVAL", 60*time.Second),
		VehicleRushMorningStart:     getInt("VEHICLE_RUSH_MORNING_START", 7),
		VehicleRushMorningEnd:       getInt("VEHICLE_RUSH_MORNING_END", 9),
		VehicleRushEveningStart:     getInt("VEHICLE_RUSH_EVENING_START", 16),
		VehicleRushEveningEnd:       getInt("VEHICLE_RUSH_EVENING_END", 19),
	}
}
