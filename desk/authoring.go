package desk

import (
	"github.com/egoist/mygo/ui"
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

func (a *App) selectionMenus(m *kit.Menu, text string) {
	for _, item := range ext.Items(a.extensions, hostapi.SelectionMenus) {
		actions, err := ext.Call(a.extensions, ext.HookOf(ext.CallPoint[hostapi.SelectionMenu, string, []hostapi.SelectionAction]{Point: hostapi.SelectionMenus}, item.ID), text, selectionMenuCall(item.Value))
		if err == nil {
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
		m.Item(action.Label).Disabled(action.Disabled).Do(func() {
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
