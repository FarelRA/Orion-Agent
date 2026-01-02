package builtin

import (
	"context"
	"encoding/json"
	"fmt"

	"go.mau.fi/whatsmeow/types"

	"orion-agent/internal/service/agent/tools"
	"orion-agent/internal/service/send"
)

// SetTypingTool shows typing indicator.
type SetTypingTool struct {
	sendService *send.SendService
}

func NewSetTypingTool(s *send.SendService) *SetTypingTool {
	return &SetTypingTool{sendService: s}
}

func (t *SetTypingTool) Name() string { return "set_typing" }

func (t *SetTypingTool) Description() string {
	return "Show or hide typing indicator in the chat"
}

func (t *SetTypingTool) Parameters() json.RawMessage {
	return tools.MustMarshal(tools.ParameterSchema{
		Type: "object",
		Properties: map[string]tools.PropertySchema{
			"typing": {Type: "boolean", Description: "true to show typing, false to stop"},
		},
		Required: []string{"typing"},
	})
}

func (t *SetTypingTool) Execute(ctx context.Context, args json.RawMessage, execCtx *tools.ExecutionContext) (*tools.Result, error) {
	var params struct {
		Typing bool `json:"typing"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return tools.ErrorResult("invalid parameters"), nil
	}

	var err error
	if params.Typing {
		err = t.sendService.StartTyping(ctx, execCtx.ChatJID)
	} else {
		err = t.sendService.StopTyping(ctx, execCtx.ChatJID)
	}

	if err != nil {
		return tools.ErrorResult(err.Error()), nil
	}

	return tools.SuccessResult(map[string]bool{"typing": params.Typing}), nil
}

// MarkReadTool marks messages as read.
type MarkReadTool struct {
	sendService *send.SendService
}

func NewMarkReadTool(s *send.SendService) *MarkReadTool {
	return &MarkReadTool{sendService: s}
}

func (t *MarkReadTool) Name() string { return "mark_read" }

func (t *MarkReadTool) Description() string {
	return "Mark messages as read up to a specific index"
}

func (t *MarkReadTool) Parameters() json.RawMessage {
	return tools.MustMarshal(tools.ParameterSchema{
		Type: "object",
		Properties: map[string]tools.PropertySchema{
			"message_index": {Type: "integer", Description: "Index of message to mark as read (optional, defaults to current)"},
		},
	})
}

func (t *MarkReadTool) Execute(ctx context.Context, args json.RawMessage, execCtx *tools.ExecutionContext) (*tools.Result, error) {
	var params struct {
		MessageIndex int `json:"message_index"`
	}
	json.Unmarshal(args, &params)

	msgID := execCtx.MessageID
	if params.MessageIndex > 0 {
		if id, ok := execCtx.MessageMap[params.MessageIndex]; ok {
			msgID = id
		}
	}

	err := t.sendService.MarkReadSingle(ctx, execCtx.ChatJID, execCtx.SenderJID, types.MessageID(msgID))
	if err != nil {
		return tools.ErrorResult(err.Error()), nil
	}

	return tools.SuccessResult(map[string]string{"status": "read"}), nil
}

// SetPresenceTool sets online/offline presence.
type SetPresenceTool struct {
	sendService *send.SendService
}

func NewSetPresenceTool(s *send.SendService) *SetPresenceTool {
	return &SetPresenceTool{sendService: s}
}

func (t *SetPresenceTool) Name() string { return "set_presence" }

func (t *SetPresenceTool) Description() string {
	return "Set online or offline presence"
}

func (t *SetPresenceTool) Parameters() json.RawMessage {
	return tools.MustMarshal(tools.ParameterSchema{
		Type: "object",
		Properties: map[string]tools.PropertySchema{
			"available": {Type: "boolean", Description: "true for online, false for offline"},
		},
		Required: []string{"available"},
	})
}

func (t *SetPresenceTool) Execute(ctx context.Context, args json.RawMessage, execCtx *tools.ExecutionContext) (*tools.Result, error) {
	var params struct {
		Available bool `json:"available"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return tools.ErrorResult("invalid parameters"), nil
	}

	var err error
	if params.Available {
		err = t.sendService.SetAvailable(ctx)
	} else {
		err = t.sendService.SetUnavailable(ctx)
	}

	if err != nil {
		return tools.ErrorResult(err.Error()), nil
	}

	return tools.SuccessResult(map[string]bool{"available": params.Available}), nil
}

// PinMessageTool pins a message.
type PinMessageTool struct {
	sendService *send.SendService
}

func NewPinMessageTool(s *send.SendService) *PinMessageTool {
	return &PinMessageTool{sendService: s}
}

func (t *PinMessageTool) Name() string { return "pin_message" }

func (t *PinMessageTool) Description() string {
	return "Pin a message by index"
}

func (t *PinMessageTool) Parameters() json.RawMessage {
	return tools.MustMarshal(tools.ParameterSchema{
		Type: "object",
		Properties: map[string]tools.PropertySchema{
			"message_index": {Type: "integer", Description: "Index of message to pin"},
		},
		Required: []string{"message_index"},
	})
}

