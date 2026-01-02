package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"orion-agent/internal/service/agent/llm"
)

// Registry manages tool registration and execution.
type Registry struct {
	tools map[string]Tool
	mu    sync.RWMutex
}

// NewRegistry creates a new tool registry.
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

// Register adds a tool to the registry.
func (r *Registry) Register(t Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[t.Name()] = t
}

// Get retrieves a tool by name.
func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

// List returns all registered tool names.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}

// GetDefinitions returns OpenAI-compatible tool definitions.
func (r *Registry) GetDefinitions() []llm.Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	defs := make([]llm.Tool, 0, len(r.tools))
	for _, t := range r.tools {
		defs = append(defs, Definition(t))
	}
	return defs
}

// Execute runs a tool by name with the given arguments.
func (r *Registry) Execute(ctx context.Context, name string, argsJSON string, execCtx *ExecutionContext) (*Result, error) {
	tool, ok := r.Get(name)
	if !ok {
		return nil, fmt.Errorf("tool not found: %s", name)
	}

	var args json.RawMessage
	if argsJSON != "" {
		args = json.RawMessage(argsJSON)
	}

	return tool.Execute(ctx, args, execCtx)
}

// ExecuteToolCalls processes multiple tool calls from LLM response.
func (r *Registry) ExecuteToolCalls(ctx context.Context, calls []llm.ToolCall, execCtx *ExecutionContext) []llm.ChatMessage {
	results := make([]llm.ChatMessage, 0, len(calls))

	for _, call := range calls {
		result, err := r.Execute(ctx, call.Function.Name, call.Function.Arguments, execCtx)

		var content string
		if err != nil {
			content = fmt.Sprintf(`{"success":false,"error":%q}`, err.Error())
		} else {
			data, _ := json.Marshal(result)
			content = string(data)
		}

		results = append(results, llm.ChatMessage{
			Role:       llm.RoleTool,
			Content:    content,
			ToolCallID: call.ID,
		})
	}

	return results
}
