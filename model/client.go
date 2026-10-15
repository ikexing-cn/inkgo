package model

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/zalando/go-keyring"
	"github.com/zendev-sh/goai"
	"github.com/zendev-sh/goai/provider"
	"github.com/zendev-sh/goai/provider/anthropic"
	"github.com/zendev-sh/goai/provider/compat"
	"github.com/zendev-sh/goai/provider/openai"
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
	// EffortNone asks for no reasoning where the provider can turn it off,
	// for a reply wanted at once. It is not offered as a choice.
	EffortNone Effort = "none"
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
	if e == EffortNone {
		return true
	}
	for _, x := range Efforts {
		if e == x {
			return true
		}
	}
	return false
}

// Message is a turn of a conversation. An assistant turn may call tools
// as well as, or instead of, saying something; a "tool" turn is what one
// call returned, and names it by CallID.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Calls   []Call `json:"calls,omitempty"`
	CallID  string `json:"call_id,omitempty"`
}

// Tool is something the model may ask the caller to do, its input
// described by a JSON Schema.
type Tool struct {
	Name, Description string
	Schema            json.RawMessage
}

// Call is a model asking for a tool, its input as the JSON it wrote.
type Call struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

type Request struct {
	System    string
	Messages  []Message
	Tools     []Tool
	MaxTokens int
	Effort    Effort
}

// Delta is a piece of streamed output: of the reasoning or of the reply.
type Delta struct {
	Reasoning, Text string
}

// Reply is a finished answer and the reasoning shown before it. A reply
// with Calls waits for what they return before it says more; the caller
// runs them and asks again.
type Reply struct {
	Text, Reasoning string
	Calls           []Call
	Usage           Usage
}

// Usage is what a request took and gave, as the provider counts it. Input
// counts everything sent, CacheRead and CacheWrite among it; Reasoning is
// part of Output.
type Usage struct {
	Input      int `json:"input"`
	CacheRead  int `json:"cache_read,omitempty"`
	CacheWrite int `json:"cache_write,omitempty"`
	Output     int `json:"output"`
	Reasoning  int `json:"reasoning,omitempty"`
}

// usage counts as Usage does what goai reports: goai leaves cached input
// out of InputTokens.
func usage(u provider.Usage) Usage {
	return Usage{Input: u.InputTokens + u.CacheReadTokens + u.CacheWriteTokens, CacheRead: u.CacheReadTokens,
		CacheWrite: u.CacheWriteTokens, Output: u.OutputTokens, Reasoning: u.ReasoningTokens}
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
	retries  int // for transient failures before a reply starts
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
	return nil, fmt.Errorf("本机没有配置提供商 %q，请在设置中「AI」配置里重新选择", providerID)
}

// NewClientWithKey permits custom transports and credential-free local servers.
// A blank model ID can list models; Complete requires a nonempty model ID.
// The HTTP client is copied with redirects rejected and guard in front of its
// transport; a request's duration is bounded by its context, as a stream
// outlasts any fixed client timeout.
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
	provider.Models = append([]Model(nil), provider.Models...)
	return &Client{provider: provider, modelID: modelID, key: key, http: &hc, retries: 2}, nil
}

// languageModel opens the model through goai. Base URLs end in the API
// version, as /v1, except that goai's Anthropic provider adds it itself.
// goai falls back to OPENAI_API_KEY and ANTHROPIC_API_KEY when given no key,
// so a keyless provider gets a placeholder, which guard replaces anyway.
func (c *Client) languageModel() provider.LanguageModel {
	base := strings.TrimRight(c.provider.BaseURL, "/")
	key := c.key
	if key == "" {
		key = "none"
	}
	switch c.provider.Protocol {
	case "responses":
		return openai.Chat(c.modelID, openai.WithBaseURL(base), openai.WithAPIKey(key), openai.WithHTTPClient(c.http))
	case "anthropic":
		return anthropic.Chat(c.modelID, anthropic.WithBaseURL(strings.TrimSuffix(base, "/v1")), anthropic.WithAPIKey(key),
			anthropic.WithSkipEnvResolve(), anthropic.WithHTTPClient(c.http))
	default:
		return compat.Chat(c.modelID, compat.WithBaseURL(base), compat.WithAPIKey(key), compat.WithHTTPClient(c.http))
	}
}

