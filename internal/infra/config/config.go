package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// Config holds all application configuration.
type Config struct {
	// Logging
	LogLevel string `json:"log_level"`

	// Storage
	StorePath string `json:"store_path"`

	// Device
	DeviceName string `json:"device_name"`

	// Sync
	SyncOnConnect    bool          `json:"sync_on_connect"`
	SyncInterval     time.Duration `json:"-"`
	SyncIntervalMins int           `json:"sync_interval_mins"`
	HistorySyncPages int           `json:"history_sync_pages"`

	// AI Configuration
	AI AIConfig `json:"ai"`
}

// AIConfig holds AI/LLM configuration.
type AIConfig struct {
	Enabled       bool          `json:"enabled"`
	Models        []ModelConfig `json:"models"`
	DefaultModel  string        `json:"default_model"`
	AgentName     string        `json:"agent_name"`
	SystemPrompt  string        `json:"system_prompt"`
	CommandPrefix string        `json:"command_prefix"`
	MaxMessageAge int           `json:"max_message_age"` // Max age in seconds for messages to process (0 = no limit)

	// Default trigger settings
	Triggers TriggerConfig `json:"triggers"`
}

// ModelConfig defines an LLM model configuration.
type ModelConfig struct {
	Name       string `json:"name"`
	APIKey     string `json:"api_key"`
	BaseURL    string `json:"base_url"`
	Model      string `json:"model"`
	MaxContext int    `json:"max_context"`
}

// TriggerConfig defines default trigger behavior.
type TriggerConfig struct {
	DMAutoRespond    bool     `json:"dm_auto_respond"`
	GroupMentionOnly bool     `json:"group_mention_only"`
	TriggerWords     []string `json:"trigger_words"`
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
		SyncIntervalMins: 30,
		HistorySyncPages: 10,
		AI: AIConfig{
			Enabled:       false,
			AgentName:     "Orion",
			CommandPrefix: "/",
			SystemPrompt:  "You are a helpful AI assistant.",
			MaxMessageAge: 60, // Default 60 seconds
			Triggers: TriggerConfig{
				DMAutoRespond:    true,
				GroupMentionOnly: true,
				TriggerWords:     []string{},
			},
		},
	}
}

// LoadFromFile loads configuration from a JSON file.
func LoadFromFile(path string) (*Config, error) {
	cfg := Default()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil // Return defaults if file doesn't exist
		}
		return nil, err
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	// Convert minutes to duration
	if cfg.SyncIntervalMins > 0 {
		cfg.SyncInterval = time.Duration(cfg.SyncIntervalMins) * time.Minute
	}

	return cfg, nil
}

// Load loads configuration from environment variables with defaults.
// If configPath is provided, loads from file first.
func Load(configPath string) *Config {
	var cfg *Config
	var err error

	if configPath != "" {
		cfg, err = LoadFromFile(configPath)
		if err != nil {
			cfg = Default()
		}
	} else {
		cfg = Default()
	}

	// Environment variable overrides
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
	if v := os.Getenv("ORION_AI_ENABLED"); v != "" {
		cfg.AI.Enabled = v == "true" || v == "1"
	}

	return cfg
}

// GetModel returns the model config by name, or the default model.
func (c *Config) GetModel(name string) *ModelConfig {
	if name == "" {
		name = c.AI.DefaultModel
	}
	for i := range c.AI.Models {
		if c.AI.Models[i].Name == name {
			return &c.AI.Models[i]
		}
	}
	if len(c.AI.Models) > 0 {
		return &c.AI.Models[0]
	}
	return nil
}

// EnsureStorePath creates the store directory if it doesn't exist.
func (c *Config) EnsureStorePath() error {
	return os.MkdirAll(c.StorePath, 0755)
}
