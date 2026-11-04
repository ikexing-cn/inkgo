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
		run, label := tool.Run, tool.Label
		tool.Run = func(ctx context.Context, e ToolEnv, in json.RawMessage) (result ToolResult) {
			err := ext.Guard(h, owner, "ai.tools."+tool.Name, func() { result = run(ctx, e, in) })
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
