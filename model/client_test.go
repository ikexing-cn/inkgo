package model

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func testProvider(baseURL, protocol string) Provider {
	return Provider{ID: "custom", Name: "Custom", BaseURL: baseURL + "/v1", Protocol: protocol, Models: []string{"catalog-only"}}
}

func sse(w http.ResponseWriter, events ...string) {
	w.Header().Set("Content-Type", "text/event-stream")
	for _, e := range events {
		io.WriteString(w, e)
	}
}

func chatChunk(delta string, finish string) string {
	f := "null"
	if finish != "" {
		f = `"` + finish + `"`
	}
	return fmt.Sprintf("data: {\"id\":\"c1\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"m\",\"choices\":[{\"index\":0,\"delta\":%s,\"finish_reason\":%s}]}\n\n", delta, f)
}

func event(name, data string) string { return "event: " + name + "\ndata: " + data + "\n\n" }

// streams are a reply as each protocol streams it: reasoning, then "hello".
var streams = map[string][]string{
	"chat": {
		chatChunk(`{"role":"assistant","content":null,"reasoning_content":"先想"}`, ""),
		chatChunk(`{"content":null,"reasoning_content":"一下"}`, ""),
		chatChunk(`{"content":"hel","reasoning_content":null}`, ""),
		chatChunk(`{"content":"lo"}`, "stop"),
		"data: [DONE]\n\n",
	},
	"responses": {
		event("response.created", `{"type":"response.created","response":{"id":"r1","status":"in_progress","output":[]}}`),
		event("response.output_item.added", `{"type":"response.output_item.added","output_index":0,"item":{"id":"rs_1","type":"reasoning","summary":[]}}`),
		event("response.reasoning_summary_part.added", `{"type":"response.reasoning_summary_part.added","item_id":"rs_1","output_index":0,"summary_index":0,"part":{"type":"summary_text","text":""}}`),
		event("response.reasoning_summary_text.delta", `{"type":"response.reasoning_summary_text.delta","item_id":"rs_1","output_index":0,"summary_index":0,"delta":"先想一下"}`),
		event("response.output_item.done", `{"type":"response.output_item.done","output_index":0,"item":{"id":"rs_1","type":"reasoning","summary":[{"type":"summary_text","text":"先想一下"}]}}`),
		event("response.output_item.added", `{"type":"response.output_item.added","output_index":1,"item":{"id":"msg_1","type":"message","role":"assistant","status":"in_progress","content":[]}}`),
		event("response.content_part.added", `{"type":"response.content_part.added","output_index":1,"content_index":0,"item_id":"msg_1","part":{"type":"output_text","text":""}}`),
		event("response.output_text.delta", `{"type":"response.output_text.delta","output_index":1,"content_index":0,"item_id":"msg_1","delta":"hel"}`),
		event("response.output_text.delta", `{"type":"response.output_text.delta","output_index":1,"content_index":0,"item_id":"msg_1","delta":"lo"}`),
		event("response.completed", `{"type":"response.completed","response":{"id":"r1","status":"completed","output":[],"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}}`),
	},
	"anthropic": {
		event("message_start", `{"type":"message_start","message":{"id":"m1","type":"message","role":"assistant","model":"m","content":[],"stop_reason":null,"usage":{"input_tokens":1,"output_tokens":0}}}`),
		event("content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":"","signature":""}}`),
		event("content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"先想一下"}}`),
		event("content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"signature_delta","signature":"sig"}}`),
		event("content_block_stop", `{"type":"content_block_stop","index":0}`),
		event("content_block_start", `{"type":"content_block_start","index":1,"content_block":{"type":"text","text":""}}`),
		event("content_block_delta", `{"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"hel"}}`),
		event("content_block_delta", `{"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"lo"}}`),
		event("content_block_stop", `{"type":"content_block_stop","index":1}`),
		event("message_delta", `{"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"output_tokens":3}}`),
		event("message_stop", `{"type":"message_stop"}`),
	},
}

var routes = map[string]string{"chat": "/v1/chat/completions", "responses": "/v1/responses", "anthropic": "/v1/messages"}

