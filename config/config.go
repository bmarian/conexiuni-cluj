package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port         string
	DatabasePath string
	Environment  string
}

func Load() *Config {
	// Load .env file
	err := godotenv.Load(".env", "keys.env")
	if err != nil {
		log.Fatal("No .env files found, using environment variables")
	}

	return &Config{
		Port:         getEnv("PORT", "8080"),
		DatabasePath: getEnv("DATABASE_PATH", "./data/ctp.db"),
		Environment:  getEnv("ENV", "development"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
