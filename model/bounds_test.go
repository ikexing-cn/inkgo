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
		{"malformed", "chat", `not-json-secret`, "invalid JSON"},
		{"missing data", "chat", `{}`, "missing data"},
		{"embedded error", "chat", `{"error":{"message":"not-json-secret"}}`, "reported an error"},
		{"unknown pagination", "chat", `{"data":[],"has_more":true,"last_id":"a"}`, "pagination"},
		{"missing cursor", "anthropic", `{"data":[],"has_more":true}`, "pagination"},
		{"looping cursor", "anthropic", `{"data":[],"has_more":true,"last_id":"a"}`, "pagination"},
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
		{"page count", 0, "100 pages"},
		{"cumulative bytes", bodyLimit / 2, "8 MiB across all pages"},
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
	for _, tc := range []struct{ protocol, body string }{
		{"responses", `{"status":"incomplete","output":[{"type":"message","content":[{"type":"output_text","text":"partial"}]}]}`},
		{"anthropic", `{"content":[{"type":"text","text":"partial"}],"stop_reason":"max_tokens"}`},
	} {
		t.Run(tc.protocol, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, tc.body) }))
			defer server.Close()
			client, err := NewClientWithKey(testProvider(server.URL, tc.protocol), "model", "", server.Client())
			if err != nil {
				t.Fatal(err)
			}
			text, err := client.Complete(context.Background(), Request{Messages: []Message{{Role: "user", Content: "hi"}}})
			if err == nil || text != "" {
				t.Fatalf("incomplete output accepted: %q %v", text, err)
			}
		})
	}
}