func TestStreamsReasoningAndReply(t *testing.T) {
	for _, protocol := range []string{"chat", "responses", "anthropic"} {
		t.Run(protocol, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != routes[protocol] {
					t.Errorf("route = %s %s", r.Method, r.URL.Path)
				}
				if protocol == "anthropic" {
					if r.Header.Get("x-api-key") != "secret" || r.Header.Get("Authorization") != "" {
						t.Error("incorrect Anthropic headers")
					}
				} else if r.Header.Get("Authorization") != "Bearer secret" {
					t.Error("incorrect OpenAI headers")
				}
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if body["model"] != "manual-outside-catalog" || body["stream"] != true {
					t.Errorf("payload = %#v", body)
				}
				raw, _ := json.Marshal(body)
				if !strings.Contains(string(raw), "rules") || !strings.Contains(string(raw), "question") {
					t.Errorf("system or messages missing: %s", raw)
				}
				sse(w, streams[protocol]...)
			}))
			defer server.Close()
			client, err := NewClientWithKey(testProvider(server.URL, protocol), "manual-outside-catalog", "secret", server.Client())
			if err != nil {
				t.Fatal(err)
			}
			messages := []Message{{Role: "user", Content: "question"}, {Role: "assistant", Content: "earlier"}, {Role: "user", Content: "again"}}
			var streamed Delta
			got, err := client.Complete(context.Background(), Request{System: "rules", Messages: messages, MaxTokens: 99}, func(d Delta) {
				streamed.Reasoning += d.Reasoning
				streamed.Text += d.Text
			})
			if err != nil || got.Text != "hello" || got.Reasoning != "先想一下" {
				t.Fatalf("Complete = %+v, %v", got, err)
			}
			if streamed.Text != "hello" || strings.TrimSpace(streamed.Reasoning) != "先想一下" {
				t.Fatalf("streamed %+v", streamed)
			}
			if len(messages) != 3 || messages[0].Role != "user" {
				t.Fatal("caller messages mutated")
			}
		})
	}
}

// Effort reaches each protocol in its own words; Default sends nothing.
func TestEffortPerProtocol(t *testing.T) {
	for _, tc := range []struct {
		protocol string
		effort   Effort
		want     []string
		absent   []string
	}{
		{"chat", EffortDefault, nil, []string{"reasoning_effort"}},
		{"chat", EffortMax, []string{`"reasoning_effort":"max"`}, nil},
		{"chat", EffortLow, []string{`"reasoning_effort":"low"`}, nil},
		{"responses", EffortMax, []string{`"effort":"max"`, `"summary":"auto"`}, nil},
		{"anthropic", EffortHigh, []string{`"effort":"high"`, `"type":"adaptive"`}, nil},
		{"anthropic", EffortDefault, nil, []string{"effort"}},
		{"chat", EffortNone, []string{`"thinking":{"type":"disabled"}`}, []string{"reasoning_effort"}},
		{"anthropic", EffortNone, nil, []string{"effort", "thinking"}},
		{"responses", EffortNone, []string{`"effort":"none"`}, []string{"summary", "thinking"}},
	} {
		t.Run(tc.protocol+"-"+tc.effort.Label(), func(t *testing.T) {
			var raw string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				b, _ := io.ReadAll(r.Body)
				raw = string(b)
				sse(w, streams[tc.protocol]...)
			}))
			defer server.Close()
			modelID := map[string]string{"chat": "deepseek-flash", "responses": "gpt-5", "anthropic": "claude-opus-5-5"}[tc.protocol]
			client, err := NewClientWithKey(testProvider(server.URL, tc.protocol), modelID, "", server.Client())
			if err != nil {
				t.Fatal(err)
			}
			if _, err = client.Complete(context.Background(), Request{Messages: []Message{{Role: "user", Content: "hi"}}, Effort: tc.effort}, nil); err != nil {
				t.Fatal(err)
			}
			for _, s := range tc.want {
				if !strings.Contains(raw, s) {
					t.Errorf("missing %s in %s", s, raw)
				}
			}
			for _, s := range tc.absent {
				if strings.Contains(raw, s) {
					t.Errorf("unexpected %s in %s", s, raw)
				}
			}
		})
	}
	client, _ := NewClientWithKey(testProvider("https://example.test", "chat"), "m", "", nil)
	if _, err := client.Complete(context.Background(), Request{Messages: []Message{{Role: "user", Content: "hi"}}, Effort: "medium"}, nil); err == nil {
		t.Fatal("accepted an effort not offered")
	}
}

