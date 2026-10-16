package model

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// toolStreams are a reply that calls search for 井, as each protocol
// streams it, having taken 100 tokens, 60 of them from the cache, and
// given 20.
var toolStreams = map[string][]string{
	"chat": {
		chatChunk(`{"role":"assistant","content":null,"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"search","arguments":"{\"q\":"}}]}`, ""),
		chatChunk(`{"tool_calls":[{"index":0,"function":{"arguments":"\"井\"}"}}]}`, "tool_calls"),
		`data: {"id":"c1","object":"chat.completion.chunk","created":1,"model":"m","choices":[],"usage":{"prompt_tokens":100,"completion_tokens":20,"total_tokens":120,"prompt_tokens_details":{"cached_tokens":60},"completion_tokens_details":{"reasoning_tokens":5}}}` + "\n\n",
		"data: [DONE]\n\n",
	},
	"responses": {
		event("response.created", `{"type":"response.created","response":{"id":"r1","status":"in_progress","output":[]}}`),
		event("response.output_item.added", `{"type":"response.output_item.added","output_index":0,"item":{"id":"fc_1","type":"function_call","call_id":"call_1","name":"search","arguments":""}}`),
		event("response.function_call_arguments.delta", `{"type":"response.function_call_arguments.delta","item_id":"fc_1","output_index":0,"delta":"{\"q\":\"井\"}"}`),
		event("response.output_item.done", `{"type":"response.output_item.done","output_index":0,"item":{"id":"fc_1","type":"function_call","call_id":"call_1","name":"search","arguments":"{\"q\":\"井\"}"}}`),
		event("response.completed", `{"type":"response.completed","response":{"id":"r1","status":"completed","output":[{"id":"fc_1","type":"function_call","call_id":"call_1","name":"search","arguments":"{\"q\":\"井\"}"}],"usage":{"input_tokens":100,"input_tokens_details":{"cached_tokens":60},"output_tokens":20,"output_tokens_details":{"reasoning_tokens":5},"total_tokens":120}}}`),
	},
	"anthropic": {
		event("message_start", `{"type":"message_start","message":{"id":"m1","type":"message","role":"assistant","model":"m","content":[],"stop_reason":null,"usage":{"input_tokens":30,"cache_read_input_tokens":60,"cache_creation_input_tokens":10,"output_tokens":0}}}`),
		event("content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"call_1","name":"search","input":{}}}`),
		event("content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"q\":"}}`),
		event("content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"\"井\"}"}}`),
		event("content_block_stop", `{"type":"content_block_stop","index":0}`),
		event("message_delta", `{"type":"message_delta","delta":{"stop_reason":"tool_use","stop_sequence":null},"usage":{"output_tokens":20}}`),
		event("message_stop", `{"type":"message_stop"}`),
	},
}

