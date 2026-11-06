package aiapi_test

import (
	"context"
	"encoding/json"
	"testing"
	"writer/ext"
	"writer/ext/ai/aiapi"
)

func TestToolSnapshotChecksOldInstanceInsideCall(t *testing.T) {
	for _, read := range []bool{false, true} {
		t.Run(map[bool]string{false: "host", true: "reader"}[read], func(t *testing.T) {
			var reader *ext.PointReader[aiapi.Tool]
			var h *ext.Host
			writes := 0
			restart := false
			stopAfter := false
			tool := aiapi.Tool{Name: "write", Run: func(context.Context, aiapi.ToolEnv, json.RawMessage) aiapi.ToolResult {
				writes++
				return aiapi.ToolResult{Text: "written"}
			}}
			var err error
			h, err = ext.New(ext.Options{}, ext.Module{ID: "ai", Setup: func(r *ext.Registrar) {
				ext.Declare(r, aiapi.Tools, ext.Hidden)
				ext.DeclareCalls(r, aiapi.Tools, aiapi.InvokeTool)
			}}, ext.Module{ID: "lore", Requires: []string{"ai"}, Setup: func(r *ext.Registrar) { ext.Contribute(r, aiapi.Tools, "write", tool) }}, ext.Module{ID: "consumer", Requires: []string{"ai"}, Setup: func(r *ext.Registrar) { reader = ext.ReadPoint(r, aiapi.Tools) }}, ext.Module{ID: "wrapper", Optional: []string{"lore"}, Setup: func(r *ext.Registrar) {
				ext.Around(r, ext.HookOf(ext.CallPoint[aiapi.Tool, aiapi.ToolCall, aiapi.ToolAnswer]{Point: aiapi.Tools}, "lore.write"), "restart", func(in aiapi.ToolCall, next func(aiapi.ToolCall) aiapi.ToolAnswer) aiapi.ToolAnswer {
					if restart {
						if err := h.Set("lore", false); err != nil {
							panic(err)
						}
						if err := h.Set("lore", true); err != nil {
							panic(err)
						}
					}
					answer := next(in)
					if stopAfter {
						if err := h.Set("lore", false); err != nil {
							panic(err)
						}
					}
					return answer
				})
			}})
			if err != nil {
				t.Fatal(err)
			}
			defer h.Close()
			snapshot := func() []aiapi.Tool {
				if read {
					return aiapi.SnapshotReadTools(reader)
				}
				return aiapi.SnapshotTools(h)
			}
			tools := snapshot()
			restart = true
			result := tools[0].Run(context.Background(), aiapi.ToolEnv{}, nil)
			if !result.Failed || writes != 0 {
				t.Fatal(result, writes)
			}
			restart = false
			stopAfter = true
			result = snapshot()[0].Run(context.Background(), aiapi.ToolEnv{}, nil)
			if result.Failed || writes != 1 {
				t.Fatal(result, writes)
			}
		})
	}
}
