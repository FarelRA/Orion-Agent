package store

import (
	"encoding/json"
	"time"

	"orion-agent/internal/infra/config"
)

// SettingsStore handles AI settings operations.
type SettingsStore struct {
	store  *Store
	config *config.Config
}

// NewSettingsStore creates a new SettingsStore.
func NewSettingsStore(s *Store, cfg *config.Config) *SettingsStore {
	return &SettingsStore{store: s, config: cfg}
}

// Get retrieves a setting value.
func (s *SettingsStore) Get(key string) (string, error) {
	var value string
	err := s.store.QueryRow(`SELECT value FROM orion_settings WHERE key = ?`, key).Scan(&value)
	return value, err
}

// Set stores a setting value.
func (s *SettingsStore) Set(key, value string) error {
	now := time.Now().Unix()
	_, err := s.store.Exec(`
		INSERT INTO orion_settings (key, value, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		key, value, now,
	)
	return err
}

// GetWithDefault retrieves a setting with a default fallback.
func (s *SettingsStore) GetWithDefault(key, defaultVal string) string {
	val, err := s.Get(key)
	if err != nil || val == "" {
		return defaultVal
	}
	return val
}

// GetBool retrieves a boolean setting.
func (s *SettingsStore) GetBool(key string, defaultVal bool) bool {
	val, err := s.Get(key)
	if err != nil {
		return defaultVal
	}
	return val == "true" || val == "1"
}

// SetBool stores a boolean setting.
func (s *SettingsStore) SetBool(key string, value bool) error {
	v := "false"
	if value {
		v = "true"
	}
	return s.Set(key, v)
}

// GetStringSlice retrieves a JSON string slice setting.
func (s *SettingsStore) GetStringSlice(key string, defaultVal []string) []string {
	val, err := s.Get(key)
	if err != nil || val == "" {
		return defaultVal
	}
	var result []string
	if json.Unmarshal([]byte(val), &result) != nil {
		return defaultVal
	}
	return result
}

// SetStringSlice stores a string slice as JSON.
func (s *SettingsStore) SetStringSlice(key string, value []string) error {
	data, _ := json.Marshal(value)
	return s.Set(key, string(data))
}

// AI-specific helpers

// GetAIEnabled returns whether AI is enabled.
func (s *SettingsStore) GetAIEnabled() bool {
	return s.GetBool("ai.enabled", s.config.AI.Enabled)
}

// SetAIEnabled sets whether AI is enabled.
func (s *SettingsStore) SetAIEnabled(enabled bool) error {
	return s.SetBool("ai.enabled", enabled)
}

// GetSystemPrompt returns the system prompt.
func (s *SettingsStore) GetSystemPrompt(chatJID string) string {
	// Check per-chat override first
	if chatJID != "" {
		if val, err := s.Get("ai.system_prompt." + chatJID); err == nil && val != "" {
			return val
		}
	}
	return s.GetWithDefault("ai.system_prompt", s.config.AI.SystemPrompt)
}

// SetSystemPrompt sets the system prompt (global or per-chat).
func (s *SettingsStore) SetSystemPrompt(prompt string, chatJID string) error {
	key := "ai.system_prompt"
	if chatJID != "" {
		key = "ai.system_prompt." + chatJID
	}
	return s.Set(key, prompt)
}

// GetCommandPrefix returns the command prefix.
func (s *SettingsStore) GetCommandPrefix() string {
	return s.GetWithDefault("ai.command_prefix", s.config.AI.CommandPrefix)
}

// GetWhitelist returns the whitelist of JIDs.
func (s *SettingsStore) GetWhitelist() []string {
	return s.GetStringSlice("ai.whitelist", s.config.AI.Whitelist)
}

// SetWhitelist sets the whitelist.
func (s *SettingsStore) SetWhitelist(jids []string) error {
	return s.SetStringSlice("ai.whitelist", jids)
}

// GetBlacklist returns the blacklist of JIDs.
func (s *SettingsStore) GetBlacklist() []string {
	return s.GetStringSlice("ai.blacklist", s.config.AI.Blacklist)
}

// SetBlacklist sets the blacklist.
func (s *SettingsStore) SetBlacklist(jids []string) error {
	return s.SetStringSlice("ai.blacklist", jids)
}

// GetTriggerWords returns the trigger words.
func (s *SettingsStore) GetTriggerWords() []string {
	return s.GetStringSlice("ai.trigger_words", s.config.AI.Triggers.TriggerWords)
}

// SetTriggerWords sets the trigger words.
func (s *SettingsStore) SetTriggerWords(words []string) error {
	return s.SetStringSlice("ai.trigger_words", words)
}

// GetAdmins returns the list of admin JIDs.
func (s *SettingsStore) GetAdmins() []string {
	return s.GetStringSlice("ai.admins", s.config.AI.Admins)
}

// SetAdmins sets the admin list.
func (s *SettingsStore) SetAdmins(jids []string) error {
	return s.SetStringSlice("ai.admins", jids)
}

// IsAdmin checks if a JID is an admin.
func (s *SettingsStore) IsAdmin(jid string) bool {
	admins := s.GetAdmins()
	for _, admin := range admins {
		if admin == jid {
			return true
		}
	}
	return false
}

// GetDMAutoRespond returns whether to auto-respond in DMs.
func (s *SettingsStore) GetDMAutoRespond() bool {
	return s.GetBool("ai.dm_auto_respond", s.config.AI.Triggers.DMAutoRespond)
}

// GetGroupAutoRespond returns whether to respond in groups.
func (s *SettingsStore) GetGroupAutoRespond() bool {
	return s.GetBool("ai.group_auto_respond", s.config.AI.Triggers.GroupAutoRespond)
}

// GetMentionToMe returns whether to respond when someone mentions the agent.
func (s *SettingsStore) GetMentionToMe() bool {
	return s.GetBool("ai.mention_to_me", s.config.AI.Triggers.MentionToMe)
}

// GetReplyToMe returns whether to respond when someone replies to agent's message.
func (s *SettingsStore) GetReplyToMe() bool {
	return s.GetBool("ai.reply_to_me", s.config.AI.Triggers.ReplyToMe)
}
