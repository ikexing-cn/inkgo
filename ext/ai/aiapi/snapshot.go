package aiapi

import (
	"context"
	"encoding/json"
	"fmt"
	"writer/ext"
)

// SnapshotTools copies host-consumed tools on the frame thread. Each returned
// callback uses the same typed call chain as an extension dependency reader.
func SnapshotTools(h *ext.Host) []Tool {
	out := []Tool{}
	if h == nil {
		return out
	}
	for _, item := range ext.Items(h, Tools) {
		original := item.Value
		tool := original
		activation := ext.ActivationOf(h, item.Owner)
		key := ext.HookOf(ext.CallPoint[Tool, ToolCall, ToolAnswer]{Point: Tools}, item.ID)
		tool.Run = func(ctx context.Context, e ToolEnv, in json.RawMessage) ToolResult {
			if !activation.Active() {
				return ToolResult{Failed: true}
			}
			out, err := ext.Call(h, key, ToolCall{Context: ext.WithSession(ctx, activation), Env: e, Input: in}, invokeActiveTool(original, activation))
			if err != nil {
				return ToolResult{Text: fmt.Sprintf("工具调用失败：%v", err), Failed: true}
			}
			return out.Result
		}
		tool.Label = func(e ToolEnv, in json.RawMessage) string {
			out, err := ext.Call(h, key, ToolCall{Env: e, Input: in, Label: true}, invokeActiveTool(original, activation))
			if err != nil {
				return original.Name
			}
			return out.Label
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
			out, err := ext.CallReadPoint(reader, item.ID, ToolCall{Context: ext.WithSession(ctx, activation), Env: e, Input: in}, invokeActiveTool(original, activation))
			if err != nil {
				return ToolResult{Text: fmt.Sprintf("工具调用失败：%v", err), Failed: true}
			}
			return out.Result
		}
		tool.Label = func(e ToolEnv, in json.RawMessage) string {
			if !activation.Active() {
				return tool.Name
			}
			out, err := ext.CallReadPoint(reader, item.ID, ToolCall{Env: e, Input: in, Label: true}, invokeActiveTool(original, activation))
			if err != nil {
				return tool.Name
			}
			return out.Label
		}
		out = append(out, tool)
	}
	return out
}

// Check the snapshot instance inside the original call boundary, including when
// an injection disables and re-enables the provider before invoking next.
func invokeActiveTool(tool Tool, activation *ext.Session) func(ToolCall) ToolAnswer {
	invoke := InvokeTool(tool)
	return func(c ToolCall) ToolAnswer {
		if !activation.Active() {
			return ToolAnswer{Result: ToolResult{Failed: true}, Label: tool.Name}
		}
		return invoke(c)
	}
}
