package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"go.mau.fi/whatsmeow/types"
	waLog "go.mau.fi/whatsmeow/util/log"

	"orion-agent/internal/data/store"
	"orion-agent/internal/infra/config"
	"orion-agent/internal/service/agent/command"
	agentctx "orion-agent/internal/service/agent/context"
	"orion-agent/internal/service/agent/llm"
	"orion-agent/internal/service/agent/tools"
	"orion-agent/internal/service/agent/tools/builtin"
	"orion-agent/internal/service/agent/trigger"
	"orion-agent/internal/service/send"
)

// Service is the main AI agent coordinator.
type Service struct {
	config       *config.Config
	settings     *store.SettingsStore
	llmClient    *llm.Client
	sendService  *send.SendService
	toolRegistry *tools.Registry
	cmdRegistry  *command.Registry
	trigger      *trigger.Trigger
	ctxBuilder   *agentctx.Builder
	ctxWindow    *agentctx.Window
	log          waLog.Logger
	ownJID       types.JID
}

// NewService creates a new agent service.
func NewService(
	cfg *config.Config,
	appStore *store.Store,
	settings *store.SettingsStore,
	summaryStore *store.SummaryStore,
	sendService *send.SendService,
	log waLog.Logger,
) *Service {
	// Get model config
	modelCfg := cfg.GetModel("")
	var llmClient *llm.Client
	if modelCfg != nil {
		llmClient = llm.NewClient(modelCfg)
	}

	// Create tool registry
	toolRegistry := tools.NewRegistry()
	builtin.RegisterMessagingTools(toolRegistry, sendService)
	builtin.RegisterMediaTools(toolRegistry, sendService)
	builtin.RegisterInteractiveTools(toolRegistry, sendService)
	builtin.RegisterPresenceTools(toolRegistry, sendService)

	// Create command registry
	cmdRegistry := command.NewRegistry(settings, sendService)
	cmdRegistry.RegisterBuiltinCommands()

	// Create trigger
	trig := trigger.NewTrigger(settings, types.JID{})

	// Create context builder and window
	ctxBuilder := agentctx.NewBuilder(appStore, summaryStore)
	var ctxWindow *agentctx.Window
	if llmClient != nil {
		ctxWindow = agentctx.NewWindow(ctxBuilder, summaryStore, llmClient, llmClient.MaxContext())
	}

	return &Service{
		config:       cfg,
		settings:     settings,
		llmClient:    llmClient,
		sendService:  sendService,
		toolRegistry: toolRegistry,
		cmdRegistry:  cmdRegistry,
		trigger:      trig,
		ctxBuilder:   ctxBuilder,
		ctxWindow:    ctxWindow,
		log:          log.Sub("Agent"),
	}
}

// SetOwnJID sets the agent's own JID (called after connection).
func (s *Service) SetOwnJID(jid types.JID) {
	s.ownJID = jid
	s.trigger.SetOwnJID(jid)
}

