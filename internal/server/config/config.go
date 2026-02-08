package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port            string
	DatabaseURL     string
	StoragePath     string
	MaxFileSize     int64
	DefaultExpiry   time.Duration
	BaseURL         string
	CleanupInterval time.Duration
	RateLimitRPS    float64
	RateLimitBurst  int
}

func Load() *Config {
	return &Config{
		Port:            getEnv("PORT", "8080"),
		DatabaseURL:     getEnv("DATABASE_URL", "postgres://beam:beam@localhost:5432/beam?sslmode=disable"),
		StoragePath:     getEnv("STORAGE_PATH", "./storage/files"),
		MaxFileSize:     getEnvInt64("MAX_FILE_SIZE", 5*1024*1024*1024), // 5GB
		DefaultExpiry:   getEnvDuration("DEFAULT_EXPIRY_HOURS", 7*24*time.Hour),
		BaseURL:         getEnv("BASE_URL", "http://localhost:8080"),
		CleanupInterval: getEnvDuration("CLEANUP_INTERVAL_HOURS", 1*time.Hour),
		RateLimitRPS:    getEnvFloat64("RATE_LIMIT_RPS", 10),
		RateLimitBurst:  getEnvInt("RATE_LIMIT_BURST", 20),
	}
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func getEnvInt64(key string, fallback int64) int64 {
	if val := os.Getenv(key); val != "" {
		if n, err := strconv.ParseInt(val, 10, 64); err == nil {
			return n
		}
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if val := os.Getenv(key); val != "" {
		if n, err := strconv.Atoi(val); err == nil {
			return n
		}
	}
	return fallback
}

func getEnvFloat64(key string, fallback float64) float64 {
	if val := os.Getenv(key); val != "" {
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f
		}
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if val := os.Getenv(key); val != "" {
		if hours, err := strconv.ParseFloat(val, 64); err == nil {
			return time.Duration(hours * float64(time.Hour))
		}
	}
	return fallback
}
