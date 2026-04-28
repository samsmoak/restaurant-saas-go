// Package client wraps a small set of LLM provider HTTP APIs behind a single
// Client interface. The intent is to avoid pulling provider SDKs as
// dependencies for what is, in this codebase, only a thin "send messages →
// get text back" surface.
//
// Selection is driven by env vars:
//
//	LLM_PROVIDER   "anthropic" | "openai"   (default: "anthropic")
//	LLM_API_KEY    provider API key
//	LLM_MODEL      optional model override
//	LLM_BASE_URL   optional base URL override (lets the openai adapter point
//	               at OpenAI-compatible providers like Groq or Together AI)
//
// FromEnv returns nil when LLM_API_KEY is unset — callers should treat a nil
// client as "AI not configured" and degrade gracefully (per the
// non-negotiable that AI endpoints must never 5xx for missing keys).
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

type CompleteOptions struct {
	System      string
	Temperature float64
	MaxTokens   int
}

// Client is a minimal one-shot completion interface. Streaming, tool calling,
// and multi-modal inputs are intentionally out of scope here.
type Client interface {
	Complete(ctx context.Context, messages []Message, opts CompleteOptions) (string, error)
}

// FromEnv returns a configured Client or nil if LLM_API_KEY is unset.
// Errors are only returned for malformed configuration (e.g. unknown
// provider). A missing key is *not* an error.
func FromEnv() (Client, error) {
	key := strings.TrimSpace(os.Getenv("LLM_API_KEY"))
	if key == "" {
		return nil, nil
	}
	provider := strings.ToLower(strings.TrimSpace(os.Getenv("LLM_PROVIDER")))
	if provider == "" {
		provider = "anthropic"
	}
	model := strings.TrimSpace(os.Getenv("LLM_MODEL"))
	baseURL := strings.TrimSpace(os.Getenv("LLM_BASE_URL"))
	switch provider {
	case "anthropic":
		if model == "" {
			model = "claude-haiku-4-5-20251001"
		}
		if baseURL == "" {
			baseURL = "https://api.anthropic.com"
		}
		return &anthropicClient{apiKey: key, model: model, baseURL: baseURL, http: defaultHTTPClient()}, nil
	case "openai":
		if model == "" {
			model = "gpt-4o-mini"
		}
		if baseURL == "" {
			baseURL = "https://api.openai.com"
		}
		return &openAIClient{apiKey: key, model: model, baseURL: baseURL, http: defaultHTTPClient()}, nil
	default:
		return nil, fmt.Errorf("unknown LLM_PROVIDER %q (expected anthropic | openai)", provider)
	}
}

func defaultHTTPClient() *http.Client {
	return &http.Client{Timeout: 30 * time.Second}
}

// ─── Anthropic ────────────────────────────────────────────────────────────

type anthropicClient struct {
	apiKey  string
	model   string
	baseURL string
	http    *http.Client
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicReq struct {
	Model       string             `json:"model"`
	System      string             `json:"system,omitempty"`
	Messages    []anthropicMessage `json:"messages"`
	MaxTokens   int                `json:"max_tokens"`
	Temperature float64            `json:"temperature,omitempty"`
}

type anthropicResp struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (c *anthropicClient) Complete(ctx context.Context, messages []Message, opts CompleteOptions) (string, error) {
	maxT := opts.MaxTokens
	if maxT <= 0 {
		maxT = 1024
	}
	body := anthropicReq{
		Model:       c.model,
		System:      opts.System,
		MaxTokens:   maxT,
		Temperature: opts.Temperature,
	}
	for _, m := range messages {
		// Anthropic doesn't accept a "system" role inside messages; if a system
		// message slipped through, fold it into the system field.
		if m.Role == RoleSystem {
			if body.System != "" {
				body.System += "\n\n"
			}
			body.System += m.Content
			continue
		}
		body.Messages = append(body.Messages, anthropicMessage{Role: string(m.Role), Content: m.Content})
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/messages", bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("anthropic: %w", err)
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("anthropic: http %d: %s", resp.StatusCode, string(rb))
	}
	var out anthropicResp
	if err := json.Unmarshal(rb, &out); err != nil {
		return "", fmt.Errorf("anthropic: decode: %w", err)
	}
	if out.Error != nil {
		return "", fmt.Errorf("anthropic: %s: %s", out.Error.Type, out.Error.Message)
	}
	for _, p := range out.Content {
		if p.Type == "text" && p.Text != "" {
			return p.Text, nil
		}
	}
	return "", errors.New("anthropic: empty response")
}

// ─── OpenAI-compatible ────────────────────────────────────────────────────

type openAIClient struct {
	apiKey  string
	model   string
	baseURL string
	http    *http.Client
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIReq struct {
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	Temperature float64         `json:"temperature,omitempty"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
}

type openAIResp struct {
	Choices []struct {
		Message openAIMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

func (c *openAIClient) Complete(ctx context.Context, messages []Message, opts CompleteOptions) (string, error) {
	body := openAIReq{
		Model:       c.model,
		Temperature: opts.Temperature,
		MaxTokens:   opts.MaxTokens,
	}
	if opts.System != "" {
		body.Messages = append(body.Messages, openAIMessage{Role: "system", Content: opts.System})
	}
	for _, m := range messages {
		body.Messages = append(body.Messages, openAIMessage{Role: string(m.Role), Content: m.Content})
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("openai: %w", err)
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("openai: http %d: %s", resp.StatusCode, string(rb))
	}
	var out openAIResp
	if err := json.Unmarshal(rb, &out); err != nil {
		return "", fmt.Errorf("openai: decode: %w", err)
	}
	if out.Error != nil {
		return "", fmt.Errorf("openai: %s: %s", out.Error.Type, out.Error.Message)
	}
	if len(out.Choices) == 0 {
		return "", errors.New("openai: empty response")
	}
	return strings.TrimSpace(out.Choices[0].Message.Content), nil
}
