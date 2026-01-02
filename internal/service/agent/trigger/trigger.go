package trigger

import (
	"strings"

	"go.mau.fi/whatsmeow/types"

	"orion-agent/internal/data/store"
)

// Trigger evaluates whether the agent should respond to a message.
type Trigger struct {
	settings *store.SettingsStore
	ownJID   types.JID
}

// NewTrigger creates a new trigger evaluator.
func NewTrigger(settings *store.SettingsStore, ownJID types.JID) *Trigger {
	return &Trigger{
		settings: settings,
		ownJID:   ownJID,
	}
}

// SetOwnJID updates the own JID (for delayed initialization).
func (t *Trigger) SetOwnJID(jid types.JID) {
	t.ownJID = jid
}

// Result contains the trigger evaluation result.
type Result struct {
	ShouldRespond bool
	Reason        string
}

// ShouldRespond evaluates if the agent should respond to a message.
func (t *Trigger) ShouldRespond(chatJID, senderJID types.JID, messageText string, mentionedJIDs []string, fromMe bool) Result {
	// Never respond to own messages
	if fromMe {
		return Result{false, "own message"}
	}

	// Check if AI is enabled
	if !t.settings.GetAIEnabled() {
		return Result{false, "AI disabled"}
	}

	chatStr := chatJID.String()
	senderStr := senderJID.String()

	// Check blacklist first
	blacklist := t.settings.GetBlacklist()
	for _, blocked := range blacklist {
		if blocked == chatStr || blocked == senderStr {
			return Result{false, "blacklisted"}
		}
	}

	// Check whitelist (if set, only respond to whitelisted)
	whitelist := t.settings.GetWhitelist()
	if len(whitelist) > 0 {
		found := false
		for _, allowed := range whitelist {
			if allowed == chatStr || allowed == senderStr {
				found = true
				break
			}
		}
		if !found {
			return Result{false, "not whitelisted"}
		}
	}

	// Determine chat type
	isGroup := chatJID.Server == types.GroupServer
	isNewsletter := chatJID.Server == types.NewsletterServer

	// Never respond in newsletters
	if isNewsletter {
		return Result{false, "newsletter"}
	}

	// DM: auto-respond if enabled
	if !isGroup {
		if t.settings.GetDMAutoRespond() {
			return Result{true, "DM auto-respond"}
		}
		return Result{false, "DM auto-respond disabled"}
	}

	// Group: check mention or trigger words
	if t.settings.GetGroupMentionOnly() {
		// Check if mentioned
		if t.isMentioned(mentionedJIDs) {
			return Result{true, "mentioned"}
		}

		// Check trigger words
		if t.hasTriggerWord(messageText) {
			return Result{true, "trigger word"}
		}

		return Result{false, "not mentioned or triggered"}
	}

	// Group without mention-only: respond to all
	return Result{true, "group auto-respond"}
}

// isMentioned checks if the bot is mentioned.
func (t *Trigger) isMentioned(mentionedJIDs []string) bool {
	if t.ownJID.IsEmpty() {
		return false
	}
	ownStr := t.ownJID.String()
	ownUser := t.ownJID.User

	for _, jid := range mentionedJIDs {
		if jid == ownStr {
			return true
		}
		// Also check just the user part
		parsed, err := types.ParseJID(jid)
		if err == nil && parsed.User == ownUser {
			return true
		}
	}
	return false
}

// hasTriggerWord checks if the message contains any trigger word.
func (t *Trigger) hasTriggerWord(text string) bool {
	triggerWords := t.settings.GetTriggerWords()
	if len(triggerWords) == 0 {
		return false
	}

	textLower := strings.ToLower(text)
	for _, word := range triggerWords {
		if strings.Contains(textLower, strings.ToLower(word)) {
			return true
		}
	}
	return false
}

// IsCommand checks if a message is a command.
func (t *Trigger) IsCommand(text string) bool {
	prefix := t.settings.GetCommandPrefix()
	return strings.HasPrefix(strings.TrimSpace(text), prefix)
}

// ParseCommand extracts command name and args from a command message.
func (t *Trigger) ParseCommand(text string) (cmd string, args []string) {
	prefix := t.settings.GetCommandPrefix()
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, prefix) {
		return "", nil
	}

	text = strings.TrimPrefix(text, prefix)
	parts := strings.Fields(text)
	if len(parts) == 0 {
		return "", nil
	}

	return parts[0], parts[1:]
}