func TestListModels(t *testing.T) {
	for _, protocol := range []string{"chat", "responses", "anthropic"} {
		t.Run(protocol, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != http.MethodGet || r.URL.Path != "/v1/models" {
					t.Errorf("catalog route: %s %s", r.Method, r.URL.Path)
				}
				if protocol == "anthropic" && calls == 1 {
					io.WriteString(w, `{"data":[{"id":"z"},{"id":"a"}],"has_more":true,"last_id":"a /&"}`)
				} else {
					if protocol == "anthropic" && r.URL.Query().Get("after_id") != "a /&" {
						t.Error("cursor not encoded")
					}
					io.WriteString(w, `{"data":[{"id":"a"},{"id":"z"},{"id":""}]}`)
				}
			}))
			defer server.Close()
			client, err := NewClientWithKey(testProvider(server.URL, protocol), "", "", server.Client())
			if err != nil {
				t.Fatal(err)
			}
			got, err := client.ListModels(context.Background())
			if err != nil || !reflect.DeepEqual(got, []string{"a", "z"}) {
				t.Fatalf("ListModels = %#v %v", got, err)
			}
			if !reflect.DeepEqual(client.provider.Models, []string{"catalog-only"}) {
				t.Fatal("fetch changed manual catalog")
			}
		})
	}
}

func TestHTTPErrorSecrecyAndBounds(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   []string
		want   string
	}{
		{"authentication", 401, []string{`{"error":{"message":"secret-credential"}}`}, "HTTP 401"},
		{"not found", 404, []string{`{"error":{"message":"secret-credential"}}`}, "模型 ID 不对"},
		{"mid-stream error", 200, []string{`data: {"error":{"message":"secret-credential"}}` + "\n\n"}, "返回了错误"},
		{"empty", 200, []string{chatChunk(`{"content":""}`, "stop"), "data: [DONE]\n\n"}, "没有返回文字"},
		{"truncated", 200, []string{chatChunk(`{"content":"partial"}`, "length"), "data: [DONE]\n\n"}, "截断"},
		{"cut off", 200, []string{chatChunk(`{"content":"partial"}`, "")}, "没有完成"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tc.status != 200 {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(tc.status)
					io.WriteString(w, tc.body[0])
					return
				}
				sse(w, tc.body...)
			}))
			defer server.Close()
			client, err := NewClientWithKey(testProvider(server.URL, "chat"), "model", "secret-credential", server.Client())
			if err != nil {
				t.Fatal(err)
			}
			got, err := client.Complete(context.Background(), Request{Messages: []Message{{Role: "user", Content: "hi"}}}, nil)
			if err == nil || !strings.Contains(err.Error(), tc.want) || strings.Contains(err.Error(), "secret-credential") || len(err.Error()) > 256 {
				t.Fatalf("unsafe/unexpected error: %v", err)
			}
			if got.Text != "" {
				t.Fatalf("failed reply returned text %q", got.Text)
			}
		})
	}
}

func TestCancellationAndTimeout(t *testing.T) {
	for _, cancelRequest := range []bool{true, false} {
		t.Run(map[bool]string{true: "cancel", false: "timeout"}[cancelRequest], func(t *testing.T) {
			started := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Reading the body lets net/http observe peer disconnects.
				io.Copy(io.Discard, r.Body)
				close(started)
				<-r.Context().Done()
			}))
			defer server.Close()
			client, err := NewClientWithKey(testProvider(server.URL, "chat"), "model", "", server.Client())
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			if !cancelRequest {
				ctx, cancel = context.WithTimeout(context.Background(), 200*time.Millisecond)
			}
			defer cancel()
			done := make(chan error, 1)
			go func() {
				_, err := client.Complete(ctx, Request{Messages: []Message{{Role: "user", Content: "hi"}}}, nil)
				done <- err
			}()
			select {
			case <-started:
			case <-time.After(2 * time.Second):
				t.Fatal("request did not start")
			}
			if cancelRequest {
				cancel()
			}
			select {
			case err := <-done:
				expected := context.DeadlineExceeded
				if cancelRequest {
					expected = context.Canceled
				}
				if !errors.Is(err, expected) {
					t.Fatalf("error = %v; expected %v", err, expected)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("request did not stop")
			}
		})
	}
}

