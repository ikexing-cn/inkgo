package desk

import (
	"writer/ext"
	"writer/ext/ai/aiapi"
	"writer/ext/context/contextapi"
	"writer/workspace"
)

func (a *App) outlineValue(id string) (string, bool) {
	if a.extensions == nil {
		return "", false
	}
	service, ok := ext.Lookup(a.extensions, contextapi.Service)
	if !ok {
		return "", false
	}
	var text string
	var present bool
	err := ext.Guard(a.extensions, "context", "context.outline", func() { text, present = service.Outline(id) })
	return text, err == nil && present
}

func (a *App) chapterOutline(id string) string { text, _ := a.outlineValue(id); return text }

func (a *App) hasOutlines() bool { a.syncExtensions(); return a.extViews.outlines }

func (a *App) entryValues() ([]workspace.Entry, bool) {
	if a.extensions == nil {
		return nil, false
	}
	service, ok := ext.Lookup(a.extensions, contextapi.Service)
	if !ok {
		return nil, false
	}
	var entries []workspace.Entry
	var present bool
	err := ext.Guard(a.extensions, "context", "context.entries", func() { entries, present = service.Entries() })
	return entries, err == nil && present
}

func (a *App) entries() []workspace.Entry { entries, _ := a.entryValues(); return entries }

func (a *App) hasEntries() bool { _, present := a.entryValues(); return present }

func (a *App) checksOn() bool { return a.aiOn() && a.hasEntries() }

// material snapshots provider availability on the frame thread.
func (a *App) material() aiapi.Material {
	var out aiapi.Material
	if a.extensions == nil {
		return out
	}
	service, ok := ext.Lookup(a.extensions, contextapi.Service)
	if !ok {
		return out
	}
	err := ext.Guard(a.extensions, "context", "context.material", func() {
		_, out.Setting = service.Entries()
		out.Outline = service.HasOutlines()
	})
	if err != nil {
		return aiapi.Material{}
	}
	return out
}
