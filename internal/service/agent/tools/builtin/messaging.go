package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"go.mau.fi/whatsmeow/types"

	"orion-agent/internal/service/agent/tools"
	"orion-agent/internal/service/send"
)

// SendTextTool sends a text message.
type SendTextTool struct {
	sendService *send.SendService
}

func NewSendTextTool(s *send.SendService) *SendTextTool {
	return &SendTextTool{sendService: s}
}

func (t *SendTextTool) Name() string { return "send_text" }

func (t *SendTextTool) Description() string {
	return "Send a text message to the current chat"
}

func (t *SendTextTool) Parameters() json.RawMessage {
	return tools.MustMarshal(tools.ParameterSchema{
		Type: "object",
		Properties: map[string]tools.PropertySchema{
			"text": {Type: "string", Description: "The text message to send"},
		},
		Required: []string{"text"},
	})
}

func (t *SendTextTool) Execute(ctx context.Context, args json.RawMessage, execCtx *tools.ExecutionContext) (*tools.Result, error) {
	var params struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return tools.ErrorResult("invalid parameters"), nil
	}

	result, err := t.sendService.Send(ctx, execCtx.ChatJID, send.Text(params.Text))
	if err != nil {
		return tools.ErrorResult(err.Error()), nil
	}

	return tools.SuccessResult(map[string]string{"message_id": string(result.MessageID)}), nil
}

// SendReplyTool sends a reply to a specific message.
type SendReplyTool struct {
	sendService *send.SendService
}

func NewSendReplyTool(s *send.SendService) *SendReplyTool {
	return &SendReplyTool{sendService: s}
}

func (t *SendReplyTool) Name() string { return "send_reply" }

func (t *SendReplyTool) Description() string {
	return "Send a reply to a specific message by index"
}

func (t *SendReplyTool) Parameters() json.RawMessage {
	return tools.MustMarshal(tools.ParameterSchema{
		Type: "object",
		Properties: map[string]tools.PropertySchema{
			"text":          {Type: "string", Description: "The reply text"},
			"message_index": {Type: "integer", Description: "Index of the message to reply to (optional, defaults to current message)"},
		},
		Required: []string{"text"},
	})
}

func (t *SendReplyTool) Execute(ctx context.Context, args json.RawMessage, execCtx *tools.ExecutionContext) (*tools.Result, error) {
	var params struct {
		Text         string `json:"text"`
		MessageIndex int    `json:"message_index"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return tools.ErrorResult("invalid parameters"), nil
	}

	replyToID := execCtx.MessageID
	if params.MessageIndex > 0 {
		if id, ok := execCtx.MessageMap[params.MessageIndex]; ok {
			replyToID = id
		} else {
			return tools.ErrorResult(fmt.Sprintf("message index %d not found", params.MessageIndex)), nil
		}
	}

	result, err := t.sendService.Reply(ctx, execCtx.ChatJID, types.MessageID(replyToID), execCtx.SenderJID, send.Text(params.Text))
	if err != nil {
		return tools.ErrorResult(err.Error()), nil
	}

	return tools.SuccessResult(map[string]string{"message_id": string(result.MessageID)}), nil
}

// EditMessageTool edits a previously sent message.
type EditMessageTool struct {
	sendService *send.SendService
}

func NewEditMessageTool(s *send.SendService) *EditMessageTool {
	return &EditMessageTool{sendService: s}
}

func (t *EditMessageTool) Name() string { return "edit_message" }

func (t *EditMessageTool) Description() string {
	return "Edit a previously sent message by index (only your own messages)"
}

func (t *EditMessageTool) Parameters() json.RawMessage {
	return tools.MustMarshal(tools.ParameterSchema{
		Type: "object",
		Properties: map[string]tools.PropertySchema{
			"message_index": {Type: "integer", Description: "Index of the message to edit"},
			"new_text":      {Type: "string", Description: "The new text content"},
		},
		Required: []string{"message_index", "new_text"},
	})
}

func (t *EditMessageTool) Execute(ctx context.Context, args json.RawMessage, execCtx *tools.ExecutionContext) (*tools.Result, error) {
	var params struct {
		MessageIndex int    `json:"message_index"`
		NewText      string `json:"new_text"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return tools.ErrorResult("invalid parameters"), nil
	}

	realMsgID, ok := execCtx.MessageMap[params.MessageIndex]
	if !ok {
		return tools.ErrorResult(fmt.Sprintf("message index %d not found", params.MessageIndex)), nil
	}

	result, err := t.sendService.Edit(ctx, execCtx.ChatJID, types.MessageID(realMsgID), params.NewText)
	if err != nil {
		return tools.ErrorResult(err.Error()), nil
	}

	return tools.SuccessResult(map[string]string{"message_id": string(result.MessageID)}), nil
}

// RevokeMessageTool deletes a message for everyone.
type RevokeMessageTool struct {
	sendService *send.SendService
}

func NewRevokeMessageTool(s *send.SendService) *RevokeMessageTool {
	return &RevokeMessageTool{sendService: s}
}

func (t *RevokeMessageTool) Name() string { return "revoke_message" }

func (t *RevokeMessageTool) Description() string {
	return "Delete a message for everyone by index (revoke)"
}

func (t *RevokeMessageTool) Parameters() json.RawMessage {
	return tools.MustMarshal(tools.ParameterSchema{
		Type: "object",
		Properties: map[string]tools.PropertySchema{
			"message_index": {Type: "integer", Description: "Index of the message to revoke"},
		},
		Required: []string{"message_index"},
	})
}

func (t *RevokeMessageTool) Execute(ctx context.Context, args json.RawMessage, execCtx *tools.ExecutionContext) (*tools.Result, error) {
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

	_, err := t.sendService.RevokeOwn(ctx, execCtx.ChatJID, types.MessageID(realMsgID))
	if err != nil {
		return tools.ErrorResult(err.Error()), nil
	}

	return tools.SuccessResult(map[string]string{"status": "revoked"}), nil
}

// RegisterMessagingTools registers all messaging tools.
func RegisterMessagingTools(registry *tools.Registry, sendService *send.SendService) {
	registry.Register(NewSendTextTool(sendService))
	registry.Register(NewSendReplyTool(sendService))
	registry.Register(NewEditMessageTool(sendService))
	registry.Register(NewRevokeMessageTool(sendService))
}

var _ tools.Tool = (*SendTextTool)(nil)
var _ tools.Tool = (*SendReplyTool)(nil)
var _ tools.Tool = (*EditMessageTool)(nil)
var _ tools.Tool = (*RevokeMessageTool)(nil)

// Ensure imports are used
var _ = fmt.Sprintf
var _ = strconv.Atoi
