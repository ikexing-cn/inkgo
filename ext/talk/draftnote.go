package talk

import (
	"writer/ext/ai/aiapi"
	"writer/ext/talk/talkapi"
	"writer/workspace"
)

func (a *Panel) draftContext() (talkapi.DraftContext, error) {
	// Preserve the dialog's built-in-tool estimate; the actual request also includes reference tools.
	var tools []aiapi.Tool
	for _, item := range a.tools.Items() {
		if item.Value.Name == workspace.ProposeTool {
			tools = append(tools, item.Value)
		}
	}
	p, err := Preloaded(a.Store, a.material(), workspace.AIDraft, a.selected(), tools...)
	return talkapi.DraftContext{Tokens: p.Tokens, Cards: p.Cards, Previous: p.Previous, VolumeChapters: p.Outlines == talkapi.VolumeChapters}, err
}
