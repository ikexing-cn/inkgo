package model

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/zalando/go-keyring"
)

const (
	requestTimeout   = 2 * time.Minute
	bodyLimit        = 8 << 20
	defaultMaxTokens = 4096
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Request struct {
	System    string
	Messages  []Message
	MaxTokens int
}

type Completer interface {
	Complete(context.Context, Request) (string, error)
}

// Client holds a credential in memory; its unexported fields prevent serialization.
// It is safe for concurrent requests after construction.
type Client struct {
	provider Provider
	modelID  string
	key      string
	http     *http.Client
}

var _ Completer = (*Client)(nil)

func NewClient(cfg Config, sel Selection) (*Client, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}
	for _, p := range cfg.Providers {
		if p.ID != sel.Provider {
			continue
		}
		key, err := loadKey(p, keyring.Get)
		if err != nil {
			return nil, err
		}
		return NewClientWithKey(p, sel.Model, key, nil)
	}
	return nil, errors.New("selected provider is not configured on this computer; choose a provider")
}

// NewClientWithKey permits custom transports and credential-free local servers.
// A blank model ID can list models; Complete requires a nonempty model ID.
// The HTTP client is copied, its timeout capped, and all redirects rejected.
func NewClientWithKey(provider Provider, modelID, key string, httpClient *http.Client) (*Client, error) {
	if err := validateProvider(provider); err != nil {
		return nil, err
	}
	if strings.ContainsAny(key, "\r\n") {
		return nil, errors.New("API key must not contain line breaks")
	}
	hc := http.Client{}
	if httpClient != nil {
		hc = *httpClient
	}
	if hc.Timeout <= 0 || hc.Timeout > requestTimeout {
		hc.Timeout = requestTimeout
	}
	hc.CheckRedirect = func(*http.Request, []*http.Request) error { return errors.New("model API redirects are disabled") }
	provider.Models = append([]string(nil), provider.Models...)
	return &Client{provider: provider, modelID: modelID, key: key, http: &hc}, nil
}

