package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	ServerPort  string
	DatabaseURL string
	AppEnv      string
	FrontendURL string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		ServerPort:  getEnv("SERVER_PORT", "8000"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		AppEnv:      getEnv("APP_ENV", "development"),
		FrontendURL: getEnv("FRONTEND_URL", "http://localhost:3000"),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
