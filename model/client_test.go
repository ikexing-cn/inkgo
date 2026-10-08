package model

import (
	"context"
	"encoding/json"
	"errors"
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

func TestCompleteProtocols(t *testing.T) {
	for _, protocol := range []string{"chat", "responses", "anthropic"} {
		t.Run(protocol, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Errorf("method = %s", r.Method)
				}
				if r.Header.Get("Content-Type") != "application/json" {
					t.Error("missing JSON header")
				}
				if protocol == "anthropic" {
					if r.Header.Get("x-api-key") != "secret" || r.Header.Get("anthropic-version") != "2023-06-01" || r.Header.Get("Authorization") != "" {
						t.Error("incorrect Anthropic headers")
					}
				} else if r.Header.Get("Authorization") != "Bearer secret" || r.Header.Get("x-api-key") != "" {
					t.Error("incorrect OpenAI headers")
				}
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if body["model"] != "manual-outside-catalog" || body["stream"] != false {
					t.Errorf("payload = %#v", body)
				}
				switch protocol {
				case "chat":
					if r.URL.Path != "/v1/chat/completions" || body["max_tokens"] != float64(99) {
						t.Error("incorrect chat route/token budget")
					}
					messages := body["messages"].([]any)
					if len(messages) != 3 || messages[0].(map[string]any)["role"] != "system" || messages[0].(map[string]any)["content"] != "rules" {
						t.Error("incorrect system message")
					}
					io.WriteString(w, `{"choices":[{"message":{"content":"hello"},"finish_reason":"stop"}]}`)
				case "responses":
					if r.URL.Path != "/v1/responses" || body["instructions"] != "rules" || body["max_output_tokens"] != float64(99) || body["store"] != false {
						t.Errorf("incorrect Responses payload: %#v", body)
					}
					if len(body["input"].([]any)) != 2 {
						t.Error("missing input messages")
					}
					io.WriteString(w, `{"status":"completed","output":[{"type":"reasoning","content":[]},{"type":"message","content":[{"type":"output_text","text":"hel"},{"type":"output_text","text":"lo"}]}]}`)
				case "anthropic":
					if r.URL.Path != "/v1/messages" || body["system"] != "rules" || body["max_tokens"] != float64(99) {
						t.Error("incorrect Anthropic payload")
					}
					if len(body["messages"].([]any)) != 2 {
						t.Error("missing messages")
					}
					io.WriteString(w, `{"content":[{"type":"thinking","thinking":"private"},{"type":"text","text":"hel"},{"type":"text","text":"lo"}],"stop_reason":"end_turn"}`)
				}
			}))
			defer server.Close()
			client, err := NewClientWithKey(testProvider(server.URL, protocol), "manual-outside-catalog", "secret", server.Client())
			if err != nil {
				t.Fatal(err)
			}
			messages := []Message{{Role: "user", Content: "question"}, {Role: "assistant", Content: "earlier"}}
			got, err := client.Complete(context.Background(), Request{System: "rules", Messages: messages, MaxTokens: 99})
			if err != nil || got != "hello" {
				t.Fatalf("Complete = %q, %v", got, err)
			}
			if len(messages) != 2 || messages[0].Role != "user" {
				t.Fatal("caller messages mutated")
			}
		})
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
		body   string
		want   string
	}{
		{"authentication", 401, `{"error":"secret-credential"}`, "HTTP 401"},
		{"huge error", 500, strings.Repeat("secret-credential", 100000), "HTTP 500"},
		{"malformed success", 200, `secret-credential`, "不是合法 JSON"},
		{"embedded error", 200, `{"error":{"message":"secret-credential"}}`, "返回了错误"},
		{"empty", 200, `{"choices":[]}`, "没有返回文字"},
		{"truncated", 200, `{"choices":[{"message":{"content":"partial"},"finish_reason":"length"}]}`, "截断"},
		{"oversize", 200, strings.Repeat("x", bodyLimit+1), "超过 8 MiB"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.status); io.WriteString(w, tc.body) }))
			defer server.Close()
			client, err := NewClientWithKey(testProvider(server.URL, "chat"), "model", "secret-credential", server.Client())
			if err != nil {
				t.Fatal(err)
			}
			_, err = client.Complete(context.Background(), Request{Messages: []Message{{Role: "user", Content: "hi"}}})
			if err == nil || !strings.Contains(err.Error(), tc.want) || strings.Contains(err.Error(), "secret-credential") || len(err.Error()) > 256 {
				t.Fatalf("unsafe/unexpected error: %v", err)
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
			hc := server.Client()
			hc.Timeout = 40 * time.Millisecond
			if cancelRequest {
				hc.Timeout = time.Second
			}
			client, err := NewClientWithKey(testProvider(server.URL, "chat"), "model", "", hc)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() {
				_, err := client.Complete(ctx, Request{Messages: []Message{{Role: "user", Content: "hi"}}})
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
			case <-time.After(2 * time.Second):
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
	client, err := NewClientWithKey(testProvider(source.URL, "anthropic"), "model", "secret-credential", hc)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Complete(context.Background(), Request{Messages: []Message{{Role: "user", Content: "private manuscript"}}})
	if err == nil || strings.Contains(err.Error(), "secret-credential") {
		t.Fatalf("unsafe redirect error: %v", err)
	}
	if forwarded.Load() != 0 {
		t.Fatal("followed redirect")
	}
	if hc.CheckRedirect(nil, nil) != nil || hc.Timeout != 0 {
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
	for _, req := range []Request{
		{Messages: []Message{{Role: "user", Content: "hi"}}},
		{Messages: []Message{{Role: "system", Content: "hi"}}},
		{Messages: []Message{{Role: "user", Content: "hi"}}, MaxTokens: -1},
		{Messages: []Message{{Role: "user", Content: strings.Repeat("x", bodyLimit+1)}}},
		{},
	} {
		_, err := client.Complete(context.Background(), req)
		if err == nil || strings.Contains(err.Error(), "secret-credential") {
			t.Fatalf("unsafe/missing error: %v", err)
		}
	}
	if client.http.Timeout != requestTimeout {
		t.Fatal("missing timeout cap")
	}
	_, err = NewClientWithKey(testProvider("https://example.test", "chat"), "model", "bad\r\nkey", nil)
	if err == nil {
		t.Fatal("accepted key with line breaks")
	}
}