func (t *PinMessageTool) Execute(ctx context.Context, args json.RawMessage, execCtx *tools.ExecutionContext) (*tools.Result, error) {
	var params struct {
		MessageIndex int `json:"message_index"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return tools.ErrorResult("invalid parameters"), nil
	}

	realMsgID, ok := execCtx.MessageMap[params.MessageIndex]
	if !ok {
		return tools.ErrorResult(fmt.Sprintf("message index %d not found", params.MessageIndex)), nil
	}

	_, err := t.sendService.Pin(ctx, execCtx.ChatJID, types.MessageID(realMsgID), execCtx.SenderJID, false)
	if err != nil {
		return tools.ErrorResult(err.Error()), nil
	}

	return tools.SuccessResult(map[string]string{"status": "pinned"}), nil
}

// UnpinMessageTool unpins a message.
type UnpinMessageTool struct {
	sendService *send.SendService
}

func NewUnpinMessageTool(s *send.SendService) *UnpinMessageTool {
	return &UnpinMessageTool{sendService: s}
}

func (t *UnpinMessageTool) Name() string { return "unpin_message" }

func (t *UnpinMessageTool) Description() string {
	return "Unpin a message by index"
}

func (t *UnpinMessageTool) Parameters() json.RawMessage {
	return tools.MustMarshal(tools.ParameterSchema{
		Type: "object",
		Properties: map[string]tools.PropertySchema{
			"message_index": {Type: "integer", Description: "Index of message to unpin"},
		},
		Required: []string{"message_index"},
	})
}

func (t *UnpinMessageTool) Execute(ctx context.Context, args json.RawMessage, execCtx *tools.ExecutionContext) (*tools.Result, error) {
	var params struct {
		MessageIndex int `json:"message_index"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return tools.ErrorResult("invalid parameters"), nil
	}

	realMsgID, ok := execCtx.MessageMap[params.MessageIndex]
	if !ok {
		return tools.ErrorResult(fmt.Sprintf("message index %d not found", params.MessageIndex)), nil
	}

	_, err := t.sendService.Unpin(ctx, execCtx.ChatJID, types.MessageID(realMsgID), execCtx.SenderJID, false)
	if err != nil {
		return tools.ErrorResult(err.Error()), nil
	}

	return tools.SuccessResult(map[string]string{"status": "unpinned"}), nil
}

// StarMessageTool stars a message.
type StarMessageTool struct {
	sendService *send.SendService
}

func NewStarMessageTool(s *send.SendService) *StarMessageTool {
	return &StarMessageTool{sendService: s}
}

func (t *StarMessageTool) Name() string { return "star_message" }

func (t *StarMessageTool) Description() string {
	return "Star/favorite a message by index"
}

func (t *StarMessageTool) Parameters() json.RawMessage {
	return tools.MustMarshal(tools.ParameterSchema{
		Type: "object",
		Properties: map[string]tools.PropertySchema{
			"message_index": {Type: "integer", Description: "Index of message to star"},
		},
		Required: []string{"message_index"},
	})
}

func (t *StarMessageTool) Execute(ctx context.Context, args json.RawMessage, execCtx *tools.ExecutionContext) (*tools.Result, error) {
	var params struct {
		MessageIndex int `json:"message_index"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return tools.ErrorResult("invalid parameters"), nil
	}

	realMsgID, ok := execCtx.MessageMap[params.MessageIndex]
	if !ok {
		return tools.ErrorResult(fmt.Sprintf("message index %d not found", params.MessageIndex)), nil
	}

	_, err := t.sendService.Star(ctx, execCtx.ChatJID, types.MessageID(realMsgID), execCtx.SenderJID, false)
	if err != nil {
		return tools.ErrorResult(err.Error()), nil
	}

	return tools.SuccessResult(map[string]string{"status": "starred"}), nil
}

// UnstarMessageTool unstars a message.
type UnstarMessageTool struct {
	sendService *send.SendService
}

func NewUnstarMessageTool(s *send.SendService) *UnstarMessageTool {
	return &UnstarMessageTool{sendService: s}
}

func (t *UnstarMessageTool) Name() string { return "unstar_message" }

func (t *UnstarMessageTool) Description() string {
	return "Remove star from a message by index"
}

func (t *UnstarMessageTool) Parameters() json.RawMessage {
	return tools.MustMarshal(tools.ParameterSchema{
		Type: "object",
		Properties: map[string]tools.PropertySchema{
			"message_index": {Type: "integer", Description: "Index of message to unstar"},
		},
		Required: []string{"message_index"},
	})
}

func (t *UnstarMessageTool) Execute(ctx context.Context, args json.RawMessage, execCtx *tools.ExecutionContext) (*tools.Result, error) {
	var params struct {
		MessageIndex int `json:"message_index"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return tools.ErrorResult("invalid parameters"), nil
	}

	realMsgID, ok := execCtx.MessageMap[params.MessageIndex]
	if !ok {
		return tools.ErrorResult(fmt.Sprintf("message index %d not found", params.MessageIndex)), nil
	}

	_, err := t.sendService.Unstar(ctx, execCtx.ChatJID, types.MessageID(realMsgID), execCtx.SenderJID, false)
	if err != nil {
		return tools.ErrorResult(err.Error()), nil
	}

	return tools.SuccessResult(map[string]string{"status": "unstarred"}), nil
}

// RegisterPresenceTools registers presence and operation tools.
func RegisterPresenceTools(registry *tools.Registry, sendService *send.SendService) {
	registry.Register(NewSetTypingTool(sendService))
	registry.Register(NewMarkReadTool(sendService))
	registry.Register(NewSetPresenceTool(sendService))
	registry.Register(NewPinMessageTool(sendService))
	registry.Register(NewUnpinMessageTool(sendService))
	registry.Register(NewStarMessageTool(sendService))
	registry.Register(NewUnstarMessageTool(sendService))
}

var _ tools.Tool = (*SetTypingTool)(nil)
var _ tools.Tool = (*MarkReadTool)(nil)
var _ tools.Tool = (*SetPresenceTool)(nil)
var _ tools.Tool = (*PinMessageTool)(nil)
var _ tools.Tool = (*UnpinMessageTool)(nil)
var _ tools.Tool = (*StarMessageTool)(nil)
var _ tools.Tool = (*UnstarMessageTool)(nil)
