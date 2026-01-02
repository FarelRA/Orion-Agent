package builtin

import (
	"context"
	"encoding/json"
	"fmt"

	"go.mau.fi/whatsmeow/types"

	"orion-agent/internal/service/agent/tools"
	"orion-agent/internal/service/send"
)

// ReactTool adds a reaction to a message.
type ReactTool struct {
	sendService *send.SendService
}

func NewReactTool(s *send.SendService) *ReactTool {
	return &ReactTool{sendService: s}
}

func (t *ReactTool) Name() string { return "react" }

func (t *ReactTool) Description() string {
	return "Add an emoji reaction to a message by index"
}

func (t *ReactTool) Parameters() json.RawMessage {
	return tools.MustMarshal(tools.ParameterSchema{
		Type: "object",
		Properties: map[string]tools.PropertySchema{
			"emoji":         {Type: "string", Description: "Emoji to react with"},
			"message_index": {Type: "integer", Description: "Index of the message to react to (optional, defaults to current)"},
		},
		Required: []string{"emoji"},
	})
}

func (t *ReactTool) Execute(ctx context.Context, args json.RawMessage, execCtx *tools.ExecutionContext) (*tools.Result, error) {
	var params struct {
		Emoji        string `json:"emoji"`
		MessageIndex int    `json:"message_index"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return tools.ErrorResult("invalid parameters"), nil
	}

	targetMsgID := execCtx.MessageID
	if params.MessageIndex > 0 {
		if id, ok := execCtx.MessageMap[params.MessageIndex]; ok {
			targetMsgID = id
		} else {
			return tools.ErrorResult(fmt.Sprintf("message index %d not found", params.MessageIndex)), nil
		}
	}

	_, err := t.sendService.React(ctx, execCtx.ChatJID, types.MessageID(targetMsgID), execCtx.SenderJID, params.Emoji)
	if err != nil {
		return tools.ErrorResult(err.Error()), nil
	}

	return tools.SuccessResult(map[string]string{"status": "reacted", "emoji": params.Emoji}), nil
}

// RemoveReactionTool removes a reaction from a message.
type RemoveReactionTool struct {
	sendService *send.SendService
}

func NewRemoveReactionTool(s *send.SendService) *RemoveReactionTool {
	return &RemoveReactionTool{sendService: s}
}

func (t *RemoveReactionTool) Name() string { return "remove_reaction" }

func (t *RemoveReactionTool) Description() string {
	return "Remove your reaction from a message by index"
}

func (t *RemoveReactionTool) Parameters() json.RawMessage {
	return tools.MustMarshal(tools.ParameterSchema{
		Type: "object",
		Properties: map[string]tools.PropertySchema{
			"message_index": {Type: "integer", Description: "Index of the message to remove reaction from"},
		},
		Required: []string{"message_index"},
	})
}

func (t *RemoveReactionTool) Execute(ctx context.Context, args json.RawMessage, execCtx *tools.ExecutionContext) (*tools.Result, error) {
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

	_, err := t.sendService.RemoveReaction(ctx, execCtx.ChatJID, types.MessageID(realMsgID), execCtx.SenderJID)
	if err != nil {
		return tools.ErrorResult(err.Error()), nil
	}

	return tools.SuccessResult(map[string]string{"status": "removed"}), nil
}

// RegisterMediaTools registers reaction tools.
// Note: Media sending tools (send_image, send_video, send_audio, send_document, send_sticker)
// are removed because AI cannot provide base64 data.
func RegisterMediaTools(registry *tools.Registry, sendService *send.SendService) {
	registry.Register(NewReactTool(sendService))
	registry.Register(NewRemoveReactionTool(sendService))
}

var _ tools.Tool = (*ReactTool)(nil)
var _ tools.Tool = (*RemoveReactionTool)(nil)
