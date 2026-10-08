package model

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCatalogErrorsAndPaginationBounds(t *testing.T) {
	for _, tc := range []struct{ name, protocol, body, want string }{
		{"malformed", "chat", `not-json-secret`, "不是合法 JSON"},
		{"missing data", "chat", `{}`, "缺少 data"},
		{"embedded error", "chat", `{"error":{"message":"not-json-secret"}}`, "返回了错误"},
		{"unknown pagination", "chat", `{"data":[],"has_more":true,"last_id":"a"}`, "分页"},
		{"missing cursor", "anthropic", `{"data":[],"has_more":true}`, "分页"},
		{"looping cursor", "anthropic", `{"data":[],"has_more":true,"last_id":"a"}`, "分页"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, tc.body) }))
			defer server.Close()
			client, err := NewClientWithKey(testProvider(server.URL, tc.protocol), "", "", server.Client())
			if err != nil {
				t.Fatal(err)
			}
			_, err = client.ListModels(context.Background())
			if err == nil || !strings.Contains(err.Error(), tc.want) || strings.Contains(err.Error(), "not-json-secret") {
				t.Fatalf("catalog error: %v", err)
			}
		})
	}
	for _, tc := range []struct {
		name    string
		padding int
		want    string
	}{
		{"page count", 0, "100 页"},
		{"cumulative bytes", bodyLimit / 2, "总计超过 8 MiB"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			page := 0
			padding := strings.Repeat(" ", tc.padding)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				page++
				fmt.Fprintf(w, `{"data":[],"has_more":true,"last_id":"%d"}%s`, page, padding)
			}))
			defer server.Close()
			client, err := NewClientWithKey(testProvider(server.URL, "anthropic"), "", "", server.Client())
			if err != nil {
				t.Fatal(err)
			}
			_, err = client.ListModels(context.Background())
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("catalog bound: %v", err)
			}
		})
	}
}

func TestIncompleteResponsesAreNotUsableText(t *testing.T) {
	for _, tc := range []struct {
		protocol string
		events   []string
	}{
		{"responses", []string{
			event("response.created", `{"type":"response.created","response":{"id":"r1","status":"in_progress","output":[]}}`),
			event("response.output_text.delta", `{"type":"response.output_text.delta","output_index":0,"content_index":0,"item_id":"msg_1","delta":"partial"}`),
			event("response.incomplete", `{"type":"response.incomplete","response":{"id":"r1","status":"incomplete","output":[],"incomplete_details":{"reason":"max_output_tokens"},"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}}`),
		}},
		{"anthropic", []string{
			event("message_start", `{"type":"message_start","message":{"id":"m1","type":"message","role":"assistant","model":"m","content":[],"stop_reason":null,"usage":{"input_tokens":1,"output_tokens":0}}}`),
			event("content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`),
			event("content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"partial"}}`),
			event("content_block_stop", `{"type":"content_block_stop","index":0}`),
			event("message_delta", `{"type":"message_delta","delta":{"stop_reason":"max_tokens","stop_sequence":null},"usage":{"output_tokens":1}}`),
			event("message_stop", `{"type":"message_stop"}`),
		}},
	} {
		t.Run(tc.protocol, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { sse(w, tc.events...) }))
			defer server.Close()
			client, err := NewClientWithKey(testProvider(server.URL, tc.protocol), "model", "", server.Client())
			if err != nil {
				t.Fatal(err)
			}
			got, err := client.Complete(context.Background(), Request{Messages: []Message{{Role: "user", Content: "hi"}}}, nil)
			if err == nil || got.Text != "" {
				t.Fatalf("incomplete output accepted: %q %v", got.Text, err)
			}
		})
	}
}
