package builtin

import (
	"context"
	"encoding/base64"
	"encoding/json"

	"go.mau.fi/whatsmeow/types"

	"orion-agent/internal/service/agent/tools"
	"orion-agent/internal/service/send"
)

// SendImageTool sends an image message.
type SendImageTool struct {
	sendService *send.SendService
}

func NewSendImageTool(s *send.SendService) *SendImageTool {
	return &SendImageTool{sendService: s}
}

func (t *SendImageTool) Name() string { return "send_image" }

func (t *SendImageTool) Description() string {
	return "Send an image message with optional caption"
}

func (t *SendImageTool) Parameters() json.RawMessage {
	return tools.MustMarshal(tools.ParameterSchema{
		Type: "object",
		Properties: map[string]tools.PropertySchema{
			"data":      {Type: "string", Description: "Base64 encoded image data"},
			"mime_type": {Type: "string", Description: "MIME type (e.g., image/jpeg, image/png)"},
			"caption":   {Type: "string", Description: "Optional caption"},
		},
		Required: []string{"data", "mime_type"},
	})
}

func (t *SendImageTool) Execute(ctx context.Context, args json.RawMessage, execCtx *tools.ExecutionContext) (*tools.Result, error) {
	var params struct {
		Data     string `json:"data"`
		MimeType string `json:"mime_type"`
		Caption  string `json:"caption"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return tools.ErrorResult("invalid parameters"), nil
	}

	data, err := base64.StdEncoding.DecodeString(params.Data)
	if err != nil {
		return tools.ErrorResult("invalid base64 data"), nil
	}

	content := send.Image(data, params.MimeType)
	if params.Caption != "" {
		content = send.ImageWithCaption(data, params.MimeType, params.Caption)
	}

	result, err := t.sendService.Send(ctx, execCtx.ChatJID, content)
	if err != nil {
		return tools.ErrorResult(err.Error()), nil
	}

	return tools.SuccessResult(map[string]string{"message_id": string(result.MessageID)}), nil
}

// SendDocumentTool sends a document.
type SendDocumentTool struct {
	sendService *send.SendService
}

func NewSendDocumentTool(s *send.SendService) *SendDocumentTool {
	return &SendDocumentTool{sendService: s}
}

func (t *SendDocumentTool) Name() string { return "send_document" }

func (t *SendDocumentTool) Description() string {
	return "Send a document/file"
}

func (t *SendDocumentTool) Parameters() json.RawMessage {
	return tools.MustMarshal(tools.ParameterSchema{
		Type: "object",
		Properties: map[string]tools.PropertySchema{
			"data":      {Type: "string", Description: "Base64 encoded file data"},
			"mime_type": {Type: "string", Description: "MIME type"},
			"filename":  {Type: "string", Description: "Filename to display"},
			"caption":   {Type: "string", Description: "Optional caption"},
		},
		Required: []string{"data", "mime_type", "filename"},
	})
}

func (t *SendDocumentTool) Execute(ctx context.Context, args json.RawMessage, execCtx *tools.ExecutionContext) (*tools.Result, error) {
	var params struct {
		Data     string `json:"data"`
		MimeType string `json:"mime_type"`
		Filename string `json:"filename"`
		Caption  string `json:"caption"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return tools.ErrorResult("invalid parameters"), nil
	}

	data, err := base64.StdEncoding.DecodeString(params.Data)
	if err != nil {
		return tools.ErrorResult("invalid base64 data"), nil
	}

	content := send.Document(data, params.MimeType, params.Filename)
	if params.Caption != "" {
		content = send.DocumentWithCaption(data, params.MimeType, params.Filename, params.Caption)
	}

	result, err := t.sendService.Send(ctx, execCtx.ChatJID, content)
	if err != nil {
		return tools.ErrorResult(err.Error()), nil
	}

	return tools.SuccessResult(map[string]string{"message_id": string(result.MessageID)}), nil
}

// ReactTool adds a reaction to a message.
type ReactTool struct {
	sendService *send.SendService
}

func NewReactTool(s *send.SendService) *ReactTool {
	return &ReactTool{sendService: s}
}

func (t *ReactTool) Name() string { return "react" }

func (t *ReactTool) Description() string {
	return "Add a reaction emoji to a message"
}

func (t *ReactTool) Parameters() json.RawMessage {
	return tools.MustMarshal(tools.ParameterSchema{
		Type: "object",
		Properties: map[string]tools.PropertySchema{
			"message_id": {Type: "string", Description: "ID of the message to react to (optional, defaults to current)"},
			"emoji":      {Type: "string", Description: "Reaction emoji (e.g., 👍, ❤️, 😂)"},
		},
		Required: []string{"emoji"},
	})
}

func (t *ReactTool) Execute(ctx context.Context, args json.RawMessage, execCtx *tools.ExecutionContext) (*tools.Result, error) {
	var params struct {
		MessageID string `json:"message_id"`
		Emoji     string `json:"emoji"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return tools.ErrorResult("invalid parameters"), nil
	}

	msgID := params.MessageID
	if msgID == "" {
		msgID = execCtx.MessageID
	}

	_, err := t.sendService.React(ctx, execCtx.ChatJID, types.MessageID(msgID), execCtx.SenderJID, params.Emoji)
	if err != nil {
		return tools.ErrorResult(err.Error()), nil
	}

	return tools.SuccessResult(map[string]string{"status": "reacted"}), nil
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
	return "Remove your reaction from a message"
}

func (t *RemoveReactionTool) Parameters() json.RawMessage {
	return tools.MustMarshal(tools.ParameterSchema{
		Type: "object",
		Properties: map[string]tools.PropertySchema{
			"message_id": {Type: "string", Description: "ID of the message to remove reaction from"},
		},
		Required: []string{"message_id"},
	})
}

func (t *RemoveReactionTool) Execute(ctx context.Context, args json.RawMessage, execCtx *tools.ExecutionContext) (*tools.Result, error) {
	var params struct {
		MessageID string `json:"message_id"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return tools.ErrorResult("invalid parameters"), nil
	}

	_, err := t.sendService.RemoveReaction(ctx, execCtx.ChatJID, types.MessageID(params.MessageID), execCtx.SenderJID)
	if err != nil {
		return tools.ErrorResult(err.Error()), nil
	}

	return tools.SuccessResult(map[string]string{"status": "removed"}), nil
}

// RegisterMediaTools registers all media and reaction tools.
func RegisterMediaTools(registry *tools.Registry, sendService *send.SendService) {
	registry.Register(NewSendImageTool(sendService))
	registry.Register(NewSendDocumentTool(sendService))
	registry.Register(NewReactTool(sendService))
	registry.Register(NewRemoveReactionTool(sendService))
}

var _ tools.Tool = (*SendImageTool)(nil)
var _ tools.Tool = (*SendDocumentTool)(nil)
var _ tools.Tool = (*ReactTool)(nil)
var _ tools.Tool = (*RemoveReactionTool)(nil)