// A request carries the tools and the calls made before with what they
// returned; a reply that calls a tool comes back as a call, with what the
// provider says it used, its input streamed a piece at a time on the way.
func TestToolCallsAndUsage(t *testing.T) {
	want := map[string]Usage{
		"chat":      {Input: 100, CacheRead: 60, Output: 20, Reasoning: 5},
		"responses": {Input: 100, CacheRead: 60, Output: 20, Reasoning: 5},
		"anthropic": {Input: 100, CacheRead: 60, CacheWrite: 10, Output: 20},
	}
	for _, protocol := range []string{"chat", "responses", "anthropic"} {
		t.Run(protocol, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				raw, _ := io.ReadAll(r.Body)
				body := string(raw)
				for _, s := range []string{`"search"`, "在全书里搜", `"call_0"`, "第3章", `"maxLength"`} {
					if !strings.Contains(body, s) {
						t.Errorf("request lacks %s: %s", s, body)
					}
				}
				sse(w, toolStreams[protocol]...)
			}))
			defer server.Close()
			client, err := NewClientWithKey(testProvider(server.URL, protocol), "m", "secret", server.Client())
			if err != nil {
				t.Fatal(err)
			}
			req := Request{
				Messages: []Message{
					{Role: "user", Content: "井边出过什么事"},
					{Role: "assistant", Calls: []Call{{ID: "call_0", Name: "search", Input: json.RawMessage(`{"q":"井边"}`)}}},
					{Role: "tool", CallID: "call_0", Content: "第3章：井边的灯"},
				},
				Tools: []Tool{{Name: "search", Description: "在全书里搜", Schema: json.RawMessage(`{"type":"object","properties":{"q":{"type":"string","maxLength":40}},"required":["q"]}`)}},
			}
			var streamed string
			got, err := client.Complete(context.Background(), req, func(d Delta) {
				if d.Call.ID != "call_1" || d.Call.Name != "search" {
					t.Errorf("streamed %+v", d)
				}
				streamed += d.Call.Input
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(got.Calls) != 1 || got.Calls[0].ID != "call_1" || got.Calls[0].Name != "search" || string(got.Calls[0].Input) != `{"q":"井"}` {
				t.Fatalf("calls = %+v", got.Calls)
			}
			if streamed != `{"q":"井"}` {
				t.Fatalf("the input streamed as %q", streamed)
			}
			if got.Text != "" || got.Usage != want[protocol] {
				t.Fatalf("reply = %+v", got)
			}
		})
	}
	client, _ := NewClientWithKey(testProvider("http://localhost", "chat"), "m", "", nil)
	for _, req := range []Request{
		{Messages: []Message{{Role: "tool", Content: "x"}}},
		{Messages: []Message{{Role: "user", Content: "x"}}, Tools: []Tool{{Name: "search", Schema: json.RawMessage("{")}}},
	} {
		if _, err := client.Complete(context.Background(), req, nil); err == nil {
			t.Errorf("accepted %+v", req)
		}
	}
}

// Within a reply that calls tools, the thinking of a turn goes back as each
// protocol takes it: as reasoning_content on Chat Completions, as signed
// blocks on Anthropic, and not at all on Responses. A last turn may keep
// the tools and be told to call none.
func TestThinkingGoesBackWithTheCalls(t *testing.T) {
	want := map[string][]string{
		"chat":      {`"reasoning_content":"想过了"`, `"tool_choice":"none"`},
		"anthropic": {`"thinking":"想过了"`, `"signature":"sig"`, `"tool_choice":{"type":"none"}`},
		"responses": {`"tool_choice":"none"`},
	}
	for _, protocol := range []string{"chat", "responses", "anthropic"} {
		t.Run(protocol, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				raw, _ := io.ReadAll(r.Body)
				body := string(raw)
				for _, s := range want[protocol] {
					if !strings.Contains(body, s) {
						t.Errorf("request lacks %s: %s", s, body)
					}
				}
				if protocol == "responses" && strings.Contains(body, "想过了") {
					t.Errorf("the thinking went back as text: %s", body)
				}
				sse(w, streams[protocol]...)
			}))
			defer server.Close()
			client, err := NewClientWithKey(testProvider(server.URL, protocol), "m", "secret", server.Client())
			if err != nil {
				t.Fatal(err)
			}
			req := Request{
				Messages: []Message{
					{Role: "user", Content: "井边出过什么事"},
					{Role: "assistant", Thinking: []Thought{{Text: "想过了", Signature: "sig"}}, Calls: []Call{{ID: "call_0", Name: "search", Input: json.RawMessage(`{"q":"井边"}`)}}},
					{Role: "tool", CallID: "call_0", Content: "第3章：井边的灯"},
				},
				Tools:   []Tool{{Name: "search", Description: "在全书里搜", Schema: json.RawMessage(`{"type":"object","properties":{"q":{"type":"string"}}}`)}},
				NoCalls: true,
			}
			if _, err := client.Complete(context.Background(), req, nil); err != nil {
				t.Fatal(err)
			}
		})
	}
}
