package desk

import (
	"encoding/json"
	"os"
	"writer/ext"
	"writer/ext/hostapi"
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
	a.write.carets, a.write.size = p.Carets, p.TextSize
	a.write.ignored = map[string]bool{}
	for _, k := range p.Ignored {
		a.write.ignored[k] = true
	}
	a.write.ignoredVer++
	a.doc.width, a.doc.tab = p.PanelWidth, paneTab(p.PaneTab)
	raw, _ := os.ReadFile(a.Store.PrefsPath())
	var state map[string]json.RawMessage
	_ = json.Unmarshal(raw, &state)
	for _, item := range ext.Items(a.extensions, hostapi.State) {
		if item.Value.Load != nil {
			_ = ext.Guard(a.extensions, item.Owner, item.ID+".load", func() { item.Value.Load(state) })
		}
	}
	known := false
	for _, tab := range a.paneTabs {
		known = known || tab.id == a.doc.tab
	}
	if !known {
		a.doc.tab = paneMargin
	}

}

func (a *App) savePrefs() error {
	old := prefs.Load(a.Store.PrefsPath())
	p := prefs.Prefs{Shelved: old.Shelved, NoAttach: old.NoAttach, SettingHidden: a.doc.hidden, SideHidden: a.sideHidden, Chapter: a.write.node, Tabs: a.write.tabs, Blank: a.write.blank, Writing: a.leftWriting,
		PanelWidth: a.doc.width, PaneTab: string(a.doc.tab), CheckWhen: old.CheckWhen,
		Carets: a.write.carets, TextSize: a.write.size,
		Ignored: a.write.ignoredKeys()}
	data, _ := json.Marshal(p)
	var state map[string]json.RawMessage
	_ = json.Unmarshal(data, &state)
	for _, item := range ext.Items(a.extensions, hostapi.State) {
		if item.Value.Save != nil {
			_ = ext.Guard(a.extensions, item.Owner, item.ID+".save", func() {
				for k, v := range item.Value.Save() {
					state[k] = v
				}
			})
		}
	}
	data, _ = json.Marshal(state)
	var merged prefs.Prefs
	_ = json.Unmarshal(data, &merged)
	err := merged.Save(a.Store.PrefsPath())
	a.warn(err)
	return err
}
