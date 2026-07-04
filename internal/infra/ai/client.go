// Package ai 提供 OpenAI-compatible HTTP client。
package ai

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

	domainaidigest "telegram-message-forward/internal/domain/aidigest"
)

type Client interface {
	Generate(ctx context.Context, req GenerateRequest) (*GenerateResult, error)
}

type GenerateRequest struct {
	Model       string
	System      string
	User        string
	Temperature float64
	MaxTokens   int
	Metadata    map[string]string
}

type GenerateResult struct {
	Text         string
	Raw          []byte
	Usage        domainaidigest.TokenUsage
	Model        string
	FinishReason string
}

type OpenAICompatibleConfig struct {
	BaseURL         string
	APIKey          string
	DefaultModel    string
	Timeout         time.Duration
	MaxRetries      int
	Temperature     float64
	DefaultMaxToken int
}

type OpenAICompatibleClient struct {
	cfg  OpenAICompatibleConfig
	http *http.Client
}

func NewOpenAICompatibleClient(cfg OpenAICompatibleConfig) *OpenAICompatibleClient {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.openai.com/v1"
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	if cfg.Timeout <= 0 {
		cfg.Timeout = 60 * time.Second
	}
	return &OpenAICompatibleClient{
		cfg:  cfg,
		http: &http.Client{Timeout: cfg.Timeout},
	}
}

func (c *OpenAICompatibleClient) Generate(ctx context.Context, req GenerateRequest) (*GenerateResult, error) {
	if strings.TrimSpace(c.cfg.APIKey) == "" {
		return nil, errors.New("AI provider 未配置 api key")
	}
	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = c.cfg.DefaultModel
	}
	if model == "" {
		return nil, errors.New("AI provider 未配置模型")
	}
	temperature := req.Temperature
	if temperature == 0 {
		temperature = c.cfg.Temperature
	}
	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = c.cfg.DefaultMaxToken
	}
	body := chatCompletionRequest{
		Model: model,
		Messages: []chatMessage{
			{Role: "system", Content: req.System},
			{Role: "user", Content: req.User},
		},
		Temperature: temperature,
		MaxTokens:   maxTokens,
	}
	attempts := c.cfg.MaxRetries + 1
	if attempts <= 0 {
		attempts = 1
	}
	var lastErr error
	for i := 0; i < attempts; i++ {
		result, err := c.doGenerate(ctx, body)
		if err == nil {
			return result, nil
		}
		lastErr = err
		if !retryable(err) || i == attempts-1 {
			break
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Duration(i+1) * 500 * time.Millisecond):
		}
	}
	return nil, lastErr
}

func (c *OpenAICompatibleClient) doGenerate(ctx context.Context, body chatCompletionRequest) (*GenerateResult, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.BaseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("AI provider 请求失败: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, fmt.Errorf("读取 AI provider 响应失败: %w", err)
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, providerError{kind: "auth", msg: "AI provider 鉴权失败"}
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, providerError{kind: "rate_limit", msg: "AI provider 限流"}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, providerError{kind: "http", msg: fmt.Sprintf("AI provider 返回 HTTP %d", resp.StatusCode)}
	}
	var out chatCompletionResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("解析 AI provider 响应失败: %w", err)
	}
	if len(out.Choices) == 0 || strings.TrimSpace(out.Choices[0].Message.Content) == "" {
		return nil, errors.New("AI provider 响应内容为空")
	}
	return &GenerateResult{
		Text: out.Choices[0].Message.Content,
		Raw:  raw,
		Usage: domainaidigest.TokenUsage{
			PromptTokens:     out.Usage.PromptTokens,
			CompletionTokens: out.Usage.CompletionTokens,
			TotalTokens:      out.Usage.TotalTokens,
		},
		Model:        out.Model,
		FinishReason: out.Choices[0].FinishReason,
	}, nil
}

type providerError struct {
	kind string
	msg  string
}

func (e providerError) Error() string { return e.msg }

func retryable(err error) bool {
	var pe providerError
	if errors.As(err, &pe) {
		return pe.kind == "rate_limit" || pe.kind == "http"
	}
	return false
}

type chatCompletionRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		Message      chatMessage `json:"message"`
		FinishReason string      `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}
