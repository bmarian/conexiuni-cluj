// Package config reads the server settings from .env and keys.env.
package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Environment      string
	LogDir           string
	LogRetentionDays int
	LogIPHashSalt    string
	AdminToken       string
	Port             string
	DBDir            string
	ArchiveDir       string

	TranzyBaseUrl            string
	TranzyRateLimit          time.Duration
	TranzyVehiclesDailyQuota int
	TranzyDefaultDailyQuota  int

	CtpCsvBaseUrl    string
	CtpCjRateLimit   time.Duration
	TimetableRefresh time.Duration
	NewsRefresh      time.Duration

	VehicleSchedule         string
	VehicleScheduleWeekend  string
	VehicleMinInterval      time.Duration
	VehicleMaxInterval      time.Duration
	VehicleLearningEnabled  bool
	VehicleLearningMaxQuota int

	VapidPublicKey    string
	VapidPrivateKey   string
	VapidSubject      string
	AndroidPackage    string
	AndroidCertSHA256 []string
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

func Bool(key string, defaultValue bool) bool {
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

func getList(key string) []string {
	var out []string
	for _, part := range strings.Split(os.Getenv(key), ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
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
		Environment:      getEnv("ENV", "development"),
		LogDir:           getEnv("LOG_DIR", "../logs"),
		LogRetentionDays: getInt("LOG_RETENTION_DAYS", 5),
		LogIPHashSalt:    getEnv("LOG_IP_HASH_SALT", ""),
		AdminToken:       getEnv("ADMIN_TOKEN", ""),
		Port:             getEnv("PORT", "6698"),
		DBDir:            getEnv("DB_DIR", "../db"),
		ArchiveDir:       getEnv("ARCHIVE_DIR", "../archive"),

		TranzyBaseUrl:            getEnv("TRANZY_BASE_URL", "https://api.tranzy.ai/v1/opendata"),
		TranzyRateLimit:          getDuration("TRANZY_RATE_LIMIT", 200*time.Millisecond),
		TranzyVehiclesDailyQuota: getInt("TRANZY_VEHICLES_DAILY_QUOTA", 4500),
		TranzyDefaultDailyQuota:  getInt("TRANZY_DEFAULT_DAILY_QUOTA", 144),

		CtpCsvBaseUrl:    getEnv("CTP_CSV_BASE_URL", "https://ctpcj.ro/orare/csv"),
		CtpCjRateLimit:   getDuration("CTP_CJ_RATE_LIMIT", time.Second),
		TimetableRefresh: getDuration("TIMETABLE_REFRESH", 24*time.Hour),
		NewsRefresh:      getDuration("NEWS_REFRESH", time.Hour),

		VehicleSchedule:         getEnv("VEHICLE_SCHEDULE", "00:00-06:00;30s;60s@20, 06:00-07:00;20s, 07:00-09:00;10s, 09:00-16:00;20s, 16:00-18:30;10s, 18:30-22:00;20s, 22:00-24:00;30s;60s@20"),
		VehicleScheduleWeekend:  getEnv("VEHICLE_SCHEDULE_WEEKEND", "00:00-06:00;30s;60s@20, 06:00-22:00;20s, 22:00-24:00;30s;60s@20"),
		VehicleMinInterval:      getDuration("VEHICLE_MIN_INTERVAL", 5*time.Second),
		VehicleMaxInterval:      getDuration("VEHICLE_MAX_INTERVAL", 60*time.Second),
		VehicleLearningEnabled:  Bool("VEHICLE_LEARNING_ENABLED", true),
		VehicleLearningMaxQuota: getInt("VEHICLE_LEARNING_DAILY_QUOTA_MAX", 3000),

		VapidPublicKey:    getEnv("VAPID_PUBLIC_KEY", ""),
		VapidPrivateKey:   getEnv("VAPID_PRIVATE_KEY", ""),
		VapidSubject:      getEnv("VAPID_SUBJECT", ""),
		AndroidPackage:    getEnv("ANDROID_PACKAGE", "online.bmarian.bus"),
		AndroidCertSHA256: getList("ANDROID_CERT_SHA256"),
	}
}
