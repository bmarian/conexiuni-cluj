package main

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Environment   string
	TranzyBaseUrl string
	ClujAgencyId  string
	CtpCsvBaseUrl string
	Port          string
	DatabasePath  string
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value != "" {
		return value
	}
	return defaultValue
}

func Load() *Config {
	err := godotenv.Load("../.env", "../keys.env")
	if err != nil {
		log.Fatal("No .env files found, using environment variables")
	}

	return &Config{
		Environment:   getEnv("ENV", "development"),
		TranzyBaseUrl: getEnv("TRANZY_BASE_URL", "https://api.tranzy.ai/v1/opendata"),
		ClujAgencyId:  getEnv("CLUJ_AGENCY_ID", "2"),
		CtpCsvBaseUrl: getEnv("CTP_CSV_BASE_URL", "https://ctpcj.ro/orare/csv"),
		Port:          getEnv("PORT", "6698"),
		DatabasePath:  getEnv("DATABASE_PATH", "../conexiuni-cluj.db"),
	}
}