// ProcessMessage is the main entry point for processing incoming messages.
func (s *Service) ProcessMessage(ctx context.Context, chatJID, senderJID types.JID, messageID, messageText string, mentionedJIDs []string, fromMe bool) error {
	// Skip if no LLM client configured
	if s.llmClient == nil {
		return nil
	}

	// Check if it's a command
	if s.cmdRegistry.IsCommand(messageText) {
		return s.cmdRegistry.Execute(ctx, messageText, chatJID, senderJID)
	}

	// Check trigger
	result := s.trigger.ShouldRespond(chatJID, senderJID, messageText, mentionedJIDs, fromMe)
	if !result.ShouldRespond {
		s.log.Debugf("Skipping message from %s: %s", senderJID, result.Reason)
		return nil // <-- This MUST return and stop processing
	}

	s.log.Infof("Processing message from %s: %s", senderJID, result.Reason)

	// Show typing indicator
	s.sendService.StartTyping(ctx, chatJID)
	defer s.sendService.StopTyping(ctx, chatJID)

	// Build context
	maxTokens := s.llmClient.MaxContext()
	messages, tokenCount, err := s.ctxBuilder.BuildContext(chatJID, maxTokens, s.ownJID)
	if err != nil {
		s.log.Warnf("Failed to build context: %v", err)
		messages = []llm.ChatMessage{}
		tokenCount = 0
	}

	// Check if summarization needed
	if s.ctxWindow != nil && s.ctxWindow.CheckThreshold(tokenCount) {
		if err := s.ctxWindow.ShouldSummarize(ctx, chatJID, tokenCount); err != nil {
			s.log.Warnf("Summarization failed: %v", err)
		}
		// Rebuild context after summarization
		messages, tokenCount, _ = s.ctxBuilder.BuildContext(chatJID, maxTokens, s.ownJID)
	}

	// Get system prompt
	systemPrompt := s.settings.GetSystemPrompt(chatJID.String())

	// Build request
	reqMessages := []llm.ChatMessage{
		{Role: llm.RoleSystem, Content: systemPrompt},
	}
	reqMessages = append(reqMessages, messages...)

	// Add current message if not already in context
	reqMessages = append(reqMessages, llm.ChatMessage{
		Role:    llm.RoleUser,
		Content: messageText,
	})

	// Call LLM
	response, err := s.callLLM(ctx, reqMessages, chatJID, senderJID, messageID)
	if err != nil {
		s.log.Errorf("LLM call failed: %v", err)
		return err
	}

	// Send response if any
	if response != "" {
		_, err = s.sendService.Send(ctx, chatJID, send.Text(response))
		if err != nil {
			s.log.Errorf("Failed to send response: %v", err)
		}
	}

	return nil
}

// callLLM handles the LLM call with tool execution loop.
func (s *Service) callLLM(ctx context.Context, messages []llm.ChatMessage, chatJID, senderJID types.JID, messageID string) (string, error) {
	execCtx := &tools.ExecutionContext{
		ChatJID:   chatJID,
		SenderJID: senderJID,
		MessageID: messageID,
	}

	// Get tool definitions
	toolDefs := s.toolRegistry.GetDefinitions()

	for i := 0; i < 10; i++ { // Max 10 iterations to prevent infinite loops
		req := &llm.ChatCompletionRequest{
			Messages: messages,
			Tools:    toolDefs,
		}

		resp, err := s.llmClient.Complete(ctx, req)
		if err != nil {
			return "", fmt.Errorf("LLM request failed: %w", err)
		}

		if len(resp.Choices) == 0 {
			return "", fmt.Errorf("no response from LLM")
		}

		choice := resp.Choices[0]

		// If no tool calls, return the content
		if len(choice.Message.ToolCalls) == 0 {
			return choice.Message.Content, nil
		}

		// Execute tool calls
		s.log.Debugf("Executing %d tool calls", len(choice.Message.ToolCalls))

		// Add assistant message with tool calls
		messages = append(messages, choice.Message)

		// Execute tools and add results
		toolResults := s.toolRegistry.ExecuteToolCalls(ctx, choice.Message.ToolCalls, execCtx)
		messages = append(messages, toolResults...)

		// Log tool results
		for _, result := range toolResults {
			s.log.Debugf("Tool result: %s", result.Content)
		}
	}

	return "", fmt.Errorf("max tool iterations reached")
}

// GetToolRegistry returns the tool registry for external registration.
func (s *Service) GetToolRegistry() *tools.Registry {
	return s.toolRegistry
}

// GetCommandRegistry returns the command registry for external registration.
func (s *Service) GetCommandRegistry() *command.Registry {
	return s.cmdRegistry
}

// MessageInfo contains extracted message info for processing.
type MessageInfo struct {
	ChatJID      types.JID
	SenderJID    types.JID
	MessageID    string
	Text         string
	MentionedJIDs []string
	FromMe       bool
}

// ExtractMentionedJIDs extracts mentioned JIDs from JSON string.
func ExtractMentionedJIDs(mentionedJSON string) []string {
	if mentionedJSON == "" {
		return nil
	}
	var jids []string
	json.Unmarshal([]byte(mentionedJSON), &jids)
	return jids
}
