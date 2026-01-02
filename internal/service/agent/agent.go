package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

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
	toolStore    *store.ToolStore
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
	toolStore *store.ToolStore,
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

	// Get agent name with default
	agentName := cfg.AI.AgentName
	if agentName == "" {
		agentName = "Orion"
	}

	// Create context builder and window
	ctxBuilder := agentctx.NewBuilder(appStore, summaryStore, toolStore, agentName)
	var ctxWindow *agentctx.Window
	if llmClient != nil {
		ctxWindow = agentctx.NewWindow(ctxBuilder, summaryStore, llmClient, llmClient.MaxContext())
	}

	return &Service{
		config:       cfg,
		settings:     settings,
		toolStore:    toolStore,
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
		return nil
	}

	s.log.Infof("Processing message from %s: %s", senderJID, result.Reason)

	// Show typing indicator
	s.sendService.StartTyping(ctx, chatJID)
	defer s.sendService.StopTyping(ctx, chatJID)

	// Build context with new format
	maxTokens := s.llmClient.MaxContext()
	ctxResult, err := s.ctxBuilder.BuildContext(chatJID, maxTokens, s.ownJID)
	if err != nil {
		s.log.Warnf("Failed to build context: %v", err)
		ctxResult = &agentctx.ContextResult{
			Messages:   []llm.ChatMessage{},
			TokenCount: 0,
			MessageMap: make(map[int]string),
			NextIndex:  1,
		}
	}

	// Check if summarization needed
	if s.ctxWindow != nil && s.ctxWindow.CheckThreshold(ctxResult.TokenCount) {
		if err := s.ctxWindow.ShouldSummarize(ctx, chatJID, ctxResult.TokenCount); err != nil {
			s.log.Warnf("Summarization failed: %v", err)
		}
		// Rebuild context after summarization
		ctxResult, _ = s.ctxBuilder.BuildContext(chatJID, maxTokens, s.ownJID)
	}

	// Build system prompt with format instructions
	basePrompt := s.settings.GetSystemPrompt(chatJID.String())
	systemPrompt := s.buildSystemPrompt(basePrompt, ctxResult.NextIndex)

	// Build request
	reqMessages := []llm.ChatMessage{
		{Role: llm.RoleSystem, Content: systemPrompt},
	}
	reqMessages = append(reqMessages, ctxResult.Messages...)

	// Add current message with index format
	agentName := s.config.AI.AgentName
	if agentName == "" {
		agentName = "Orion"
	}

	// Get sender name (could be extracted from message)
	senderName := "User"
	currentMsgFormatted := fmt.Sprintf("%d|%s|%s", ctxResult.NextIndex, senderName, messageText)
	reqMessages = append(reqMessages, llm.ChatMessage{
		Role:    llm.RoleUser,
		Content: currentMsgFormatted,
	})

	// Call LLM with MessageMap
	response, toolCallsJSON, toolResultsJSON, err := s.callLLM(ctx, reqMessages, chatJID, senderJID, messageID, ctxResult.MessageMap)
	if err != nil {
		s.log.Errorf("LLM call failed: %v", err)
		return err
	}

	// Post-process response to ensure format
	response = s.ensureFormat(response, ctxResult.NextIndex+1, agentName)

	// Extract just the content (after the format prefix)
	responseContent := s.extractContent(response)

	// Send response if any
	if responseContent != "" {
		sendResult, err := s.sendService.Send(ctx, chatJID, send.Text(responseContent))
		if err != nil {
			s.log.Errorf("Failed to send response: %v", err)
			return err
		}

		// Save tool calls to DB if any
		if len(toolCallsJSON) > 0 || len(toolResultsJSON) > 0 {
			err = s.toolStore.Put(string(sendResult.MessageID), chatJID.String(), toolCallsJSON, toolResultsJSON)
			if err != nil {
				s.log.Warnf("Failed to save tool calls: %v", err)
			}
		}
	}

	return nil
}

