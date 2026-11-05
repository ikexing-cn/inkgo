package desk

import (
	"fmt"
	"writer/prefs"
)

func (a *App) loadPrefs() {
	p := a.readPrefs(a.Store.PrefsPath())
	if p.Unreadable() {
		a.warn(fmt.Errorf("preferences could not be read; using defaults: %s", a.Store.PrefsPath()))
	}
	a.doc.hidden = p.SettingHidden
	a.sideHidden = p.SideHidden
	a.write.node, a.write.tabs, a.write.blank = p.Chapter, p.Tabs, p.Blank && p.Chapter == ""
	a.leftWriting = p.Writing
	if p.Writing {
		a.page = pageWrite
	}
	a.write.carets, a.write.size = p.Carets, p.TextSize
	a.write.ignored = map[string]bool{}
	for _, k := range p.Ignored {
		a.write.ignored[k] = true
	}
	a.write.ignoredVer++
	a.doc.width, a.doc.tab = p.PanelWidth, paneTab(p.PaneTab)
	known := false
	for _, tab := range a.paneTabs {
		known = known || tab.id == a.doc.tab
	}
	if !known {
		a.doc.tab = paneMargin
	}

}

func (a *App) savePrefs() error {
	a.extensions.SaveStates()
	p := prefs.Prefs{SettingHidden: a.doc.hidden, SideHidden: a.sideHidden, Chapter: a.write.node, Tabs: a.write.tabs, Blank: a.write.blank, Writing: a.leftWriting,
		PanelWidth: a.doc.width, PaneTab: string(a.doc.tab),
		Carets: a.write.carets, TextSize: a.write.size,
		Ignored: a.write.ignoredKeys()}
	old := a.readPrefs(a.Store.PrefsPath())
	p.Extensions = old.Extensions
	err := a.writePrefs(p, a.Store.PrefsPath())
	a.warn(err)
	return err
}

func (a *App) readPrefs(path string) prefs.Prefs {
	p := prefs.Load(path)
	if p.Unreadable() {
		if a.prefsRecovery == nil {
			a.prefsRecovery = map[string]prefs.Prefs{}
		}
		if _, exists := a.prefsRecovery[path]; !exists {
			a.prefsRecovery[path] = p
		}
	}
	return p
}

func (a *App) writePrefs(p prefs.Prefs, path string) error {
	if previous, ok := a.prefsRecovery[path]; ok {
		if err := previous.PreserveUnreadable(path); err != nil {
			return err
		}
		delete(a.prefsRecovery, path)
	}
	return p.Save(path)
}
