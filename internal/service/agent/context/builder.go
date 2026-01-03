package context

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"go.mau.fi/whatsmeow/types"

	"orion-agent/internal/data/store"
	"orion-agent/internal/service/agent/llm"
)

type InputMessage struct {
	ChatJID         types.JID
	SenderJID       types.JID
	ID              string
	Text            string
	SenderLID       string
	PushName        string
	IsDM            bool
	MentionedJIDs   []string
	QuotedSenderLID types.JID
	QuotedMessageID string
	QuotedContent   string
}

// ContextResult holds the built context with index mapping.
type ContextResult struct {
	Messages   []llm.ChatMessage
	TokenCount int
	MessageMap map[int]string // index → real message ID
	NextIndex  int            // for AI's response
}

// Builder builds conversation context from database.
type Builder struct {
	store        *store.Store
	summaryStore *store.SummaryStore
	toolStore    *store.ToolStore
	agentName    string
}

// NewBuilder creates a new context builder.
func NewBuilder(s *store.Store, sumStore *store.SummaryStore, toolStore *store.ToolStore, agentName string) *Builder {
	return &Builder{
		store:        s,
		summaryStore: sumStore,
		toolStore:    toolStore,
		agentName:    agentName,
	}
}

// BuildContext builds conversation context for a chat.
// Returns ContextResult with messages, token count, and index mapping.
func (b *Builder) BuildContext(chatJID types.JID, maxTokens int, ownJID types.JID, currentMsg *InputMessage) (*ContextResult, error) {
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
		return nil, err
	}

	// Convert to LLM messages with index|sender|content format
	var result []llm.ChatMessage
	totalTokens := summaryTokens
	messageMap := make(map[int]string)
	index := 1

	// User mapping state
	userIndexMap := make(map[string]int)
	nextUserIndex := 1
	isDM := chatJID.Server == types.DefaultUserServer

	// Add summary as system context if exists
	if summaryText != "" {
		result = append(result, llm.ChatMessage{
			Role:    llm.RoleSystem,
			Content: fmt.Sprintf("[Previous conversation summary]\n%s", summaryText),
		})
	}

	// Add messages with indexed format
	for _, msg := range messages {
		role := llm.RoleUser
		if msg.FromMe {
			role = llm.RoleAssistant
		}

		// Resolve sender name
		senderName := b.resolveSenderName(msg, isDM, userIndexMap, &nextUserIndex)

		// Store mapping FIRST so replies can reference it
		messageMap[index] = msg.ID

		// Format: {index}|{sender}|{content} (with > prefix for replies)
		content := b.formatMessageContent(msg, index, senderName, messageMap)
		tokens := llm.EstimateTokens(content) + 4

		// Check if this message has associated tool calls
		toolRecord, _ := b.toolStore.GetByMessageID(msg.ID)
		if toolRecord != nil && msg.FromMe {
			// Insert tool calls/results before the assistant response
			toolMessages := b.formatToolMessages(toolRecord)
			for _, tm := range toolMessages {
				result = append(result, tm)
				totalTokens += llm.EstimateTokens(tm.Content) + 4
			}
		}

		result = append(result, llm.ChatMessage{
			Role:    role,
			Content: content,
		})
		totalTokens += tokens
		index++
	}

	// Handle current message (deduplication and insertion)
	if currentMsg != nil {
		messageInContext := false
		for _, msg := range messages {
			if msg.ID == currentMsg.ID {
				messageInContext = true
				break
			}
		}

		if !messageInContext {
			// Resolve sender name for current message
			// We need a temporary ContextMessage wrapper for the resolver
			tempMsg := &ContextMessage{
				FromMe:          false, // Current message is from user
				PushName:        currentMsg.PushName,
				SenderLID:       currentMsg.SenderLID,
				QuotedMessageID: currentMsg.QuotedMessageID,
				QuotedSenderLID: currentMsg.QuotedSenderLID.String(),
				QuotedContent:   currentMsg.QuotedContent,
			}
			senderName := b.resolveSenderName(tempMsg, currentMsg.IsDM, userIndexMap, &nextUserIndex)

			// Store in map first so formatMessageContent can use it
			messageMap[index] = currentMsg.ID

			// Format content with reply support
			content := b.formatMessageContent(tempMsg, index, senderName, messageMap)
			// Override text content for current message (it's not from DB)
			if currentMsg.QuotedMessageID != "" {
				// Find quoted message index
				var quotedIndex int
				for idx, msgID := range messageMap {
					if msgID == currentMsg.QuotedMessageID {
						quotedIndex = idx
						break
					}
				}

				quotedPreview := currentMsg.QuotedContent
				if len(quotedPreview) > 50 {
					quotedPreview = quotedPreview[:47] + "..."
				}

				quotedSender := "User"
				mainLine := fmt.Sprintf("%d|%s|%s", index, senderName, currentMsg.Text)

				if quotedIndex > 0 && quotedPreview != "" {
					content = fmt.Sprintf("> %d|%s|%s\n%s", quotedIndex, quotedSender, quotedPreview, mainLine)
				} else if quotedPreview != "" {
					content = fmt.Sprintf("> ?|%s|%s\n%s", quotedSender, quotedPreview, mainLine)
				} else {
					content = mainLine
				}
			} else {
				content = fmt.Sprintf("%d|%s|%s", index, senderName, currentMsg.Text)
			}

			tokens := llm.EstimateTokens(content) + 4

			// Add to result
			result = append(result, llm.ChatMessage{
				Role:    llm.RoleUser,
				Content: content,
			})
			totalTokens += tokens
			index++
		}
	}

	return &ContextResult{
		Messages:   result,
		TokenCount: totalTokens,
		MessageMap: messageMap,
		NextIndex:  index,
	}, nil
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
	SenderLID   string

	// Quote/Reply context
	QuotedMessageID string
	QuotedSenderLID string
	QuotedContent   string

	// Contact info
	FullName     string
	FirstName    string
	BusinessName string
}

