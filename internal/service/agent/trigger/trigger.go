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
func (t *Trigger) ShouldRespond(messageText string, chatJID, senderJID types.JID, mentionedJIDs []string, quotedSenderLID types.JID) Result {
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
	isDm := false
	switch chatJID.Server {
	case types.DefaultUserServer:
		isDm = true
	case types.HiddenUserServer:
		isDm = true
	}
	isGroup := chatJID.Server == types.GroupServer

	// Never respond if not DM or group
	if !isDm || !isGroup {
		return Result{false, "not DM or group"}
	}

	// DM: auto-respond if enabled
	if isDm {
		if t.settings.GetDMAutoRespond() {
			return Result{true, "DM auto-respond"}
		}
		return Result{false, "DM auto-respond disabled"}
	}

	// Group: auto-respond if enabled
	if isGroup {
		if t.settings.GetGroupAutoRespond() {
			// Check if mentioned
			if t.isMentioned(mentionedJIDs) {
				return Result{true, "mentioned"}
			}
			// Check reply-to-me trigger
			if t.isReplyToMe(quotedSenderLID) {
				return Result{true, "reply to me"}
			}
			// Check trigger words
			if t.hasTriggerWord(messageText) {
				return Result{true, "trigger word"}
			}
			return Result{false, "not mentioned or triggered"}
		}
		return Result{false, "group auto-respond disabled"}
	}

	return Result{true, "unhandled"}
}

// isMentioned checks if the bot is mentioned.
func (t *Trigger) isMentioned(mentionedJIDs []string) bool {
	if !t.settings.GetMentionToMe() {
		return false
	}
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

// isReplyToMe checks if the message is a reply to the agent's message.
func (t *Trigger) isReplyToMe(quotedSenderLID types.JID) bool {
	// Check if reply-to-me trigger is enabled
	if !t.settings.GetReplyToMe() {
		return false
	}

	// Check if there's a quoted sender
	if quotedSenderLID.IsEmpty() {
		return false
	}

	// Check if own JID is set
	if t.ownJID.IsEmpty() {
		return false
	}

	// Compare the user part of the JIDs
	return quotedSenderLID.User == t.ownJID.User
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
