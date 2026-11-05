package desk

import (
	"writer/ext/lore/loreapi"
	"writer/locales"
	"writer/workspace"
)

type noteAction struct {
	label string
	run   func()
}

func (a *App) checkActions(it checkItem) []noteAction {
	leave := noteAction{locales.CheckNoChange(), func() { a.setMeant(it) }}
	prose := noteAction{locales.CheckEditText(), func() { a.findInChapter(it.node, it.f.Quote) }}
	switch it.f.Kind {
	case workspace.FindNew:
		leave.label = locales.CheckIgnore()
		return []noteAction{leave, {locales.CheckAddToSettings(), func() { a.takeFound(it) }}}
	case workspace.FindConflict:
		if !a.entryNavigationAvailable() {
			return []noteAction{leave, prose}
		}
		return []noteAction{leave, {locales.CheckEditSetting(), func() {
			a.findInChapter(it.node, it.f.Quote)
			a.entryNavigation(func(n loreapi.EntryNavigation) { n.Edit(it.entry) })
			a.savePrefs()
		}}, prose}
	}
	return []noteAction{leave, prose}
}
