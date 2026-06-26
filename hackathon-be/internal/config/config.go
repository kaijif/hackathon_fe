// Package config loads runtime configuration from the environment.
package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds runtime configuration for the server.
type Config struct {
	HTTPAddr          string
	DatabaseURL       string
	MonitorEnabled    bool
	MonitorTick       time.Duration
	DefaultLowBattery int
	LogLevel          string

	APNs APNsConfig
}

// APNsConfig holds Apple Push Notification service configuration.
type APNsConfig struct {
	Enabled    bool
	KeyPath    string // path to the .p8 auth key
	KeyID      string // Apple Key ID (kid)
	TeamID     string // Apple Team ID (iss)
	Topic      string // app bundle ID (apns-topic)
	Production bool   // true = api.push.apple.com, false = sandbox
}

// Load reads configuration from environment variables, applying defaults.
func Load() Config {
	return Config{
		HTTPAddr:          env("HTTP_ADDR", ":8080"),
		DatabaseURL:       env("DATABASE_URL", "postgres://nightwatch:nightwatch@localhost:5432/nightwatch?sslmode=disable"),
		MonitorEnabled:    envBool("MONITOR_ENABLED", true),
		MonitorTick:       envDuration("MONITOR_TICK", 30*time.Second),
		DefaultLowBattery: envInt("DEFAULT_LOW_BATTERY_THRESHOLD", 20),
		LogLevel:          env("LOG_LEVEL", "info"),
		APNs: APNsConfig{
			Enabled:    envBool("APNS_ENABLED", false),
			KeyPath:    env("APNS_KEY_PATH", ""),
			KeyID:      env("APNS_KEY_ID", ""),
			TeamID:     env("APNS_TEAM_ID", ""),
			Topic:      env("APNS_TOPIC", ""),
			Production: envBool("APNS_PRODUCTION", false),
		},
	}
}

func env(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func envBool(key string, def bool) bool {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}

func envInt(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}