// effortOptions asks the provider for e in its own words: reasoning_effort
// on Chat Completions (DeepSeek takes low, high, max), reasoning.effort with
// a summary on Responses, and output_config.effort with adaptive thinking
// shown as a summary on Anthropic. EffortNone turns thinking off on Chat
// Completions as DeepSeek takes it and asks for effort none on Responses,
// which DeepSeek and newer OpenAI models take (the default still reasons,
// and minimal too); Anthropic thinks only when asked.
func (c *Client) effortOptions(e Effort) map[string]any {
	if e == EffortDefault {
		return nil
	}
	if e == EffortNone {
		switch c.provider.Protocol {
		case "anthropic":
			return nil
		case "responses":
			return map[string]any{"reasoning_effort": "none"}
		}
		return map[string]any{"thinking": map[string]any{"type": "disabled"}}
	}
	switch c.provider.Protocol {
	case "responses":
		return map[string]any{"reasoning_effort": string(e), "reasoning_summary": "auto"}
	case "anthropic":
		return map[string]any{"effort": string(e), "thinking": map[string]any{"type": "adaptive", "display": "summarized"}}
	default:
		return map[string]any{"reasoning_effort": string(e)}
	}
}

func (c *Client) Complete(ctx context.Context, req Request, onDelta func(Delta)) (Reply, error) {
	if strings.TrimSpace(c.modelID) == "" {
		return Reply{}, errors.New("请填写模型 ID")
	}
	if req.MaxTokens < 0 {
		return Reply{}, errors.New("max tokens 不能为负数")
	}
	if !req.Effort.Valid() {
		return Reply{}, fmt.Errorf("不支持的推理等级 %q", req.Effort)
	}
	if len(req.Messages) == 0 {
		return Reply{}, errors.New("请求至少要有一条消息")
	}
	size := len(req.System)
	messages := make([]provider.Message, 0, len(req.Messages))
	for _, m := range req.Messages {
		msg, n, err := message(m)
		if err != nil {
			return Reply{}, err
		}
		size += n
		if size > bodyLimit {
			return Reply{}, errTooLarge
		}
		messages = append(messages, msg)
	}
	tools := make([]goai.Tool, len(req.Tools))
	for i, t := range req.Tools {
		if t.Name == "" || !json.Valid(t.Schema) {
			return Reply{}, fmt.Errorf("工具 %q 没有名字或参数说明不是 JSON", t.Name)
		}
		size += len(t.Description) + len(t.Schema)
		tools[i] = goai.Tool{Name: t.Name, Description: t.Description, InputSchema: t.Schema}
	}
	if size > bodyLimit {
		return Reply{}, errTooLarge
	}
	tokens := req.MaxTokens
	if tokens == 0 {
		tokens = defaultMaxTokens
	}
	ctx, cancel := context.WithTimeout(ctx, streamTimeout)
	defer cancel()
	opts := []goai.Option{goai.WithMessages(messages...), goai.WithMaxOutputTokens(tokens), goai.WithMaxRetries(c.retries)}
	if req.System != "" {
		opts = append(opts, goai.WithSystem(req.System))
	}
	if eo := c.effortOptions(req.Effort); eo != nil {
		opts = append(opts, goai.WithProviderOptions(eo))
	}
	if len(tools) > 0 {
		// goai runs no tool itself, so a reply that calls one ends there.
		opts = append(opts, goai.WithTools(tools...))
	}
	stream, err := goai.StreamText(ctx, c.languageModel(), opts...)
	if err != nil {
		return Reply{}, safeError(ctx, err)
	}
	var text, reasoning strings.Builder
	var calls []Call
	var used provider.Usage
	finished := false
	for chunk := range stream.Stream() {
		var d Delta
		switch chunk.Type {
		case provider.ChunkReasoning:
			d.Reasoning = chunk.Text
			reasoning.WriteString(chunk.Text)
		case provider.ChunkText:
			d.Text = chunk.Text
			text.WriteString(chunk.Text)
		case provider.ChunkToolCall:
			input := json.RawMessage(chunk.ToolInput)
			if len(input) == 0 {
				input = json.RawMessage("{}")
			}
			if !json.Valid(input) {
				return Reply{}, errors.New("模型调用工具时写的参数不是 JSON")
			}
			calls = append(calls, Call{ID: chunk.ToolCallID, Name: chunk.ToolName, Input: input})
			continue
		case provider.ChunkError:
			if ctx.Err() == nil && text.Len()+reasoning.Len() > 0 {
				return Reply{}, errors.New("模型回复中途断开，没有完成")
			}
			return Reply{}, safeError(ctx, chunk.Error)
		case provider.ChunkStepFinish, provider.ChunkFinish:
			// The stream's last finish has what the whole request used.
			if chunk.Usage != (provider.Usage{}) {
				used = chunk.Usage
			}
			// goai reports why the model stopped on the step's finish; the
			// stream's last finish may carry no reason, and a stream cut off
			// before the model stopped brings none at all.
			switch chunk.FinishReason {
			case "":
			case provider.FinishLength:
				return Reply{}, errors.New("模型回复被截断")
			case provider.FinishContentFilter:
				return Reply{}, errors.New("模型回复被内容过滤")
			case provider.FinishError:
				return Reply{}, errors.New("模型回复没有完成")
			default:
				finished = true
			}
			continue
		default:
			continue
		}
		if text.Len()+reasoning.Len() > bodyLimit {
			return Reply{}, errors.New("模型回复超过 8 MiB")
		}
		if onDelta != nil && d != (Delta{}) {
			onDelta(d)
		}
	}
	if err := ctx.Err(); err != nil {
		return Reply{}, safeError(ctx, err)
	}
	if !finished {
		return Reply{}, errors.New("模型回复没有完成")
	}
	reply := Reply{Text: text.String(), Reasoning: strings.TrimSpace(reasoning.String()), Calls: calls, Usage: usage(used)}
	if strings.TrimSpace(reply.Text) == "" && len(calls) == 0 {
		return Reply{}, errors.New("模型服务没有返回文字")
	}
	return reply, nil
}

