package config

import (
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// Config holds all application configuration.
type Config struct {
	// Logging
	LogLevel string

	// Storage
	StorePath string

	// Device
	DeviceName string

	// Sync
	SyncOnConnect    bool
	SyncInterval     time.Duration
	HistorySyncPages int
}

// Default returns a Config with sensible defaults.
func Default() *Config {
	homeDir, _ := os.UserHomeDir()
	defaultStore := filepath.Join(homeDir, ".orion-agent", "store")

	return &Config{
		LogLevel:         "INFO",
		StorePath:        defaultStore,
		DeviceName:       "Orion Agent",
		SyncOnConnect:    true,
		SyncInterval:     30 * time.Minute,
		HistorySyncPages: 10,
	}
}

// Load loads configuration from environment variables with defaults.
func Load() *Config {
	cfg := Default()

	if v := os.Getenv("ORION_LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
	}
	if v := os.Getenv("ORION_STORE_PATH"); v != "" {
		cfg.StorePath = v
	}
	if v := os.Getenv("ORION_DEVICE_NAME"); v != "" {
		cfg.DeviceName = v
	}
	if v := os.Getenv("ORION_SYNC_ON_CONNECT"); v != "" {
		cfg.SyncOnConnect = v == "true" || v == "1"
	}
	if v := os.Getenv("ORION_SYNC_INTERVAL"); v != "" {
		if mins, err := strconv.Atoi(v); err == nil {
			cfg.SyncInterval = time.Duration(mins) * time.Minute
		}
	}
	if v := os.Getenv("ORION_HISTORY_SYNC_PAGES"); v != "" {
		if pages, err := strconv.Atoi(v); err == nil {
			cfg.HistorySyncPages = pages
		}
	}

	return cfg
}

// EnsureStorePath creates the store directory if it doesn't exist.
func (c *Config) EnsureStorePath() error {
	return os.MkdirAll(filepath.Dir(c.StorePath), 0755)
}
