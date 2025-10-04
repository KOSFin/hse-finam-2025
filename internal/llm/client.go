package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const defaultBaseURL = "https://api.viberouter.dev/v1"

// Message represents a chat message for the VibeRouter API.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatCompletionRequest represents the payload sent to the VibeRouter chat API.
type ChatCompletionRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	TopP        float64   `json:"top_p,omitempty"`
}

// Choice captures a single completion alternative.
type Choice struct {
	Message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"message"`
	FinishReason string `json:"finish_reason"`
	Index        int    `json:"index"`
}

// ChatCompletionResponse is the subset of the API response we care about.
type ChatCompletionResponse struct {
	Choices []Choice `json:"choices"`
}

// ChatClient captures the ability to perform chat completions.
type ChatClient interface {
	ChatCompletion(ctx context.Context, req ChatCompletionRequest) (*ChatCompletionResponse, error)
}

// Client is a thin wrapper around the VibeRouter REST API.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewClient constructs a client with sane defaults.
func NewClient(apiKey string, opts ...func(*Client)) *Client {
	c := &Client{
		baseURL: defaultBaseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Hour,
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// WithHTTPClient overrides the internal HTTP client.
func WithHTTPClient(hc *http.Client) func(*Client) {
	return func(c *Client) {
		c.httpClient = hc
	}
}

// WithBaseURL overrides the default API base URL (useful for tests).
func WithBaseURL(url string) func(*Client) {
	return func(c *Client) {
		if url != "" {
			c.baseURL = url
		}
	}
}

// ChatCompletion executes a chat completion request against VibeRouter.
func (c *Client) ChatCompletion(ctx context.Context, req ChatCompletionRequest) (*ChatCompletionResponse, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("llm: missing API key")
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("llm: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("llm: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("llm: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("llm: api error %d: %s", resp.StatusCode, string(data))
	}

	var payload ChatCompletionResponse
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(&payload); err != nil {
		return nil, fmt.Errorf("llm: decode response: %w", err)
	}

	return &payload, nil
}