// buildSystemPrompt builds the system prompt with format instructions.
func (s *Service) buildSystemPrompt(basePrompt string, nextIndex int) string {
	agentName := s.config.AI.AgentName
	if agentName == "" {
		agentName = "Orion"
	}

	formatInstructions := fmt.Sprintf(`
%s

IMPORTANT: Format your responses as: {index}|%s|{your message}
Example: %d|%s|Hello! How can I help you?

The conversation uses this format where each message has an index number.
Your response should use the next available index.
`, basePrompt, agentName, nextIndex, agentName)

	return formatInstructions
}

// ensureFormat ensures the response follows the format.
func (s *Service) ensureFormat(response string, nextIndex int, agentName string) string {
	if response == "" {
		return ""
	}

	// Check if response already has format (starts with number|)
	formatRegex := regexp.MustCompile(`^\d+\|`)
	if formatRegex.MatchString(response) {
		return response
	}

	// Add format prefix
	return fmt.Sprintf("%d|%s|%s", nextIndex, agentName, response)
}

// extractContent extracts the message content from formatted response.
func (s *Service) extractContent(response string) string {
	if response == "" {
		return ""
	}

	// Split by | and get everything after second |
	parts := strings.SplitN(response, "|", 3)
	if len(parts) >= 3 {
		return parts[2]
	}
	return response
}

// callLLM handles the LLM call with tool execution loop.
// Returns: response content, tool calls JSON, tool results JSON, error
func (s *Service) callLLM(ctx context.Context, messages []llm.ChatMessage, chatJID, senderJID types.JID, messageID string, messageMap map[int]string) (string, []byte, []byte, error) {
	execCtx := &tools.ExecutionContext{
		ChatJID:    chatJID,
		SenderJID:  senderJID,
		MessageID:  messageID,
		MessageMap: messageMap,
	}

	// Get tool definitions
	toolDefs := s.toolRegistry.GetDefinitions()

	var allToolCalls []llm.ToolCall
	var allToolResults []llm.ChatMessage

	for i := 0; i < 10; i++ { // Max 10 iterations to prevent infinite loops
		req := &llm.ChatCompletionRequest{
			Messages: messages,
			Tools:    toolDefs,
		}

		resp, err := s.llmClient.Complete(ctx, req)
		if err != nil {
			return "", nil, nil, fmt.Errorf("LLM request failed: %w", err)
		}

		if len(resp.Choices) == 0 {
			return "", nil, nil, fmt.Errorf("no response from LLM")
		}

		choice := resp.Choices[0]

		// If no tool calls, return the content
		if len(choice.Message.ToolCalls) == 0 {
			// Serialize collected tool calls and results
			var toolCallsJSON, toolResultsJSON []byte
			if len(allToolCalls) > 0 {
				toolCallsJSON, _ = json.Marshal(allToolCalls)
			}
			if len(allToolResults) > 0 {
				toolResultsJSON, _ = json.Marshal(allToolResults)
			}
			return choice.Message.Content, toolCallsJSON, toolResultsJSON, nil
		}

		// Collect tool calls
		allToolCalls = append(allToolCalls, choice.Message.ToolCalls...)

		// Execute tool calls
		s.log.Debugf("Executing %d tool calls", len(choice.Message.ToolCalls))

		// Add assistant message with tool calls
		messages = append(messages, choice.Message)

		// Execute tools and add results
		toolResults := s.toolRegistry.ExecuteToolCalls(ctx, choice.Message.ToolCalls, execCtx)
		messages = append(messages, toolResults...)
		allToolResults = append(allToolResults, toolResults...)

		// Log tool results
		for _, result := range toolResults {
			s.log.Debugf("Tool result: %s", result.Content)
		}
	}

	return "", nil, nil, fmt.Errorf("max tool iterations reached")
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
	ChatJID       types.JID
	SenderJID     types.JID
	MessageID     string
	Text          string
	MentionedJIDs []string
	FromMe        bool
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
