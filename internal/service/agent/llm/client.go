package llm

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/shared"
	"github.com/openai/openai-go/shared/constant"

	"orion-agent/internal/infra/config"
)

// Client is an OpenAI SDK-compatible HTTP client.
type Client struct {
	client *openai.Client
	config *config.ModelConfig
}

// NewClient creates a new LLM client.
func NewClient(cfg *config.ModelConfig) *Client {
	opts := []option.RequestOption{}
	if cfg.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(cfg.BaseURL))
	}
	if cfg.APIKey != "" {
		opts = append(opts, option.WithAPIKey(cfg.APIKey))
	}

	cl := openai.NewClient(opts...)
	return &Client{
		client: &cl,
		config: cfg,
	}
}

// Complete sends a chat completion request.
func (c *Client) Complete(ctx context.Context, req *ChatCompletionRequest) (*ChatCompletionResponse, error) {
	// 1. Prepare messages
	var messages []openai.ChatCompletionMessageParamUnion
	for _, msg := range req.Messages {
		switch msg.Role {
		case RoleSystem:
			messages = append(messages, openai.ChatCompletionMessageParamUnion{
				OfSystem: &openai.ChatCompletionSystemMessageParam{
					Role:    constant.System("system"),
					Content: openai.ChatCompletionSystemMessageParamContentUnion{OfString: openai.String(msg.Content)},
					Name:    openai.String(msg.Name),
				},
			})
		case RoleUser:
			messages = append(messages, openai.ChatCompletionMessageParamUnion{
				OfUser: &openai.ChatCompletionUserMessageParam{
					Role:    constant.User("user"),
					Content: openai.ChatCompletionUserMessageParamContentUnion{OfString: openai.String(msg.Content)},
					Name:    openai.String(msg.Name),
				},
			})
		case RoleAssistant:
			m := openai.ChatCompletionAssistantMessageParam{
				Role:    constant.Assistant("assistant"),
				Content: openai.ChatCompletionAssistantMessageParamContentUnion{OfString: openai.String(msg.Content)},
				Name:    openai.String(msg.Name),
			}

			// Handle tool calls
			if len(msg.ToolCalls) > 0 {
				var calls []openai.ChatCompletionMessageToolCallParam
				for _, call := range msg.ToolCalls {
					calls = append(calls, openai.ChatCompletionMessageToolCallParam{
						ID:   call.ID,
						Type: constant.Function("function"),
						Function: openai.ChatCompletionMessageToolCallFunctionParam{
							Name:      call.Function.Name,
							Arguments: call.Function.Arguments,
						},
					})
				}
				m.ToolCalls = calls
			}
			messages = append(messages, openai.ChatCompletionMessageParamUnion{
				OfAssistant: &m,
			})
		case RoleTool:
			messages = append(messages, openai.ChatCompletionMessageParamUnion{
				OfTool: &openai.ChatCompletionToolMessageParam{
					Role:       constant.Tool("tool"),
					Content:    openai.ChatCompletionToolMessageParamContentUnion{OfString: openai.String(msg.Content)},
					ToolCallID: msg.ToolCallID,
				},
			})
		}
	}

	// 2. Prepare tools
	var tools []openai.ChatCompletionToolParam
	for _, t := range req.Tools {
		var params map[string]interface{}
		if len(t.Function.Parameters) > 0 {
			if err := json.Unmarshal(t.Function.Parameters, &params); err != nil {
				return nil, fmt.Errorf("unmarshal function parameters: %w", err)
			}
		}

		tools = append(tools, openai.ChatCompletionToolParam{
			Type: constant.Function("function"),
			Function: shared.FunctionDefinitionParam{
				Name:        t.Function.Name, // primitive string
				Description: openai.String(t.Function.Description),
				Parameters:  shared.FunctionParameters(params),
			},
		})
	}

	// 3. Build params
	params := openai.ChatCompletionNewParams{
		Messages: messages,
		Model:    shared.ChatModel(c.config.Model),
	}

	// Override model if provided in request
	if req.Model != "" {
		params.Model = shared.ChatModel(req.Model)
	}

	// Add tools if present
	if len(tools) > 0 {
		params.Tools = tools
	}
	if req.ToolChoice != nil {
		switch v := req.ToolChoice.(type) {
		case string:
			if v == "none" || v == "auto" || v == "required" {
				params.ToolChoice = openai.ChatCompletionToolChoiceOptionUnionParam{
					OfAuto: openai.String(v),
				}
			}
		case map[string]interface{}:
			// Handle specific tool choice: {"type": "function", "function": {"name": "my_function"}}
			if t, ok := v["type"].(string); ok && t == "function" {
				if fn, ok := v["function"].(map[string]interface{}); ok {
					if name, ok := fn["name"].(string); ok {
						params.ToolChoice = openai.ChatCompletionToolChoiceOptionUnionParam{
							OfChatCompletionNamedToolChoice: &openai.ChatCompletionNamedToolChoiceParam{
								Type: constant.Function("function"),
								Function: openai.ChatCompletionNamedToolChoiceFunctionParam{
									Name: name,
								},
							},
						}
					}
				}
			}
		}
	}

	// 4. Apply all configuration params from c.config (global defaults)
	// We prioritize request-specific params if they existed in ChatCompletionRequest (currently few)
	// mostly we pull from config.
	if c.config.MaxCompletionTokens > 0 {
		params.MaxCompletionTokens = openai.Int(int64(c.config.MaxCompletionTokens))
	} else if c.config.MaxTokens > 0 {
		params.MaxTokens = openai.Int(int64(c.config.MaxTokens))
	}

	if c.config.Temperature != 0 {
		params.Temperature = openai.Float(c.config.Temperature)
	}
	if c.config.TopP != 0 {
		params.TopP = openai.Float(c.config.TopP)
	}
	if c.config.FrequencyPenalty != 0 {
		params.FrequencyPenalty = openai.Float(c.config.FrequencyPenalty)
	}
	if c.config.PresencePenalty != 0 {
		params.PresencePenalty = openai.Float(c.config.PresencePenalty)
	}
	if c.config.N > 0 {
		params.N = openai.Int(int64(c.config.N))
	}
	// Stop sequence handling
	if len(c.config.Stop) > 0 {
		params.Stop = openai.ChatCompletionNewParamsStopUnion{
			OfStringArray: c.config.Stop,
		}
	}
	if c.config.Seed != 0 {
		params.Seed = openai.Int(int64(c.config.Seed))
	}
	if len(c.config.LogitBias) > 0 {
		lb := make(map[string]int64)
		for k, v := range c.config.LogitBias {
			lb[k] = int64(v)
		}
		params.LogitBias = lb
	}
	if c.config.Logprobs {
		params.Logprobs = openai.Bool(true)
		if c.config.TopLogProbs > 0 {
			params.TopLogprobs = openai.Int(int64(c.config.TopLogProbs))
		}
	}
	if c.config.ParallelToolCalls {
		params.ParallelToolCalls = openai.Bool(true)
	}
	if c.config.ResponseFormat != "" {
		switch c.config.ResponseFormat {
		case "json_object":
			params.ResponseFormat = openai.ChatCompletionNewParamsResponseFormatUnion{
				OfJSONObject: &shared.ResponseFormatJSONObjectParam{
					Type: constant.JSONObject("json_object"),
				},
			}
		case "text":
			params.ResponseFormat = openai.ChatCompletionNewParamsResponseFormatUnion{
				OfText: &shared.ResponseFormatTextParam{
					Type: constant.Text("text"),
				},
			}
		}
	}
	if c.config.ReasoningEffort != "" {
		params.ReasoningEffort = shared.ReasoningEffort(c.config.ReasoningEffort)
	}
	if c.config.ServiceTier != "" {
		params.ServiceTier = openai.ChatCompletionNewParamsServiceTier(c.config.ServiceTier)
	}
	if c.config.User != "" {
		params.User = openai.String(c.config.User)
	}
	if len(c.config.Metadata) > 0 {
		params.Metadata = shared.Metadata(c.config.Metadata)
	}
	if len(c.config.Modalities) > 0 {
		params.Modalities = c.config.Modalities
	}
	if c.config.PromptCacheKey != "" {
		params.PromptCacheKey = openai.String(c.config.PromptCacheKey)
	}
	if c.config.SafetyIdentifier != "" {
		params.SafetyIdentifier = openai.String(c.config.SafetyIdentifier)
	}
	if c.config.Store {
		params.Store = openai.Bool(true)
	}
	if c.config.Audio != nil {
		params.Audio = openai.ChatCompletionAudioParam{
			Voice:  openai.ChatCompletionAudioParamVoice(c.config.Audio.Voice),
			Format: openai.ChatCompletionAudioParamFormat(c.config.Audio.Format),
		}
	}
	if c.config.Prediction != nil {
		if c.config.Prediction.Type == "content" {
			params.Prediction = openai.ChatCompletionPredictionContentParam{
				Type:    constant.Content("content"),
				Content: openai.ChatCompletionPredictionContentContentUnionParam{OfString: openai.String(c.config.Prediction.Content)},
			}
		}
	}
	if c.config.StreamOptions != nil {
		params.StreamOptions = openai.ChatCompletionStreamOptionsParam{
			IncludeUsage: openai.Bool(c.config.StreamOptions.IncludeUsage),
		}
	}
	if c.config.WebSearchOptions != nil {
		params.WebSearchOptions = openai.ChatCompletionNewParamsWebSearchOptions{
			SearchContextSize: c.config.WebSearchOptions.SearchContextSize,
			// UserLocation not mapped for now as it's complex
		}
	}

	// 5. Execute request
	resp, err := c.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("create completion: %w", err)
	}

	// 6. Map response back to internal types
	result := &ChatCompletionResponse{
		ID:      resp.ID,
		Object:  string(resp.Object),
		Created: resp.Created,
		Model:   resp.Model,
		Usage: Usage{
			PromptTokens:     int(resp.Usage.PromptTokens),
			CompletionTokens: int(resp.Usage.CompletionTokens),
			TotalTokens:      int(resp.Usage.TotalTokens),
		},
	}

	for _, choice := range resp.Choices {
		c := Choice{
			Index:        int(choice.Index),
			FinishReason: string(choice.FinishReason),
			Message: ChatMessage{
				Role:    string(choice.Message.Role),
				Content: choice.Message.Content,
			},
		}

		// Map tool calls
		if len(choice.Message.ToolCalls) > 0 {
			for _, tc := range choice.Message.ToolCalls {
				c.Message.ToolCalls = append(c.Message.ToolCalls, ToolCall{
					ID:   tc.ID,
					Type: string(tc.Type),
					Function: FunctionCall{
						Name:      tc.Function.Name,
						Arguments: tc.Function.Arguments,
					},
				})
			}
		}

		result.Choices = append(result.Choices, c)
	}

	return result, nil
}

// MaxContext returns the model's max context size.
func (c *Client) MaxContext() int {
	return c.config.MaxContext
}
