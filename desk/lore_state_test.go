package desk

import (
	"writer/desk/lore"
	"writer/ext"
	"writer/ext/hostapi"
	"writer/workspace"
)

func lorePanel(a *App) *lore.Panel {
	v, ok := ext.Lookup(a.extensions, hostapi.EntryNavigationService)
	if !ok {
		panic("entry UI absent")
	}
	return v.Navigation.(*lore.Panel)
}
func (a *App) describeOp(op workspace.Op) string { return lorePanel(a).DescribeOp(op) }
