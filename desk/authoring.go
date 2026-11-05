package desk

import (
	"cmp"
	"github.com/egoist/mygo/ui"
	"slices"
	"writer/desk/kit"
	"writer/ext"
	"writer/ext/hostapi"
)

func selectionMenuCall(v hostapi.SelectionMenu) func(string) []hostapi.SelectionAction {
	return func(text string) []hostapi.SelectionAction {
		if v.Build == nil {
			return nil
		}
		return v.Build(text)
	}
}

func (a *App) selectionMenus(m *kit.Menu, text string) { a.selectionMenusAt(m, text, false) }
func (a *App) selectionMenusAt(m *kit.Menu, text string, before bool) {
	a.selectionMenusIn(m, text, "", false, before)
}
func (a *App) selectionMenusGroup(m *kit.Menu, text, group string, separator bool) {
	a.selectionMenusIn(m, text, group, separator, false)
}
func (a *App) selectionMenusIn(m *kit.Menu, text, group string, separator, before bool) {
	started := false
	items := ext.Items(a.extensions, hostapi.SelectionMenus)
	slices.SortStableFunc(items, func(a, b ext.Item[hostapi.SelectionMenu]) int { return cmp.Compare(a.Value.Order, b.Value.Order) })
	for _, item := range items {
		if item.Value.Group != group || group == "" && (item.Value.Order < 0) != before {
			continue
		}
		actions, err := ext.Call(a.extensions, ext.HookOf(ext.CallPoint[hostapi.SelectionMenu, string, []hostapi.SelectionAction]{Point: hostapi.SelectionMenus}, item.ID), text, selectionMenuCall(item.Value))
		if err == nil && len(actions) > 0 {
			if separator && !started {
				m.Separator()
				started = true
			}
			a.selectionActions(m, item.Owner, item.ID, actions)
		}
	}
}

func (a *App) selectionActions(m *kit.Menu, owner, id string, actions []hostapi.SelectionAction) {
	for _, action := range actions {
		if action.Children != nil {
			m.Submenu(action.Label, func(sub *kit.Menu) { a.selectionActions(sub, owner, id, action.Children) })
			continue
		}
		hint := ""
		for _, cmd := range a.extensionCommands() {
			if cmd.id == action.Command {
				hint = cmd.cmd.keys()
				break
			}
		}
		m.Item(action.Label).Hint(hint).Disabled(action.Disabled).Do(func() {
			if action.Run != nil {
				_ = ext.Guard(a.extensions, owner, id, action.Run)
			}
		})
	}
}

func (a *App) besideItems(c *ui.Context) {
	for _, item := range ext.Items(a.extensions, hostapi.BesideItems) {
		body := ui.Column(c).Fill()
		var err error
		body.Children(func() {
			_, err = ext.Call(a.extensions, ext.HookOf(ext.CallPoint[hostapi.DrawItem, *ui.Context, struct{}]{Point: hostapi.BesideItems}, item.ID), c, drawItemCall(item.Value))
		})
		body.Hidden(err != nil)
	}
}
