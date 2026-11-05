package check

import (
	"writer/ext/check/checktext"
	"writer/ext/lore/loreapi"
	"writer/locales"
	"writer/workspace"
)

type noteAction struct {
	label string
	run   func()
}

func (a *Panel) checkActions(it Item) []noteAction {
	leave := noteAction{checktext.CheckNoChange(), func() { a.setMeant(it) }}
	prose := noteAction{checktext.CheckEditText(), func() { a.search.FindInChapter(it.Node, it.F.Quote) }}
	switch it.F.Kind {
	case workspace.FindNew:
		leave.label = locales.CheckIgnore()
		return []noteAction{leave, {checktext.CheckAddToSettings(), func() { a.takeFound(it) }}}
	case workspace.FindConflict:
		if !a.entryNavigationAvailable() {
			return []noteAction{leave, prose}
		}
		return []noteAction{leave, {checktext.CheckEditSetting(), func() {
			a.search.FindInChapter(it.Node, it.F.Quote)
			a.entryNavigation(func(n loreapi.EntryNavigation) { n.Edit(it.Entry) })
			a.editingHost.SavePreferences()
		}}, prose}
	}
	return []noteAction{leave, prose}
}
