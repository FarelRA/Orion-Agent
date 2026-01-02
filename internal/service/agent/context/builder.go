package context

import (
	"database/sql"
	"fmt"

	"go.mau.fi/whatsmeow/types"

	"orion-agent/internal/data/store"
	"orion-agent/internal/service/agent/llm"
)

// Builder builds conversation context from database.
type Builder struct {
	store        *store.Store
	summaryStore *store.SummaryStore
}

// NewBuilder creates a new context builder.
func NewBuilder(s *store.Store, sumStore *store.SummaryStore) *Builder {
	return &Builder{
		store:        s,
		summaryStore: sumStore,
	}
}

// BuildContext builds conversation context for a chat.
// Returns messages formatted for LLM and total token count.
func (b *Builder) BuildContext(chatJID types.JID, maxTokens int, ownJID types.JID) ([]llm.ChatMessage, int, error) {
	// Get latest summary if exists
	var summaryText string
	var summaryTokens int
	var lastSummarizedMsgID string

	summary, err := b.summaryStore.GetLatest(chatJID)
	if err == nil && summary != nil {
		summaryText = summary.SummaryText
		summaryTokens = summary.TokenCount
		lastSummarizedMsgID = summary.ToMessageID
	}

	// Get messages from DB (after last summarized message if exists)
	messages, err := b.getMessagesAfter(chatJID, lastSummarizedMsgID, maxTokens-summaryTokens)
	if err != nil {
		return nil, 0, err
	}

	// Convert to LLM messages
	var result []llm.ChatMessage
	totalTokens := summaryTokens

	// Add summary as system context if exists
	if summaryText != "" {
		result = append(result, llm.ChatMessage{
			Role:    llm.RoleSystem,
			Content: fmt.Sprintf("[Previous conversation summary]\n%s", summaryText),
		})
	}

	// Add messages
	for _, msg := range messages {
		role := llm.RoleUser
		if msg.FromMe {
			role = llm.RoleAssistant
		}

		content := b.formatMessageContent(msg)
		tokens := llm.EstimateTokens(content) + 4

		result = append(result, llm.ChatMessage{
			Role:    role,
			Content: content,
		})
		totalTokens += tokens
	}

	return result, totalTokens, nil
}

// ContextMessage is a simplified message for context building.
type ContextMessage struct {
	ID          string
	FromMe      bool
	PushName    string
	MessageType string
	TextContent string
	Caption     string
	Timestamp   int64
}

// getMessagesAfter fetches messages after a given message ID.
func (b *Builder) getMessagesAfter(chatJID types.JID, afterMsgID string, maxTokens int) ([]*ContextMessage, error) {
	query := `
		SELECT id, from_me, push_name, message_type, text_content, caption, timestamp
		FROM orion_messages 
		WHERE chat_jid = ? AND is_revoked = 0`

	args := []interface{}{chatJID.String()}

	if afterMsgID != "" {
		query += ` AND timestamp > (SELECT timestamp FROM orion_messages WHERE id = ? AND chat_jid = ?)`
		args = append(args, afterMsgID, chatJID.String())
	}

	query += ` ORDER BY timestamp ASC`

	rows, err := b.store.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*ContextMessage
	totalTokens := 0

	for rows.Next() {
		msg, err := b.scanContextMessage(rows)
		if err != nil {
			continue
		}

		content := msg.TextContent
		if content == "" {
			content = msg.Caption
		}
		tokens := llm.EstimateTokens(content) + 4

		if totalTokens+tokens > maxTokens {
			break
		}

		messages = append(messages, msg)
		totalTokens += tokens
	}

	return messages, nil
}

func (b *Builder) scanContextMessage(rows *sql.Rows) (*ContextMessage, error) {
	var msg ContextMessage
	var pushName, textContent, caption sql.NullString
	var fromMe int

	err := rows.Scan(
		&msg.ID, &fromMe, &pushName, &msg.MessageType, &textContent, &caption, &msg.Timestamp,
	)
	if err != nil {
		return nil, err
	}

	msg.FromMe = fromMe == 1
	msg.PushName = pushName.String
	msg.TextContent = textContent.String
	msg.Caption = caption.String

	return &msg, nil
}

// formatMessageContent formats a message for LLM context.
func (b *Builder) formatMessageContent(msg *ContextMessage) string {
	content := msg.TextContent
	if content == "" {
		content = msg.Caption
	}
	if content == "" {
		content = fmt.Sprintf("[%s message]", msg.MessageType)
	}

	// Add sender name for group context
	if !msg.FromMe && msg.PushName != "" {
		content = fmt.Sprintf("%s: %s", msg.PushName, content)
	}

	return content
}

// GetMessagesForSummary gets messages between two message IDs for summarization.
func (b *Builder) GetMessagesForSummary(chatJID types.JID, fromMsgID, toMsgID string) ([]*ContextMessage, error) {
	query := `
		SELECT id, from_me, push_name, message_type, text_content, caption, timestamp
		FROM orion_messages 
		WHERE chat_jid = ? AND is_revoked = 0
		AND timestamp >= (SELECT timestamp FROM orion_messages WHERE id = ? AND chat_jid = ?)
		AND timestamp <= (SELECT timestamp FROM orion_messages WHERE id = ? AND chat_jid = ?)
		ORDER BY timestamp ASC`

	rows, err := b.store.Query(query, chatJID.String(), fromMsgID, chatJID.String(), toMsgID, chatJID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*ContextMessage
	for rows.Next() {
		msg, err := b.scanContextMessage(rows)
		if err != nil {
			continue
		}
		messages = append(messages, msg)
	}

	return messages, nil
}