func TestRedirectsNeverForwardCredentialsOrContent(t *testing.T) {
	var forwarded atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { forwarded.Add(1) }))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/secret-credential", http.StatusTemporaryRedirect)
	}))
	defer source.Close()
	hc := source.Client()
	hc.CheckRedirect = func(*http.Request, []*http.Request) error { return nil }
	for _, protocol := range []string{"chat", "anthropic"} {
		client, err := NewClientWithKey(testProvider(source.URL, protocol), "model", "secret-credential", hc)
		if err != nil {
			t.Fatal(err)
		}
		client.retries = 0
		_, err = client.Complete(context.Background(), Request{Messages: []Message{{Role: "user", Content: "private manuscript"}}}, nil)
		if err == nil || strings.Contains(err.Error(), "secret-credential") {
			t.Fatalf("unsafe redirect error: %v", err)
		}
	}
	if forwarded.Load() != 0 {
		t.Fatal("followed redirect")
	}
	if hc.CheckRedirect(nil, nil) != nil {
		t.Fatal("mutated injected HTTP client")
	}
}

type failingTransport struct{}

func (failingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("secret-credential from transport")
}

func TestTransportErrorAndRequestValidation(t *testing.T) {
	client, err := NewClientWithKey(testProvider("https://example.test", "chat"), "model", "secret-credential", &http.Client{Transport: failingTransport{}})
	if err != nil {
		t.Fatal(err)
	}
	client.retries = 0
	for _, req := range []Request{
		{Messages: []Message{{Role: "user", Content: "hi"}}},
		{Messages: []Message{{Role: "system", Content: "hi"}}},
		{Messages: []Message{{Role: "user", Content: "hi"}}, MaxTokens: -1},
		{Messages: []Message{{Role: "user", Content: strings.Repeat("x", bodyLimit+1)}}},
		{},
	} {
		_, err := client.Complete(context.Background(), req, nil)
		if err == nil || strings.Contains(err.Error(), "secret-credential") {
			t.Fatalf("unsafe/missing error: %v", err)
		}
	}
	_, err = NewClientWithKey(testProvider("https://example.test", "chat"), "model", "bad\r\nkey", nil)
	if err == nil {
		t.Fatal("accepted key with line breaks")
	}
}

// Credentials and headers the SDKs find in the environment never leave:
// only the provider's own key goes, and only to its host.
func TestEnvironmentCredentialsNeverLeak(t *testing.T) {
	for _, name := range []string{"OPENAI_API_KEY", "OPENAI_ORG_ID", "OPENAI_PROJECT_ID", "ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN"} {
		t.Setenv(name, "env-leak")
	}
	t.Setenv("OPENAI_CUSTOM_HEADERS", "X-Leak: env-leak")
	t.Setenv("ANTHROPIC_CUSTOM_HEADERS", "X-Leak: env-leak")
	for _, protocol := range []string{"chat", "responses", "anthropic"} {
		for _, key := range []string{"", "own-key"} {
			t.Run(protocol+"-"+key, func(t *testing.T) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					for name, values := range r.Header {
						for _, v := range values {
							if strings.Contains(v, "env-leak") {
								t.Errorf("leaked %s: %s", name, v)
							}
						}
					}
					auth := r.Header.Get("Authorization") + r.Header.Get("x-api-key")
					if (key == "") != (auth == "") || key != "" && !strings.Contains(auth, key) {
						t.Errorf("credential = %q", auth)
					}
					if r.Method == http.MethodGet {
						io.WriteString(w, `{"data":[]}`)
						return
					}
					sse(w, streams[protocol]...)
				}))
				defer server.Close()
				client, err := NewClientWithKey(testProvider(server.URL, protocol), "model", key, server.Client())
				if err != nil {
					t.Fatal(err)
				}
				if _, err = client.Complete(context.Background(), Request{Messages: []Message{{Role: "user", Content: "hi"}}}, nil); err != nil {
					t.Fatal(err)
				}
				if _, err = client.ListModels(context.Background()); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
