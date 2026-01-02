package context

import (
	"context"
	"fmt"
	"strings"

	"go.mau.fi/whatsmeow/types"

	"orion-agent/internal/data/store"
	"orion-agent/internal/service/agent/llm"
)

// Window manages the sliding context window with summarization.
type Window struct {
	builder      *Builder
	summaryStore *store.SummaryStore
	llmClient    *llm.Client
	maxContext   int
}

// NewWindow creates a new context window manager.
func NewWindow(builder *Builder, summaryStore *store.SummaryStore, llmClient *llm.Client, maxContext int) *Window {
	return &Window{
		builder:      builder,
		summaryStore: summaryStore,
		llmClient:    llmClient,
		maxContext:   maxContext,
	}
}

// CheckThreshold returns true if token count exceeds 50% of max context.
func (w *Window) CheckThreshold(tokenCount int) bool {
	return tokenCount > w.maxContext/2
}

// FindSummarizationPoint finds the message ID at the 2/3 mark for summarization.
// Returns (fromMsgID, toMsgID, tokensToSummarize) - summarize from first message to toMsgID.
func (w *Window) FindSummarizationPoint(messages []llm.ChatMessage, chatJID types.JID) (string, string, int, error) {
	if len(messages) == 0 {
		return "", "", 0, fmt.Errorf("no messages to summarize")
	}

	// Calculate total tokens
	totalTokens := llm.EstimateMessagesTokens(messages)

	// Find 2/3 mark (we want to summarize oldest 2/3)
	targetTokens := (totalTokens * 2) / 3
	currentTokens := 0
	splitIndex := 0

	for i, msg := range messages {
		msgTokens := llm.EstimateTokens(msg.Content) + 4
		currentTokens += msgTokens
		if currentTokens >= targetTokens {
			splitIndex = i
			break
		}
	}

	if splitIndex == 0 {
		splitIndex = len(messages) - 1
	}

	// Get message IDs from DB for the range
	// Note: We need to map back to actual message IDs
	// For now, return the index-based split info
	return "", "", currentTokens, nil
}

// Summarize generates a rolling summary from old summary + new messages.
func (w *Window) Summarize(ctx context.Context, chatJID types.JID, oldSummary string, messages []*ContextMessage) (*store.Summary, error) {
	if len(messages) == 0 {
		return nil, fmt.Errorf("no messages to summarize")
	}

	// Build prompt for summarization
	var sb strings.Builder
	sb.WriteString("Summarize the following conversation concisely, preserving key information, decisions, and context:\n\n")

	if oldSummary != "" {
		sb.WriteString("Previous summary:\n")
		sb.WriteString(oldSummary)
		sb.WriteString("\n\nNew messages:\n")
	}

	for _, msg := range messages {
		role := "User"
		if msg.FromMe {
			role = "Assistant"
		}
		content := msg.TextContent
		if content == "" {
			content = msg.Caption
		}
		if content == "" {
			content = fmt.Sprintf("[%s]", msg.MessageType)
		}
		if msg.PushName != "" && !msg.FromMe {
			sb.WriteString(fmt.Sprintf("%s (%s): %s\n", role, msg.PushName, content))
		} else {
			sb.WriteString(fmt.Sprintf("%s: %s\n", role, content))
		}
	}

	// Call LLM for summarization
	req := &llm.ChatCompletionRequest{
		Messages: []llm.ChatMessage{
			{Role: llm.RoleSystem, Content: "You are a summarization assistant. Create concise summaries that preserve important context."},
			{Role: llm.RoleUser, Content: sb.String()},
		},
		MaxTokens:   1000,
		Temperature: 0.3,
	}

	resp, err := w.llmClient.Complete(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("LLM summarization failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no summary generated")
	}

	summaryText := resp.Choices[0].Message.Content
	tokenCount := llm.EstimateTokens(summaryText)

	summary := &store.Summary{
		ChatJID:       chatJID,
		SummaryText:   summaryText,
		TokenCount:    tokenCount,
		FromMessageID: messages[0].ID,
		ToMessageID:   messages[len(messages)-1].ID,
	}

	// Store the summary
	if err := w.summaryStore.Put(summary); err != nil {
		return nil, fmt.Errorf("failed to store summary: %w", err)
	}

	return summary, nil
}

// ShouldSummarize checks if summarization is needed and performs it if so.
func (w *Window) ShouldSummarize(ctx context.Context, chatJID types.JID, tokenCount int) error {
	if !w.CheckThreshold(tokenCount) {
		return nil
	}

	// Get existing summary
	var oldSummary string
	existingSummary, err := w.summaryStore.GetLatest(chatJID)
	if err == nil && existingSummary != nil {
		oldSummary = existingSummary.SummaryText
	}

	// Get messages to summarize (oldest 2/3)
	messages, err := w.builder.getMessagesAfter(chatJID, "", w.maxContext)
	if err != nil {
		return err
	}

	if len(messages) < 3 {
		return nil // Not enough messages to summarize
	}

	// Find 2/3 point
	totalTokens := 0
	for _, msg := range messages {
		content := msg.TextContent
		if content == "" {
			content = msg.Caption
		}
		totalTokens += llm.EstimateTokens(content) + 4
	}

	targetTokens := (totalTokens * 2) / 3
	currentTokens := 0
	splitIndex := 0

	for i, msg := range messages {
		content := msg.TextContent
		if content == "" {
			content = msg.Caption
		}
		currentTokens += llm.EstimateTokens(content) + 4
		if currentTokens >= targetTokens {
			splitIndex = i
			break
		}
	}

	if splitIndex == 0 {
		return nil
	}

	// Summarize oldest 2/3
	messagesToSummarize := messages[:splitIndex+1]
	_, err = w.Summarize(ctx, chatJID, oldSummary, messagesToSummarize)
	return err
}