func (c *Client) Complete(ctx context.Context, req Request) (string, error) {
	if strings.TrimSpace(c.modelID) == "" {
		return "", errors.New("model ID is required")
	}
	if req.MaxTokens < 0 {
		return "", errors.New("max tokens must not be negative")
	}
	if len(req.Messages) == 0 {
		return "", errors.New("at least one message is required")
	}
	inputBytes := len(req.System)
	for _, m := range req.Messages {
		if m.Role != "user" && m.Role != "assistant" {
			return "", errors.New("message role must be user or assistant; use Request.System for instructions")
		}
		inputBytes += len(m.Content)
		if inputBytes > bodyLimit {
			return "", errors.New("model request exceeds 8 MiB")
		}
	}
	tokens := req.MaxTokens
	if tokens == 0 {
		tokens = defaultMaxTokens
	}
	var endpoint string
	var payload any
	switch c.provider.Protocol {
	case "chat":
		messages := make([]Message, 0, len(req.Messages)+1)
		if req.System != "" {
			messages = append(messages, Message{Role: "system", Content: req.System})
		}
		messages = append(messages, req.Messages...)
		endpoint = "/chat/completions"
		payload = struct {
			Model     string    `json:"model"`
			Messages  []Message `json:"messages"`
			MaxTokens int       `json:"max_tokens"`
			Stream    bool      `json:"stream"`
		}{c.modelID, messages, tokens, false}
	case "responses":
		endpoint = "/responses"
		payload = struct {
			Model           string    `json:"model"`
			Instructions    string    `json:"instructions,omitempty"`
			Input           []Message `json:"input"`
			MaxOutputTokens int       `json:"max_output_tokens"`
			Stream          bool      `json:"stream"`
			Store           bool      `json:"store"`
		}{c.modelID, req.System, req.Messages, tokens, false, false}
	case "anthropic":
		endpoint = "/messages"
		payload = struct {
			Model     string    `json:"model"`
			System    string    `json:"system,omitempty"`
			Messages  []Message `json:"messages"`
			MaxTokens int       `json:"max_tokens"`
			Stream    bool      `json:"stream"`
		}{c.modelID, req.System, req.Messages, tokens, false}
	}
	data, err := c.call(ctx, http.MethodPost, endpoint, payload)
	if err != nil {
		return "", err
	}
	var result struct {
		Error   json.RawMessage `json:"error"`
		Status  string          `json:"status"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Output []struct {
			Type    string `json:"type"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
		StopReason string `json:"stop_reason"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", errors.New("model API returned invalid JSON or an unexpected response shape")
	}
	if len(result.Error) > 0 && string(result.Error) != "null" {
		return "", errors.New("model API reported an error (details omitted to protect credentials and content)")
	}
	var text strings.Builder
	switch c.provider.Protocol {
	case "chat":
		if len(result.Choices) > 0 {
			if reason := result.Choices[0].FinishReason; reason == "length" || reason == "content_filter" {
				return "", errors.New("model response was truncated or filtered")
			}
			text.WriteString(result.Choices[0].Message.Content)
		}
	case "responses":
		if result.Status != "" && result.Status != "completed" {
			return "", errors.New("model response did not complete")
		}
		for _, out := range result.Output {
			if out.Type == "message" {
				for _, part := range out.Content {
					if part.Type == "output_text" {
						text.WriteString(part.Text)
					}
				}
			}
		}
	case "anthropic":
		if result.StopReason == "max_tokens" {
			return "", errors.New("model response was truncated")
		}
		for _, part := range result.Content {
			if part.Type == "text" {
				text.WriteString(part.Text)
			}
		}
	}
	if strings.TrimSpace(text.String()) == "" {
		return "", errors.New("model API returned no text")
	}
	return text.String(), nil
}

// ListModels fetches the remote catalog without modifying the manually configured
// Models list. Manual IDs remain usable even when catalog retrieval is unsupported.
func (c *Client) ListModels(ctx context.Context) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	ids := map[string]bool{}
	cursors := map[string]bool{}
	endpoint := "/models"
	catalogBytes := 0
	for page := 0; page < 100; page++ {
		data, err := c.call(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}
		catalogBytes += len(data)
		if catalogBytes > bodyLimit {
			return nil, errors.New("model catalog exceeds 8 MiB across all pages")
		}
		var result struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
			HasMore bool            `json:"has_more"`
			LastID  string          `json:"last_id"`
			Error   json.RawMessage `json:"error"`
		}
		if err := json.Unmarshal(data, &result); err != nil {
			return nil, errors.New("model catalog returned invalid JSON")
		}
		if len(result.Error) > 0 && string(result.Error) != "null" {
			return nil, errors.New("model catalog API reported an error")
		}
		if result.Data == nil {
			return nil, errors.New("model catalog response is missing data")
		}
		for _, entry := range result.Data {
			if strings.TrimSpace(entry.ID) != "" {
				ids[entry.ID] = true
			}
		}
		if !result.HasMore {
			models := make([]string, 0, len(ids))
			for id := range ids {
				models = append(models, id)
			}
			sort.Strings(models)
			return models, nil
		}
		if c.provider.Protocol != "anthropic" || result.LastID == "" || cursors[result.LastID] {
			return nil, errors.New("model catalog has unsupported or invalid pagination")
		}
		cursors[result.LastID] = true
		endpoint = "/models?after_id=" + url.QueryEscape(result.LastID)
	}
	return nil, errors.New("model catalog exceeds 100 pages")
}

func (c *Client) call(ctx context.Context, method, endpoint string, payload any) ([]byte, error) {
	var body []byte
	if payload != nil {
		var err error
		body, err = json.Marshal(payload)
		if err != nil {
			return nil, errors.New("could not encode model request")
		}
		if len(body) > bodyLimit {
			return nil, errors.New("model request exceeds 8 MiB")
		}
	}
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.provider.BaseURL, "/")+endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, errors.New("could not create model HTTP request")
	}
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.provider.Protocol == "anthropic" {
		req.Header.Set("anthropic-version", "2023-06-01")
		if c.key != "" {
			req.Header.Set("x-api-key", c.key)
		}
	} else if c.key != "" {
		req.Header.Set("Authorization", "Bearer "+c.key)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, safeNetworkError(ctx, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("model API HTTP %d (response details omitted to protect credentials and content)", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, bodyLimit+1))
	if err != nil {
		return nil, safeNetworkError(ctx, err)
	}
	if len(data) > bodyLimit {
		return nil, errors.New("model API response exceeds 8 MiB")
	}
	return data, nil
}

func safeNetworkError(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return fmt.Errorf("model request: %w", ctx.Err())
	}
	var timeout net.Error
	if errors.As(err, &timeout) && timeout.Timeout() {
		return fmt.Errorf("model request: %w", context.DeadlineExceeded)
	}
	// Transport errors can contain URLs, headers, credentials, and remote bodies.
	return errors.New("model HTTP request failed; check the endpoint, connection, and redirect policy")
}
