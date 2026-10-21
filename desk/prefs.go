package desk

import (
	"maps"
	"slices"
	"strconv"

	"writer/prefs"
)

func (a *App) loadPrefs() {
	p := prefs.Load(a.Store.PrefsPath())
	a.doc.hidden = p.SettingHidden
	a.sideHidden = p.SideHidden
	a.write.node, a.write.tabs, a.write.blank = p.Chapter, p.Tabs, p.Blank && p.Chapter == ""
	a.leftWriting = p.Writing
	if p.Writing {
		a.page = pageWrite
	}
	a.write.autoPredict = p.AutoPredict
	a.write.counter = p.Counter
	a.write.goalText, a.write.carets, a.write.size = "", p.Carets, p.TextSize
	a.write.ignored = map[string]bool{}
	for _, k := range p.Ignored {
		a.write.ignored[k] = true
	}
	a.shelved = map[string]bool{}
	for _, id := range p.Shelved {
		a.shelved[id] = true
	}
	if p.Goal > 0 {
		a.write.goalText = strconv.Itoa(p.Goal)
	}
	a.doc.width, a.doc.tab, a.doc.attach = p.PanelWidth, paneTab(p.PaneTab), !p.NoAttach
	switch a.doc.tab {
	case paneSetting, paneTalk, paneFore, panePhone:
	default:
		a.doc.tab = paneSetting
	}
	a.write.lateAfter, a.write.fore = p.LateAfter, foreState{}
	a.ck.when = checkOnLeave
	if p.CheckWhen == checkOnDemand || p.CheckWhen == checkOnPause {
		a.ck.when = p.CheckWhen
	}
}

func (a *App) savePrefs() {
	shelved := slices.Sorted(maps.Keys(a.shelved))
	p := prefs.Prefs{Shelved: shelved, SettingHidden: a.doc.hidden, SideHidden: a.sideHidden, Chapter: a.write.node, Tabs: a.write.tabs, Blank: a.write.blank, Writing: a.leftWriting, AutoPredict: a.write.autoPredict,
		PanelWidth: a.doc.width, PaneTab: string(a.doc.tab), LateAfter: a.write.lateAfter, CheckWhen: a.ck.when, NoAttach: !a.doc.attach,
		Counter: a.write.counter, Carets: a.write.carets, TextSize: a.write.size,
		Ignored: a.write.ignoredKeys()}
	a.warn(p.Save(a.Store.PrefsPath()))
}
