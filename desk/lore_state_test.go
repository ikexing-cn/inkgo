package desk

import (
	"writer/ext"
	"writer/ext/lore"
	"writer/ext/lore/loreapi"
	"writer/workspace"
)

func lorePanel(a *App) *lore.Panel {
	v, ok := ext.Lookup(a.extensions, loreapi.EntryNavigationService)
	if !ok {
		panic("entry UI absent")
	}
	return v.Navigation.(*lore.Panel)
}
func (a *App) describeOp(op workspace.Op) string { return lorePanel(a).DescribeOp(op) }
