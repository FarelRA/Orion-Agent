package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"orion-agent/internal/infra/config"
)

// Client is an OpenAI SDK-compatible HTTP client.
type Client struct {
	httpClient *http.Client
	config     *config.ModelConfig
}

// NewClient creates a new LLM client.
func NewClient(cfg *config.ModelConfig) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 120 * time.Second},
		config:     cfg,
	}
}

// Complete sends a chat completion request.
func (c *Client) Complete(ctx context.Context, req *ChatCompletionRequest) (*ChatCompletionResponse, error) {
	if req.Model == "" {
		req.Model = c.config.Model
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := c.config.BaseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.config.APIKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, errResp.Error.Message)
	}

	var result ChatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// MaxContext returns the model's max context size.
func (c *Client) MaxContext() int {
	return c.config.MaxContext
}
