// Package llm provides a small OpenAI-compatible client shared by AI features.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"blog/internal/safefetch"
)

// Config describes one OpenAI-compatible endpoint. API keys are never logged.
type Config struct {
	BaseURL string
	APIKey  string
	Model   string
}

// Message is a chat completion message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Client owns the HTTP policy and is safe for concurrent use.
type Client struct {
	HTTPClient *http.Client
}

// New returns a client. A nil HTTP client uses the SSRF-protected production client.
func New(httpClient *http.Client) *Client { return &Client{HTTPClient: httpClient} }

type chatResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

type embeddingResponse struct {
	Data []struct {
		Index     int       `json:"index"`
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// ChatCompletion calls /chat/completions and returns the first choice.
func (c *Client) ChatCompletion(ctx context.Context, cfg Config, messages []Message) (string, error) {
	if strings.TrimSpace(cfg.BaseURL) == "" || strings.TrimSpace(cfg.Model) == "" {
		return "", errors.New("LLM endpoint and model are required")
	}
	payload := struct {
		Model       string    `json:"model"`
		Messages    []Message `json:"messages"`
		Temperature float32   `json:"temperature"`
	}{Model: cfg.Model, Messages: messages, Temperature: 0.2}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	data, status, err := c.doJSON(ctx, cfg, "/chat/completions", body)
	if err != nil {
		return "", err
	}
	var response chatResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return "", fmt.Errorf("response is not JSON (HTTP %d)", status)
	}
	if response.Error != nil && response.Error.Message != "" {
		return "", errors.New(response.Error.Message)
	}
	if status < 200 || status >= 300 {
		return "", fmt.Errorf("HTTP %d", status)
	}
	if len(response.Choices) == 0 {
		return "", errors.New("response has no choices")
	}
	return response.Choices[0].Message.Content, nil
}

// Embed calls /embeddings. Inputs are sent as one batch request.
func (c *Client) Embed(ctx context.Context, cfg Config, inputs []string) ([][]float32, error) {
	if strings.TrimSpace(cfg.BaseURL) == "" || strings.TrimSpace(cfg.Model) == "" {
		return nil, errors.New("embedding endpoint and model are required")
	}
	if len(inputs) == 0 {
		return [][]float32{}, nil
	}
	payload := struct {
		Model string   `json:"model"`
		Input []string `json:"input"`
	}{Model: cfg.Model, Input: inputs}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	data, status, err := c.doJSON(ctx, cfg, "/embeddings", body)
	if err != nil {
		return nil, err
	}
	var response embeddingResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("response is not JSON (HTTP %d)", status)
	}
	if response.Error != nil && response.Error.Message != "" {
		return nil, errors.New(response.Error.Message)
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("HTTP %d", status)
	}
	if len(response.Data) != len(inputs) {
		return nil, fmt.Errorf("embedding response count %d, want %d", len(response.Data), len(inputs))
	}
	out := make([][]float32, len(inputs))
	for i, item := range response.Data {
		idx := item.Index
		if idx < 0 || idx >= len(out) || len(out[idx]) != 0 {
			idx = i
		}
		out[idx] = item.Embedding
	}
	for i := range out {
		if len(out[i]) == 0 {
			return nil, fmt.Errorf("embedding %d is empty", i)
		}
	}
	return out, nil
}

func (c *Client) doJSON(ctx context.Context, cfg Config, path string, body []byte) ([]byte, int, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	endpoint := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/") + path
	client := c.HTTPClient
	if client == nil {
		validated, err := safefetch.ValidateURL(endpoint)
		if err != nil {
			return nil, 0, err
		}
		endpoint = validated.String()
		client = safefetch.New(safefetch.Options{Timeout: 30 * time.Second})
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	request.Header.Set("Content-Type", "application/json")
	if cfg.APIKey != "" {
		request.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, 0, err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return nil, response.StatusCode, err
	}
	return data, response.StatusCode, nil
}
