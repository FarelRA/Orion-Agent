package builtin

import (
	"context"
	"encoding/json"
	"time"

	"orion-agent/internal/service/agent/tools"
	"orion-agent/internal/service/send"
)

// SendLocationTool sends a location message.
type SendLocationTool struct {
	sendService *send.SendService
}

func NewSendLocationTool(s *send.SendService) *SendLocationTool {
	return &SendLocationTool{sendService: s}
}

func (t *SendLocationTool) Name() string { return "send_location" }

func (t *SendLocationTool) Description() string {
	return "Send a location pin"
}

func (t *SendLocationTool) Parameters() json.RawMessage {
	return tools.MustMarshal(tools.ParameterSchema{
		Type: "object",
		Properties: map[string]tools.PropertySchema{
			"latitude":  {Type: "number", Description: "Latitude coordinate"},
			"longitude": {Type: "number", Description: "Longitude coordinate"},
			"name":      {Type: "string", Description: "Location name"},
			"address":   {Type: "string", Description: "Location address"},
		},
		Required: []string{"latitude", "longitude"},
	})
}

func (t *SendLocationTool) Execute(ctx context.Context, args json.RawMessage, execCtx *tools.ExecutionContext) (*tools.Result, error) {
	var params struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
		Name      string  `json:"name"`
		Address   string  `json:"address"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return tools.ErrorResult("invalid parameters"), nil
	}

	content := send.Location(params.Latitude, params.Longitude, params.Name, params.Address)
	result, err := t.sendService.Send(ctx, execCtx.ChatJID, content)
	if err != nil {
		return tools.ErrorResult(err.Error()), nil
	}

	return tools.SuccessResult(map[string]string{"message_id": string(result.MessageID)}), nil
}

// SendContactTool sends a contact card.
type SendContactTool struct {
	sendService *send.SendService
}

func NewSendContactTool(s *send.SendService) *SendContactTool {
	return &SendContactTool{sendService: s}
}

func (t *SendContactTool) Name() string { return "send_contact" }

func (t *SendContactTool) Description() string {
	return "Send a contact card"
}

func (t *SendContactTool) Parameters() json.RawMessage {
	return tools.MustMarshal(tools.ParameterSchema{
		Type: "object",
		Properties: map[string]tools.PropertySchema{
			"name":  {Type: "string", Description: "Contact name"},
			"phone": {Type: "string", Description: "Phone number"},
		},
		Required: []string{"name", "phone"},
	})
}

func (t *SendContactTool) Execute(ctx context.Context, args json.RawMessage, execCtx *tools.ExecutionContext) (*tools.Result, error) {
	var params struct {
		Name  string `json:"name"`
		Phone string `json:"phone"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return tools.ErrorResult("invalid parameters"), nil
	}

	vcard := send.NewVCard(params.Name).CellPhone(params.Phone)
	content := vcard.ToContact()

	result, err := t.sendService.Send(ctx, execCtx.ChatJID, content)
	if err != nil {
		return tools.ErrorResult(err.Error()), nil
	}

	return tools.SuccessResult(map[string]string{"message_id": string(result.MessageID)}), nil
}

// CreatePollTool creates a poll.
type CreatePollTool struct {
	sendService *send.SendService
}

func NewCreatePollTool(s *send.SendService) *CreatePollTool {
	return &CreatePollTool{sendService: s}
}

func (t *CreatePollTool) Name() string { return "create_poll" }

func (t *CreatePollTool) Description() string {
	return "Create a poll with multiple options"
}

func (t *CreatePollTool) Parameters() json.RawMessage {
	return tools.MustMarshal(tools.ParameterSchema{
		Type: "object",
		Properties: map[string]tools.PropertySchema{
			"question":     {Type: "string", Description: "Poll question"},
			"options":      {Type: "array", Description: "Poll options (array of strings)"},
			"multi_select": {Type: "boolean", Description: "Allow multiple selections"},
		},
		Required: []string{"question", "options"},
	})
}

func (t *CreatePollTool) Execute(ctx context.Context, args json.RawMessage, execCtx *tools.ExecutionContext) (*tools.Result, error) {
	var params struct {
		Question    string   `json:"question"`
		Options     []string `json:"options"`
		MultiSelect bool     `json:"multi_select"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return tools.ErrorResult("invalid parameters"), nil
	}

	var content *send.PollContent
	if params.MultiSelect {
		content = send.PollMultiSelect(params.Question, params.Options, len(params.Options))
	} else {
		content = send.Poll(params.Question, params.Options)
	}

	result, err := t.sendService.Send(ctx, execCtx.ChatJID, content)
	if err != nil {
		return tools.ErrorResult(err.Error()), nil
	}

	return tools.SuccessResult(map[string]string{"message_id": string(result.MessageID)}), nil
}

// CreateEventTool creates an event.
type CreateEventTool struct {
	sendService *send.SendService
}

func NewCreateEventTool(s *send.SendService) *CreateEventTool {
	return &CreateEventTool{sendService: s}
}

func (t *CreateEventTool) Name() string { return "create_event" }

func (t *CreateEventTool) Description() string {
	return "Create a calendar event"
}

func (t *CreateEventTool) Parameters() json.RawMessage {
	return tools.MustMarshal(tools.ParameterSchema{
		Type: "object",
		Properties: map[string]tools.PropertySchema{
			"name":        {Type: "string", Description: "Event name"},
			"description": {Type: "string", Description: "Event description"},
			"start_time":  {Type: "string", Description: "Start time (RFC3339 format)"},
			"end_time":    {Type: "string", Description: "End time (RFC3339 format)"},
		},
		Required: []string{"name", "start_time", "end_time"},
	})
}

func (t *CreateEventTool) Execute(ctx context.Context, args json.RawMessage, execCtx *tools.ExecutionContext) (*tools.Result, error) {
	var params struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		StartTime   string `json:"start_time"`
		EndTime     string `json:"end_time"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return tools.ErrorResult("invalid parameters"), nil
	}

	startTime, err := time.Parse(time.RFC3339, params.StartTime)
	if err != nil {
		return tools.ErrorResult("invalid start_time format"), nil
	}
	endTime, err := time.Parse(time.RFC3339, params.EndTime)
	if err != nil {
		return tools.ErrorResult("invalid end_time format"), nil
	}

	content := send.Event(params.Name, startTime, endTime)
	if params.Description != "" {
		content = content.WithDescription(params.Description)
	}

	result, err := t.sendService.Send(ctx, execCtx.ChatJID, content)
	if err != nil {
		return tools.ErrorResult(err.Error()), nil
	}

	return tools.SuccessResult(map[string]string{"message_id": string(result.MessageID)}), nil
}

// RegisterInteractiveTools registers location, contact, and interactive tools.
func RegisterInteractiveTools(registry *tools.Registry, sendService *send.SendService) {
	registry.Register(NewSendLocationTool(sendService))
	registry.Register(NewSendContactTool(sendService))
	registry.Register(NewCreatePollTool(sendService))
	registry.Register(NewCreateEventTool(sendService))
}

var _ tools.Tool = (*SendLocationTool)(nil)
var _ tools.Tool = (*SendContactTool)(nil)
var _ tools.Tool = (*CreatePollTool)(nil)
var _ tools.Tool = (*CreateEventTool)(nil)