// getMessagesAfter fetches messages after a given message ID.
func (b *Builder) getMessagesAfter(chatJID types.JID, afterMsgID string, maxTokens int) ([]*ContextMessage, error) {
	query := `
		SELECT 
			m.id, m.from_me, m.push_name, m.message_type, m.text_content, m.caption, m.timestamp, m.sender_lid,
			m.quoted_message_id, m.quoted_sender_lid, m.quoted_content,
			c.full_name, c.first_name, c.business_name
		FROM orion_messages m
		LEFT JOIN orion_contacts c ON m.sender_lid = c.lid
		WHERE m.chat_jid = ? AND m.is_revoked = 0`

	args := []interface{}{chatJID.String()}

	if afterMsgID != "" {
		query += ` AND m.timestamp > (SELECT timestamp FROM orion_messages WHERE id = ? AND chat_jid = ?)`
		args = append(args, afterMsgID, chatJID.String())
	}

	query += ` ORDER BY m.timestamp ASC`

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
	var pushName, textContent, caption, senderLID sql.NullString
	var quotedMsgID, quotedSenderLID, quotedContent sql.NullString
	var fullName, firstName, businessName sql.NullString
	var fromMe int

	err := rows.Scan(
		&msg.ID, &fromMe, &pushName, &msg.MessageType, &textContent, &caption, &msg.Timestamp, &senderLID,
		&quotedMsgID, &quotedSenderLID, &quotedContent,
		&fullName, &firstName, &businessName,
	)
	if err != nil {
		return nil, err
	}

	msg.FromMe = fromMe == 1
	msg.PushName = pushName.String
	msg.TextContent = textContent.String
	msg.Caption = caption.String
	msg.SenderLID = senderLID.String
	msg.QuotedMessageID = quotedMsgID.String
	msg.QuotedSenderLID = quotedSenderLID.String
	msg.QuotedContent = quotedContent.String
	msg.FullName = fullName.String
	msg.FirstName = firstName.String
	msg.BusinessName = businessName.String

	return &msg, nil
}

