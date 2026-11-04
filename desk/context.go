package desk

import (
	"writer/ext"
	"writer/ext/context/contextapi"
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
