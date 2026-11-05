package desk

import (
	"writer/ext"
	"writer/ext/lore/loreapi"
)

func (a *App) openTake() {
	if s, ok := ext.Lookup(a.extensions, loreapi.SelectionService); ok {
		_ = ext.Guard(a.extensions, "lore", "lore.selection", s.OpenSelection)
	}
}
func (a *App) takeOpen() bool {
	if s, ok := ext.Lookup(a.extensions, loreapi.SelectionService); ok {
		var open bool
		_ = ext.Guard(a.extensions, "lore", "lore.selection", func() { open = s.SelectionOpen() })
		return open
	}
	return false
}