// resolveSenderName determines the best name for the sender.
func (b *Builder) resolveSenderName(msg *ContextMessage, isDM bool, userIndexMap map[string]int, nextUserIndex *int) string {
	if msg.FromMe {
		return b.agentName
	}

	// Order: Full Name > First Name > Push Name > Business Name
	if msg.FullName != "" {
		return msg.FullName
	}
	if msg.FirstName != "" {
		return msg.FirstName
	}
	if msg.PushName != "" {
		return msg.PushName
	}
	if msg.BusinessName != "" {
		return msg.BusinessName
	}

	// Fallback
	if isDM {
		return "User"
	}

	// User{index} for groups/others
	if msg.SenderLID == "" {
		return "Unknown"
	}

	idx, exists := userIndexMap[msg.SenderLID]
	if !exists {
		idx = *nextUserIndex
		userIndexMap[msg.SenderLID] = idx
		*nextUserIndex++
	}
	return fmt.Sprintf("User%d", idx)
}

// formatMessageContent formats a message with index|sender|content format.
// For replied messages, it prepends with > quotedIndex|sender|quotedContent
func (b *Builder) formatMessageContent(msg *ContextMessage, index int, senderName string, messageMap map[int]string) string {
	content := msg.TextContent
	if content == "" {
		content = msg.Caption
	}
	if content == "" {
		content = fmt.Sprintf("[%s]", msg.MessageType)
	}

	// Build the main message line
	mainLine := fmt.Sprintf("%d|%s|%s", index, senderName, content)

	// If this is a reply, prepend with quoted context
	if msg.QuotedMessageID != "" {
		// Find the quoted message index from messageMap
		var quotedIndex int
		for idx, msgID := range messageMap {
			if msgID == msg.QuotedMessageID {
				quotedIndex = idx
				break
			}
		}

		// Build quoted preview
		quotedPreview := msg.QuotedContent
		if len(quotedPreview) > 50 {
			quotedPreview = quotedPreview[:47] + "..."
		}

		// Determine quoted sender name (simplified - use agent name if from self)
		quotedSender := "Unknown"
		if msg.QuotedSenderLID != "" {
			// This is a simplified check - in production you'd look up the name
			quotedSender = "User"
		}

		// Format: > quotedIndex|quotedSender|quotedPreview\n index|sender|content
		if quotedIndex > 0 {
			return fmt.Sprintf("> %d|%s|%s\n%s", quotedIndex, quotedSender, quotedPreview, mainLine)
		}
		// If quoted message not in context, just show preview
		if quotedPreview != "" {
			return fmt.Sprintf("> ?|%s|%s\n%s", quotedSender, quotedPreview, mainLine)
		}
	}

	return mainLine
}

// formatToolMessages formats tool call/result as LLM messages.
func (b *Builder) formatToolMessages(record *store.ToolRecord) []llm.ChatMessage {
	var messages []llm.ChatMessage

	// Add tool calls
	if len(record.ToolCalls) > 0 {
		messages = append(messages, llm.ChatMessage{
			Role:    llm.RoleTool,
			Content: string(record.ToolCalls),
		})
	}

	// Add tool results
	if len(record.ToolResults) > 0 {
		messages = append(messages, llm.ChatMessage{
			Role:    llm.RoleTool,
			Content: string(record.ToolResults),
		})
	}

	return messages
}

// GetMessagesForSummary gets messages between two message IDs for summarization.
func (b *Builder) GetMessagesForSummary(chatJID types.JID, fromMsgID, toMsgID string) ([]*ContextMessage, error) {
	query := `
		SELECT 
			m.id, m.from_me, m.push_name, m.message_type, m.text_content, m.caption, m.timestamp, m.sender_lid,
			c.full_name, c.first_name, c.business_name
		FROM orion_messages m
		LEFT JOIN orion_contacts c ON m.sender_lid = c.lid
		WHERE m.chat_jid = ? AND m.is_revoked = 0
		AND m.timestamp >= (SELECT timestamp FROM orion_messages WHERE id = ? AND chat_jid = ?)
		AND m.timestamp <= (SELECT timestamp FROM orion_messages WHERE id = ? AND chat_jid = ?)
		ORDER BY m.timestamp ASC`

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

// ToolCallData represents a tool call for JSON serialization.
type ToolCallData struct {
	ID   string          `json:"id"`
	Tool string          `json:"tool"`
	Args json.RawMessage `json:"args"`
}

// ToolResultData represents a tool result for JSON serialization.
type ToolResultData struct {
	ToolCallID string `json:"tool_call_id"`
	Success    bool   `json:"success"`
	Data       any    `json:"data,omitempty"`
	Error      string `json:"error,omitempty"`
}
