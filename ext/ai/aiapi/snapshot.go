package aiapi

import (
	"context"
	"encoding/json"
	"fmt"
	"writer/ext"
)

// SnapshotTools copies active tools on the frame thread and guards callbacks
// with the host's concurrent failure boundary. Failed owners stop immediately;
// task joins and OnStop follow the host's normal deferred cleanup rule.
func SnapshotTools(h *ext.Host) []Tool {
	out := []Tool{}
	if h == nil {
		return out
	}
	for _, item := range ext.Items(h, Tools) {
		tool := item.Value
		owner := item.Owner
		activation := ext.ActivationOf(h, owner)
		run, label := tool.Run, tool.Label
		tool.Run = func(ctx context.Context, e ToolEnv, in json.RawMessage) (result ToolResult) {
			err := ext.Guard(h, owner, "ai.tools."+tool.Name, func() {
				if activation.Active() {
					result = run(ext.WithSession(ctx, activation), e, in)
				}
			})
			if err != nil {
				return ToolResult{Text: fmt.Sprintf("工具调用失败：%v", err), Failed: true}
			}
			return
		}
		tool.Label = func(e ToolEnv, in json.RawMessage) string {
			v := tool.Name
			if label != nil {
				ext.Guard(h, owner, "ai.tools."+tool.Name+".label", func() { v = label(e, in) })
			}
			return v
		}
		out = append(out, tool)
	}
	return out
}

// ToolCall invokes or labels a contributed tool on a worker thread.
type ToolCall struct {
	Context context.Context
	Env     ToolEnv
	Input   json.RawMessage
	Label   bool
}

// ToolAnswer is a tool invocation or label result, without frame objects.
type ToolAnswer struct {
	Result ToolResult
	Label  string
}

// InvokeTool adapts a tool to the host's typed invocation boundary.
func InvokeTool(tool Tool) func(ToolCall) ToolAnswer {
	return func(c ToolCall) ToolAnswer {
		if c.Label {
			label := tool.Name
			if tool.Label != nil {
				label = tool.Label(c.Env, c.Input)
			}
			return ToolAnswer{Label: label}
		}
		return ToolAnswer{Result: tool.Run(c.Context, c.Env, c.Input)}
	}
}

// SnapshotReadTools copies active tools from a recorded dependency reader on the frame thread.
// Returned callbacks invoke the typed host boundary from worker threads.
func SnapshotReadTools(reader *ext.PointReader[Tool]) []Tool {
	out := []Tool{}
	for _, item := range reader.Items() {
		activation := reader.Activation(item.Owner)
		original := item.Value
		tool := original
		tool.Run = func(ctx context.Context, e ToolEnv, in json.RawMessage) ToolResult {
			if !activation.Active() {
				return ToolResult{Failed: true}
			}
			out, err := ext.CallReadPoint(reader, item.ID, ToolCall{Context: ext.WithSession(ctx, activation), Env: e, Input: in}, InvokeTool(original))
			if err != nil {
				return ToolResult{Text: fmt.Sprintf("工具调用失败：%v", err), Failed: true}
			}
			if !activation.Active() {
				return ToolResult{Failed: true}
			}
			return out.Result
		}
		tool.Label = func(e ToolEnv, in json.RawMessage) string {
			if !activation.Active() {
				return tool.Name
			}
			out, err := ext.CallReadPoint(reader, item.ID, ToolCall{Env: e, Input: in, Label: true}, InvokeTool(original))
			if err != nil {
				return tool.Name
			}
			return out.Label
		}
		out = append(out, tool)
	}
	return out
}
