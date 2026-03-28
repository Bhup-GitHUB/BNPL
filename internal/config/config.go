package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port           string
	DatabaseURL    string
	RequestTimeout time.Duration
	BureauTimeout  time.Duration
	AccountTimeout time.Duration
	FraudTimeout   time.Duration
	FeatureLatency time.Duration
	BureauLatency  time.Duration
	AccountLatency time.Duration
	FraudLatency   time.Duration
	ApproveScore   int
	RejectScore    int
}

func Load() (Config, error) {
	cfg := Config{
		Port:           getEnv("APP_PORT", "9090"),
		DatabaseURL:    getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/bnpl?sslmode=disable"),
		RequestTimeout: getDurationMillis("REQUEST_TIMEOUT_MS", 4500),
		BureauTimeout:  getDurationMillis("BUREAU_TIMEOUT_MS", 700),
		AccountTimeout: getDurationMillis("AA_TIMEOUT_MS", 900),
		FraudTimeout:   getDurationMillis("FRAUD_TIMEOUT_MS", 600),
		FeatureLatency: getDurationMillis("FEATURE_LATENCY_MS", 40),
		BureauLatency:  getDurationMillis("BUREAU_LATENCY_MS", 200),
		AccountLatency: getDurationMillis("AA_LATENCY_MS", 260),
		FraudLatency:   getDurationMillis("FRAUD_LATENCY_MS", 120),
		ApproveScore:   getInt("APPROVE_SCORE", 72),
		RejectScore:    getInt("REJECT_SCORE", 48),
	}

	if cfg.Port == "" {
		return Config{}, fmt.Errorf("APP_PORT cannot be empty")
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL cannot be empty")
	}
	if cfg.RejectScore >= cfg.ApproveScore {
		return Config{}, fmt.Errorf("REJECT_SCORE must be lower than APPROVE_SCORE")
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

func getInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getDurationMillis(key string, fallback int) time.Duration {
	return time.Duration(getInt(key, fallback)) * time.Millisecond
}
