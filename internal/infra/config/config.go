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

	// Media
	Media MediaConfig `json:"media"`

	// AI Configuration
	AI AIConfig `json:"ai"`
}

// MediaConfig holds media download settings.
type MediaConfig struct {
	AutoDownload  bool     `json:"auto_download"`    // Master switch for auto-download
	Types         []string `json:"types"`            // Media types to download: image, video, audio, document, sticker, profile_picture, view_once
	MaxFileSizeMB int      `json:"max_file_size_mb"` // Skip files larger than this (0 = no limit)
	WorkerCount   int      `json:"worker_count"`     // Number of concurrent download workers

	// Advanced Settings
	DownloadTimeoutMs     int  `json:"download_timeout_ms"`      // Timeout for downloads in ms (default 60000)
	RetryMaxAttempts      int  `json:"retry_max_attempts"`       // Max retries for failed downloads (default 3)
	RetryInitialBackoffMs int  `json:"retry_initial_backoff_ms"` // Initial backoff in ms (default 500)
	RetryMaxBackoffMs     int  `json:"retry_max_backoff_ms"`     // Max backoff in ms (default 30000)
	HistorySyncDownload   bool `json:"history_sync_download"`    // Download media from history sync
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

	// Security rules
	Admins    []string `json:"admins"`
	Whitelist []string `json:"whitelist"`
	Blacklist []string `json:"blacklist"`
}

// ModelConfig defines an LLM model configuration.
type ModelConfig struct {
	Name string `json:"name"`
	// Client settings
	APIKey  string `json:"api_key"`
	BaseURL string `json:"base_url"`

	// Model settings
	Model               string            `json:"model"`
	MaxContext          int               `json:"max_context"`           // Legacy explicit context limit
	MaxTokens           int               `json:"max_tokens"`            // Deprecated
	MaxCompletionTokens int               `json:"max_completion_tokens"` // Recommended
	Temperature         float64           `json:"temperature,omitempty"`
	TopP                float64           `json:"top_p,omitempty"`
	FrequencyPenalty    float64           `json:"frequency_penalty,omitempty"`
	PresencePenalty     float64           `json:"presence_penalty,omitempty"`
	N                   int               `json:"n,omitempty"`
	Stop                []string          `json:"stop,omitempty"`
	Seed                int               `json:"seed,omitempty"`
	LogitBias           map[string]int    `json:"logit_bias,omitempty"`
	Logprobs            bool              `json:"logprobs,omitempty"`
	TopLogProbs         int               `json:"top_logprobs,omitempty"`
	ParallelToolCalls   bool              `json:"parallel_tool_calls,omitempty"`
	ResponseFormat      string            `json:"response_format,omitempty"` // "text", "json_object"
	ReasoningEffort     string            `json:"reasoning_effort,omitempty"`
	ServiceTier         string            `json:"service_tier,omitempty"`
	User                string            `json:"user,omitempty"`
	Metadata            map[string]string `json:"metadata,omitempty"`
	Modalities          []string          `json:"modalities,omitempty"`
	PromptCacheKey      string            `json:"prompt_cache_key,omitempty"`
	SafetyIdentifier    string            `json:"safety_identifier,omitempty"`
	Store               bool              `json:"store,omitempty"`

	// Nested configs
	Audio            *AudioConfig            `json:"audio,omitempty"`
	Prediction       *PredictionConfig       `json:"prediction,omitempty"`
	StreamOptions    *StreamOptionsConfig    `json:"stream_options,omitempty"`
	WebSearchOptions *WebSearchOptionsConfig `json:"web_search_options,omitempty"`
}

type AudioConfig struct {
	Voice  string `json:"voice"`
	Format string `json:"format"`
}

type PredictionConfig struct {
	Type    string `json:"type"`
	Content string `json:"content"`
}

type StreamOptionsConfig struct {
	IncludeUsage bool `json:"include_usage"`
}

type WebSearchOptionsConfig struct {
	Limit             int    `json:"limit,omitempty"`
	SearchContextSize string `json:"search_context_size,omitempty"`
}

// TriggerConfig defines default trigger behavior.
type TriggerConfig struct {
	DMAutoRespond    bool     `json:"dm_auto_respond"`
	GroupAutoRespond bool     `json:"group_auto_respond"`
	MentionToMe      bool     `json:"mention_to_me"`
	ReplyToMe        bool     `json:"reply_to_me"`
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
		Media: MediaConfig{
			AutoDownload:          false, // Disabled by default
			Types:                 []string{"image", "video", "audio", "document", "sticker", "profile_picture"},
			MaxFileSizeMB:         100,
			WorkerCount:           3,
			DownloadTimeoutMs:     60000,
			RetryMaxAttempts:      3,
			RetryInitialBackoffMs: 500,
			RetryMaxBackoffMs:     30000,
			HistorySyncDownload:   true,
		},
		AI: AIConfig{
			Enabled:       false,
			AgentName:     "Orion",
			CommandPrefix: "/",
			SystemPrompt:  "You are a helpful AI assistant.",
			MaxMessageAge: 60, // Default 60 seconds
			Triggers: TriggerConfig{
				DMAutoRespond:    true,
				GroupAutoRespond: true,
				ReplyToMe:        true,
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
