package model

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"charm.land/fantasy"
	"charm.land/fantasy/providers/anthropic"
	"charm.land/fantasy/providers/openai"
	"charm.land/fantasy/providers/openaicompat"
	"github.com/zalando/go-keyring"
)

const (
	// streamTimeout bounds one whole request; a reply at maximum effort can
	// think for minutes before its first word.
	streamTimeout    = 15 * time.Minute
	requestTimeout   = 2 * time.Minute // a catalog request
	bodyLimit        = 8 << 20
	defaultMaxTokens = 4096
)

var errTooLarge = errors.New("模型请求超过 8 MiB")

// statusError names the likely cause of an HTTP failure. The response body
// is never shown: some providers echo part of the key or the request.
func statusError(code int) error {
	var hint string
	switch {
	case code == 401 || code == 403:
		hint = "：API key 无效、缺失或没有权限"
	case code == 404:
		hint = "：API 地址或模型 ID 不对"
	case code == 429:
		hint = "：请求太频繁或额度不足"
	case code == 400 || code == 422:
		hint = "：请求被拒绝，常见原因是模型 ID、推理等级或参数不受支持"
	case code >= 500:
		hint = "：服务端出错，稍后重试"
	}
	return fmt.Errorf("模型服务返回 HTTP %d%s", code, hint)
}

// Effort is how hard a model reasons before it answers. The zero value
// leaves the choice to the provider.
type Effort string

const (
	EffortDefault Effort = ""
	EffortLow     Effort = "low"
	EffortHigh    Effort = "high"
	EffortMax     Effort = "max"
)

// Efforts are the levels offered, in order.
var Efforts = []Effort{EffortDefault, EffortLow, EffortHigh, EffortMax}

// Label is the effort's name in the interface.
func (e Effort) Label() string {
	switch e {
	case EffortLow:
		return "Low"
	case EffortHigh:
		return "High"
	case EffortMax:
		return "Max"
	}
	return "Default"
}

