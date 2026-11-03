package desk

import (
	"github.com/egoist/mygo/ui"
	"writer/locales"
)

type tabEntry[K comparable] struct {
	id       K
	name     func() string
	label    string
	icon     *ui.SVG
	ai, work bool
	fallback bool
	visible  func() bool
	count    func() int
}

func (t tabEntry[K]) available(a *App) bool {
	return (!t.ai || a.aiOn()) && (!t.work || a.Store != nil) && (t.visible == nil || t.visible())
}

func (t tabEntry[K]) title(a *App) string {
	return t.name()
}

func (t tabEntry[K]) number() int {
	if t.count != nil {
		return t.count()
	}
	return 0
}

type tabTable[K comparable] []tabEntry[K]

// The pane and settings fall back to the first available entry. The pane
// keeps the choice; the settings replace it with what is shown.
func (t tabTable[K]) shown(a *App, chosen K) K {
	for _, entry := range t {
		if entry.id == chosen && entry.available(a) {
			return chosen
		}
	}
	for _, entry := range t {
		if entry.fallback && entry.available(a) {
			return entry.id
		}
	}
	for _, entry := range t {
		if entry.available(a) {
			return entry.id
		}
	}
	return chosen
}

func (a *App) initTabs() {
	a.hostPaneTabs = tabTable[paneTab]{
		{id: paneMargin, name: locales.PaneTabNotes},
		{id: paneSetting, name: locales.PaneTabLore, fallback: true, count: func() int { return a.paneCounts().open }},
		{id: paneTalk, name: locales.PaneTabChat, ai: true},
		{id: paneFore, name: locales.PaneTabHooks, count: func() int { return a.paneCounts().waiting }},
	}
	a.paneTabs = a.hostPaneTabs
	a.prefTabs = tabTable[string]{
		{id: "AI", name: func() string { return "AI" }, label: "设置-AI"},
		{id: "外观", name: locales.ViewLook, label: "设置-外观"},
		{id: "提醒与检查", name: locales.SettingsTabRemind, label: "设置-提醒", work: true},
		{id: "备份", name: locales.SettingsTabBackup, label: "设置-备份", work: true},
	}
}

// With no work the talk page is the welcome, even with the AI off.
func (a *App) pageShown() page {
	if a.Store == nil {
		return pageTalk
	}
	if !a.pages[a.page].available(a) {
		return pageWrite
	}
	return a.page
}
