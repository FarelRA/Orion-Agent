package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

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

// AgentService is the main AI agent coordinator.
type AgentService struct {
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

// NewAgentService creates a new agent service.
func NewAgentService(
	cfg *config.Config,
	appStore *store.Store,
	settings *store.SettingsStore,
	summaryStore *store.SummaryStore,
	toolStore *store.ToolStore,
	sendService *send.SendService,
	log waLog.Logger,
) *AgentService {
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

	// Create context builder and window
	ctxBuilder := agentctx.NewBuilder(appStore, summaryStore, toolStore, agentName)
	var ctxWindow *agentctx.Window
	if llmClient != nil {
		ctxWindow = agentctx.NewWindow(ctxBuilder, summaryStore, llmClient, llmClient.MaxContext())
	}

	return &AgentService{
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
func (s *AgentService) SetOwnJID(jid types.JID) {
	s.ownJID = jid
	s.trigger.SetOwnJID(jid)
}

// HandleMessage handles a new message from the database.
// This implements the AgentProcessor interface for direct side-by-side processing.
func (s *AgentService) HandleMessage(ctx context.Context, msg *store.Message) {
	// PIPELINE START

	// 1. Process and prepare message
	inputMsg := s.ProcessMessage(msg)
	if inputMsg == nil {
		s.log.Debugf("Skipping message from %s: %s", msg.SenderLID, "Message is not a valid message")
		return
	}

	// 2. Check for commands
	if s.cmdRegistry.IsCommand(inputMsg.Text) {
		s.log.Debugf("Processing command from %s: %s", inputMsg.SenderJID, inputMsg.Text)
		if err := s.cmdRegistry.Execute(ctx, inputMsg.Text, inputMsg.ChatJID, inputMsg.SenderJID); err != nil {
			s.log.Errorf("Command execution failed: %v", err)
		}
		return
	}

	// 3. Check trigger
	result := s.trigger.ShouldRespond(inputMsg.Text, inputMsg.ChatJID, inputMsg.SenderJID, inputMsg.MentionedJIDs, inputMsg.QuotedSenderLID)
	if !result.ShouldRespond {
		s.log.Debugf("Skipping message from %s: %s", inputMsg.SenderJID, result.Reason)
		return
	}
	s.log.Infof("Processing message from %s: %s", inputMsg.SenderJID, result.Reason)

	// 4. Show typing indicator
	s.sendService.StartTyping(ctx, inputMsg.ChatJID)
	defer s.sendService.StopTyping(ctx, inputMsg.ChatJID)

	// 5. Context Building
	maxTokens := s.llmClient.MaxContext()
	ctxResult, err := s.ctxBuilder.BuildContext(inputMsg.ChatJID, maxTokens, s.ownJID, inputMsg)
	if err != nil {
		s.log.Warnf("Failed to build context: %v", err)
		return
	}

	// 6. Summarization Check
	if s.ctxWindow != nil && s.ctxWindow.CheckThreshold(ctxResult.TokenCount) {
		if err := s.ctxWindow.ShouldSummarize(ctx, inputMsg.ChatJID, ctxResult.TokenCount); err != nil {
			s.log.Warnf("Summarization failed: %v", err)
		}
		// Rebuild context after summarization
		ctxResult, err = s.ctxBuilder.BuildContext(inputMsg.ChatJID, maxTokens, s.ownJID, inputMsg)
		if err != nil {
			s.log.Errorf("Failed to rebuild context after summarization: %v", err)
			return
		}
	}

	// 7. Build System Prompt & Combine Messages
	basePrompt := s.settings.GetSystemPrompt(inputMsg.ChatJID.String())
	systemPrompt := s.buildSystemPrompt(basePrompt, ctxResult.NextIndex)
	reqMessages := []llm.ChatMessage{{Role: llm.RoleSystem, Content: systemPrompt}}
	reqMessages = append(reqMessages, ctxResult.Messages...)

	// 8. Execute Agent (LLM + Tools)
	response, toolCallsJSON, toolResultsJSON, err := s.callLLM(
		ctx,
		reqMessages,
		inputMsg.ChatJID,
		inputMsg.SenderJID,
		inputMsg.ID,
		ctxResult.MessageMap,
	)
	if err != nil {
		s.log.Errorf("LLM call failed: %v", err)
		return
	}

	// 9. Handle Response
	responseContent := s.sanitizeResponse(response, ctxResult.NextIndex)
	if responseContent != "" {
		sendResult, err := s.sendService.Send(ctx, inputMsg.ChatJID, send.Text(responseContent))
		if err != nil {
			s.log.Errorf("Failed to send response: %v", err)
			return
		}

		// Save tool calls to DB if any
		if len(toolCallsJSON) > 0 || len(toolResultsJSON) > 0 {
			err = s.toolStore.Put(string(sendResult.MessageID), inputMsg.ChatJID.String(), toolCallsJSON, toolResultsJSON)
			if err != nil {
				s.log.Warnf("Failed to save tool calls: %v", err)
			}
		}
	}

	// PIPELINE END
}

// ProcessMessage extracts and prepares message data for the agent key pipeline.
// It handles text extraction, normalization, and validation.
func (s *AgentService) ProcessMessage(msg *store.Message) *agentctx.InputMessage {
	// Skip if from self
	if msg.FromMe {
		return nil
	}

	// Skip old/history messages based on config (0 = no limit)
	maxAge := s.config.AI.MaxMessageAge
	if maxAge > 0 && time.Since(msg.Timestamp) > time.Duration(maxAge)*time.Second {
		return nil
	}

	// Get text content
	var text string
	if msg.TextContent != "" {
		text = msg.TextContent
	} else if msg.Caption != "" {
		text = msg.Caption
	} else {
		return nil
	}

	// Convert mentioned JIDs to strings
	var mentionedJIDs []string
	for _, jid := range msg.MentionedJIDs {
		mentionedJIDs = append(mentionedJIDs, jid.String())
	}

	return &agentctx.InputMessage{
		ChatJID:         msg.ChatJID,
		SenderJID:       msg.SenderLID,
		ID:              msg.ID,
		Text:            text,
		SenderLID:       msg.SenderLID.String(),
		PushName:        msg.PushName,
		IsDM:            msg.ChatJID.Server == types.DefaultUserServer,
		MentionedJIDs:   mentionedJIDs,
		QuotedSenderLID: msg.QuotedSenderLID,
		QuotedMessageID: msg.QuotedMessageID,
		QuotedContent:   msg.QuotedContent,
	}
}

// buildSystemPrompt builds the system prompt with format instructions.
func (s *AgentService) buildSystemPrompt(basePrompt string, nextIndex int) string {
	agentName := s.config.AI.AgentName
	formatInstructions := fmt.Sprintf(`
%s

IMPORTANT: Format your responses as: {index}|%s|{your message}
Example: %d|%s|Hello! How can I help you?

The conversation uses this format where each message has an index number.
Your response should use the next available index.
`, basePrompt, agentName, nextIndex, agentName)
	return formatInstructions
}

// sanitizeResponse cleans the LLM response.
// It removes the {index}|{sender}| prefix if present and trims whitespace.
func (s *AgentService) sanitizeResponse(response string, nextIndex int) string {
	// Remove leading/trailing whitespace
	response = strings.TrimSpace(response)

	// Check for {index}|{sender}|{content} format
	// Regex: digits + pipe + anything + pipe + content
	parts := strings.SplitN(response, "|", 3)
	if len(parts) == 3 {
		// Extract index
		index, _ := strconv.Atoi(parts[1])

		// If index matches, return the content
		if index == nextIndex {
			response = parts[2]
		}
	}

	// Trim again after extraction
	return strings.TrimSpace(response)
}

// callLLM handles the LLM call with tool execution loop.
// Returns: response content, tool calls JSON, tool results JSON, error
func (s *AgentService) callLLM(ctx context.Context, messages []llm.ChatMessage, chatJID, senderJID types.JID, messageID string, messageMap map[int]string) (string, []byte, []byte, error) {
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
func (s *AgentService) GetToolRegistry() *tools.Registry {
	return s.toolRegistry
}

// GetCommandRegistry returns the command registry for external registration.
func (s *AgentService) GetCommandRegistry() *command.Registry {
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