// message is m as goai takes it, and how many bytes it sends.
func message(m Message) (provider.Message, int, error) {
	switch m.Role {
	case "user":
		return provider.Message{Role: provider.RoleUser, Content: []provider.Part{{Type: provider.PartText, Text: m.Content}}}, len(m.Content), nil
	case "assistant":
		var parts []provider.Part
		if m.Content != "" || len(m.Calls) == 0 {
			parts = append(parts, provider.Part{Type: provider.PartText, Text: m.Content})
		}
		n := len(m.Content)
		for _, c := range m.Calls {
			parts = append(parts, provider.Part{Type: provider.PartToolCall, ToolCallID: c.ID, ToolName: c.Name, ToolInput: c.Input})
			n += len(c.ID) + len(c.Name) + len(c.Input)
		}
		return provider.Message{Role: provider.RoleAssistant, Content: parts}, n, nil
	case "tool":
		if m.CallID == "" {
			return provider.Message{}, 0, errors.New("工具结果没有对应的调用")
		}
		return provider.Message{Role: provider.RoleTool, Content: []provider.Part{{Type: provider.PartToolResult, ToolCallID: m.CallID, ToolOutput: m.Content}}},
			len(m.Content) + len(m.CallID), nil
	}
	return provider.Message{}, 0, errors.New("消息角色只能是 user、assistant 或 tool；系统指令放在 Request.System")
}

// safeError reports a failed request by cause alone. Provider errors carry
// the response body, and transport errors URLs and headers; none of it is
// shown.
func safeError(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("模型请求超时：%w", context.DeadlineExceeded)
		}
		return fmt.Errorf("模型请求已中止：%w", ctx.Err())
	}
	var api *goai.APIError
	if errors.As(err, &api) {
		if api.StatusCode >= 300 {
			return statusError(api.StatusCode)
		}
		return errors.New("模型服务返回了错误（为保护密钥和内容，不显示原文）")
	}
	var timeout net.Error
	if errors.As(err, &timeout) && timeout.Timeout() {
		return fmt.Errorf("模型请求超时：%w", context.DeadlineExceeded)
	}
	return errors.New("连不上 AI：检查网络和服务地址")
}

// guard sends a request only to the provider's host, with the provider's
// key and nothing else that identifies the author. Model SDKs fill in
// credentials, organizations and custom headers from the environment, which
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

// passes reports whether a header a client set may go out: what the
// protocols need, and SDK retry and platform details.
func passes(header string) bool {
	h := strings.ToLower(header)
	switch h {
	case "accept", "accept-encoding", "content-type", "content-length", "user-agent",
		"anthropic-version", "anthropic-beta", "idempotency-key":
		return true
	}
	return strings.HasPrefix(h, "x-stainless-")
}
