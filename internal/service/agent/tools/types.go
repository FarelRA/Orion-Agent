package tools

import (
	"context"
	"encoding/json"

	"go.mau.fi/whatsmeow/types"

	"orion-agent/internal/service/agent/llm"
)

// Tool is the interface all tools must implement.
type Tool interface {
	Name() string
	Description() string
	Parameters() json.RawMessage
	Execute(ctx context.Context, args json.RawMessage, execCtx *ExecutionContext) (*Result, error)
}

// ExecutionContext provides context for tool execution.
type ExecutionContext struct {
	ChatJID    types.JID
	SenderJID  types.JID
	MessageID  string
	FromMe     bool
	MessageMap map[int]string // index → real message ID
}

// Result is the result of a tool execution.
type Result struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// SuccessResult creates a successful result.
func SuccessResult(data interface{}) *Result {
	return &Result{Success: true, Data: data}
}

// ErrorResult creates an error result.
func ErrorResult(err string) *Result {
	return &Result{Success: false, Error: err}
}

// Definition returns the OpenAI-compatible tool definition.
func Definition(t Tool) llm.Tool {
	return llm.Tool{
		Type: "function",
		Function: llm.FunctionDefinition{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  t.Parameters(),
		},
	}
}

// ParameterSchema helps build JSON schema for parameters.
type ParameterSchema struct {
	Type       string                    `json:"type"`
	Properties map[string]PropertySchema `json:"properties,omitempty"`
	Required   []string                  `json:"required,omitempty"`
}

// PropertySchema defines a single property in the schema.
type PropertySchema struct {
	Type        string   `json:"type"`
	Description string   `json:"description,omitempty"`
	Enum        []string `json:"enum,omitempty"`
}

// MustMarshal marshals to JSON or panics.
func MustMarshal(v interface{}) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}