// Valid reports whether e is one of Efforts.
func (e Effort) Valid() bool {
	for _, x := range Efforts {
		if e == x {
			return true
		}
	}
	return false
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Request struct {
	System    string
	Messages  []Message
	MaxTokens int
	Effort    Effort
}

// Delta is a piece of streamed output: of the reasoning or of the reply.
type Delta struct {
	Reasoning, Text string
}

// Reply is a finished answer and the reasoning shown before it.
type Reply struct {
	Text, Reasoning string
}

// Completer answers a request, passing each piece of output to onDelta,
// which may be nil, as it arrives.
type Completer interface {
	Complete(ctx context.Context, req Request, onDelta func(Delta)) (Reply, error)
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

// NewClient talks to modelID at the configured provider providerID, with the
// key from the OS keychain or the provider's environment variable.
func NewClient(cfg Config, providerID, modelID string) (*Client, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}
	for _, p := range cfg.Providers {
		if p.ID != providerID {
			continue
		}
		key, err := loadKey(p, keyring.Get)
		if err != nil {
			return nil, err
		}
		return NewClientWithKey(p, modelID, key, nil)
	}
	return nil, fmt.Errorf("本机没有配置提供商 %q，请在模型设置里重新选择", providerID)
}

// NewClientWithKey permits custom transports and credential-free local servers.
// A blank model ID can list models; Complete requires a nonempty model ID.
// The HTTP client is copied with redirects rejected; a request's duration is
// bounded by its context, as a stream outlasts any fixed client timeout.
func NewClientWithKey(provider Provider, modelID, key string, httpClient *http.Client) (*Client, error) {
	if err := validateProvider(provider); err != nil {
		return nil, err
	}
	if strings.ContainsAny(key, "\r\n") {
		return nil, errors.New("API key 不能包含换行")
	}
	hc := http.Client{}
	if httpClient != nil {
		hc = *httpClient
	}
	hc.CheckRedirect = func(*http.Request, []*http.Request) error { return errors.New("模型服务的重定向已禁用") }
	u, err := url.Parse(provider.BaseURL)
	if err != nil {
		return nil, err
	}
	base := hc.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	hc.Transport = guard{base: base, host: u.Host, anthropic: provider.Protocol == "anthropic", key: key}
	provider.Models = append([]string(nil), provider.Models...)
	return &Client{provider: provider, modelID: modelID, key: key, http: &hc}, nil
}

// languageModel opens the provider's model through fantasy. Base URLs end
// in the API version, as /v1, except that the Anthropic SDK adds it itself.
func (c *Client) languageModel(ctx context.Context) (fantasy.LanguageModel, error) {
	base := strings.TrimRight(c.provider.BaseURL, "/")
	var p fantasy.Provider
	var err error
	switch c.provider.Protocol {
	case "chat":
		p, err = openaicompat.New(openaicompat.WithBaseURL(base), openaicompat.WithAPIKey(c.key), openaicompat.WithHTTPClient(c.http))
	case "responses":
		every := func(string) bool { return true }
		p, err = openai.New(openai.WithBaseURL(base), openai.WithAPIKey(c.key), openai.WithHTTPClient(c.http), openai.WithUseResponsesAPI(), openai.WithResponsesAPIFunc(every))
	case "anthropic":
		// Without a key the SDK searches the environment and login profiles
		// and fails when it finds none; a placeholder stops the search, and
		// guard sends no credential either way.
		key := c.key
		if key == "" {
			key = "none"
		}
		p, err = anthropic.New(anthropic.WithBaseURL(strings.TrimSuffix(base, "/v1")), anthropic.WithAPIKey(key), anthropic.WithHTTPClient(c.http))
	default:
		return nil, errors.New("协议必须是 chat、responses 或 anthropic")
	}
	if err != nil {
		return nil, errors.New("无法创建模型客户端")
	}
	lm, err := p.LanguageModel(ctx, c.modelID)
	if err != nil {
		return nil, safeError(ctx, err)
	}
	return lm, nil
}

// effortOptions asks the provider for e. DeepSeek and other compatible
// services take low, high and max. OpenAI's highest is xhigh, and fantasy
// sends it only to models it knows reason (gpt-5, the o series). The
// Anthropic provider turns effort into adaptive thinking with its summary
// shown.
func (c *Client) effortOptions(e Effort) fantasy.ProviderOptions {
	if e == EffortDefault {
		return nil
	}
	switch c.provider.Protocol {
	case "chat":
		re := openai.ReasoningEffort(e)
		return openaicompat.NewProviderOptions(&openaicompat.ProviderOptions{ReasoningEffort: &re})
	case "responses":
		re := openai.ReasoningEffort(e)
		if e == EffortMax {
			re = openai.ReasoningEffortXHigh
		}
		summary := "auto"
		return openai.NewResponsesProviderOptions(&openai.ResponsesProviderOptions{ReasoningEffort: &re, ReasoningSummary: &summary})
	case "anthropic":
		ae := anthropic.Effort(e)
		return anthropic.NewProviderOptions(&anthropic.ProviderOptions{Effort: &ae})
	}
	return nil
}

func (c *Client) Complete(ctx context.Context, req Request, onDelta func(Delta)) (Reply, error) {
	var reply Reply
	if strings.TrimSpace(c.modelID) == "" {
		return reply, errors.New("请填写模型 ID")
	}
	if req.MaxTokens < 0 {
		return reply, errors.New("max tokens 不能为负数")
	}
	if !req.Effort.Valid() {
		return reply, fmt.Errorf("不支持的推理等级 %q", req.Effort)
	}
	if len(req.Messages) == 0 {
		return reply, errors.New("请求至少要有一条消息")
	}
	size := len(req.System)
	var prompt fantasy.Prompt
	if req.System != "" {
		prompt = append(prompt, fantasy.NewSystemMessage(req.System))
	}
	for _, m := range req.Messages {
		size += len(m.Content)
		if size > bodyLimit {
			return reply, errTooLarge
		}
		switch m.Role {
		case "user":
			prompt = append(prompt, fantasy.NewUserMessage(m.Content))
		case "assistant":
			prompt = append(prompt, fantasy.Message{Role: fantasy.MessageRoleAssistant, Content: []fantasy.MessagePart{fantasy.TextPart{Text: m.Content}}})
		default:
			return reply, errors.New("消息角色只能是 user 或 assistant；系统指令放在 Request.System")
		}
	}
	tokens := int64(req.MaxTokens)
	if tokens == 0 {
		tokens = defaultMaxTokens
	}
	ctx, cancel := context.WithTimeout(ctx, streamTimeout)
	defer cancel()
	lm, err := c.languageModel(ctx)
	if err != nil {
		return reply, err
	}
	stream, err := lm.Stream(ctx, fantasy.Call{Prompt: prompt, MaxOutputTokens: &tokens, ProviderOptions: c.effortOptions(req.Effort)})
	if err != nil {
		return reply, safeError(ctx, err)
	}
	var text, reasoning strings.Builder
	finished := false
	for part := range stream {
		var d Delta
		switch part.Type {
		case fantasy.StreamPartTypeReasoningDelta:
			d.Reasoning = part.Delta
			reasoning.WriteString(part.Delta)
		case fantasy.StreamPartTypeTextDelta:
			d.Text = part.Delta
			text.WriteString(part.Delta)
		case fantasy.StreamPartTypeError:
			if ctx.Err() == nil && text.Len()+reasoning.Len() > 0 {
				return reply, errors.New("模型回复中途断开，没有完成")
			}
			return reply, safeError(ctx, part.Error)
		case fantasy.StreamPartTypeFinish:
			finished = true
			switch part.FinishReason {
			case fantasy.FinishReasonLength:
				return reply, errors.New("模型回复被截断")
			case fantasy.FinishReasonContentFilter:
				return reply, errors.New("模型回复被内容过滤")
			case fantasy.FinishReasonError:
				return reply, errors.New("模型回复没有完成")
			}
			continue
		default:
			continue
		}
		if text.Len()+reasoning.Len() > bodyLimit {
			return reply, errors.New("模型回复超过 8 MiB")
		}
		if onDelta != nil && d != (Delta{}) {
			onDelta(d)
		}
	}
	if err := ctx.Err(); err != nil {
		return reply, safeError(ctx, err)
	}
	if !finished {
		return reply, errors.New("模型回复没有完成")
	}
	reply = Reply{Text: text.String(), Reasoning: strings.TrimSpace(reasoning.String())}
	if strings.TrimSpace(reply.Text) == "" {
		return Reply{}, errors.New("模型服务没有返回文字")
	}
	return reply, nil
}

// safeError reports a failed request by cause alone. Provider errors carry
// the request and response bodies, and transport errors URLs and headers;
// none of it is shown.
func safeError(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("模型请求超时：%w", context.DeadlineExceeded)
		}
		return fmt.Errorf("模型请求已中止：%w", ctx.Err())
	}
	var pe *fantasy.ProviderError
	if errors.As(err, &pe) {
		if pe.StatusCode >= 300 {
			return statusError(pe.StatusCode)
		}
		if pe.StatusCode == 0 && pe.ResponseBody == nil && pe.Cause != nil {
			return networkError(pe.Cause)
		}
		return errors.New("模型服务返回了错误（为保护密钥和内容，不显示原文）")
	}
	return networkError(err)
}

