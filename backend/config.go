package main

import (
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Environment           string
	TranzyBaseUrl         string
	ClujAgencyId          string
	CtpCsvBaseUrl         string
	Port                  string
	DatabasePath          string
	TranzyApiKey          string
	VehicleCacheShelfLife time.Duration
	ShapeCacheShelfLife   time.Duration
	RouteCacheShelfLife   time.Duration
	TripCacheShelfLife    time.Duration
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value != "" {
		return value
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
		Environment:           getEnv("ENV", "development"),
		TranzyBaseUrl:         getEnv("TRANZY_BASE_URL", "https://api.tranzy.ai/v1/opendata"),
		ClujAgencyId:          getEnv("CLUJ_AGENCY_ID", "2"),
		CtpCsvBaseUrl:         getEnv("CTP_CSV_BASE_URL", "https://ctpcj.ro/orare/csv"),
		Port:                  getEnv("PORT", "6698"),
		DatabasePath:          getEnv("DATABASE_PATH", "../conexiuni-cluj.db"),
		TranzyApiKey:          getEnv("TRANZY_API_KEY", ""),
		VehicleCacheShelfLife: getDuration("VEHICLE_CACHE_SHELF_LIFE", 30*time.Second),
		ShapeCacheShelfLife:   getDuration("SHAPE_CACHE_SHELF_LIFE", 24*time.Hour),
		RouteCacheShelfLife:   getDuration("ROUTE_CACHE_SHELF_LIFE", 24*time.Hour),
		TripCacheShelfLife:    getDuration("TRIP_CACHE_SHELF_LIFE", 24*time.Hour),
	}
}