func networkError(err error) error {
	var timeout net.Error
	if errors.As(err, &timeout) && timeout.Timeout() {
		return fmt.Errorf("模型请求超时：%w", context.DeadlineExceeded)
	}
	return errors.New("连不上模型服务：请检查 API 地址和网络")
}

// guard sends a request only to the provider's host, with the provider's
// key and nothing else that identifies the author. The SDKs fill in
// credentials, organizations and custom headers from the environment
// (OPENAI_API_KEY, ANTHROPIC_AUTH_TOKEN, OPENAI_CUSTOM_HEADERS, ...), which
// would otherwise reach whatever endpoint the author configured.
type guard struct {
	base      http.RoundTripper
	host      string
	anthropic bool
	key       string
}

func (g guard) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.Host != g.host {
		return nil, errors.New("模型请求的目的地址与配置不符")
	}
	out := r.Clone(r.Context())
	out.Header = http.Header{}
	for k, v := range r.Header {
		if passes(k) {
			out.Header[k] = v
		}
	}
	if g.key != "" {
		if g.anthropic {
			out.Header.Set("x-api-key", g.key)
		} else {
			out.Header.Set("Authorization", "Bearer "+g.key)
		}
	}
	return g.base.RoundTrip(out)
}

// passes reports whether a header the SDKs set may go out: what the
// protocols need, and the SDKs' own retry and platform details.
func passes(header string) bool {
	h := strings.ToLower(header)
	switch h {
	case "accept", "accept-encoding", "content-type", "content-length", "user-agent",
		"anthropic-version", "anthropic-beta", "idempotency-key":
		return true
	}
	return strings.HasPrefix(h, "x-stainless-")
}
