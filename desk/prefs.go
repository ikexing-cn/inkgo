package desk

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"

	"writer/internal/atomicfile"
)

// prefs are the desk's own per-work settings, kept in .writer, which is
// not part of the work.
type prefs struct {
	SettingHidden bool   `json:"setting_hidden"`
	SideHidden    bool   `json:"side_hidden,omitempty"` // the sidebar is put away
	Chapter       string `json:"chapter,omitempty"`     // the node last open
	// Tabs are the nodes open in tabs, left to right; Chapter is the one shown.
	Tabs []string `json:"tabs,omitempty"`
	// Blank says the author closed every tab: none opens by itself.
	Blank bool `json:"blank,omitempty"`
	// Writing reopens the work on the writing page, where it was left.
	Writing bool `json:"writing,omitempty"`
	// AutoPredict continues the prose as typing pauses, not on demand.
	AutoPredict bool `json:"auto_predict,omitempty"`
	// The right column's width and selected tab, and whether a turn from
	// there leaves the chapter out.
	PanelWidth float32 `json:"panel_width,omitempty"`
	PaneTab    paneTab `json:"pane_tab,omitempty"`
	NoAttach   bool    `json:"no_attach,omitempty"`
	// The book's count when Day began, for how much was written today.
	Day      string `json:"day,omitempty"`
	DayStart int    `json:"day_start,omitempty"`
	// Goal is how much the author means to write a day, or 0 for none.
	Goal int `json:"day_goal,omitempty"`
	// Carets is where the caret was left in each chapter, by ID.
	Carets map[string]int `json:"carets,omitempty"`
	// Days is what was written on each day before today, by local date.
	Days map[string]int `json:"days,omitempty"`
	// TextSize is the size of the chapters' text, or 0 for the default.
	TextSize int `json:"text_size,omitempty"`
	// Shelved are the conversations put out of the list, still on disk.
	Shelved []string `json:"shelved_sessions,omitempty"`
	// Ignored are the keys of the pending things the author ignored.
	Ignored []string `json:"ignored,omitempty"`
	// LateAfter is how many chapters may follow a foreshadowing before it
	// is listed as buried long, or 0 for the default.
	LateAfter int `json:"late_after,omitempty"`
	// CheckWhen is when chapters are checked against the setting: as the
	// author leaves one changed when empty, "manual" or "always".
	CheckWhen string `json:"check_when,omitempty"`
}

func (a *App) prefsPath() string { return filepath.Join(a.Store.Root(), ".writer", "desk.json") }

func (a *App) loadPrefs() {
	p := prefs{Writing: true} // a work new to this machine opens on its prose
	if b, err := os.ReadFile(a.prefsPath()); err == nil {
		p = prefs{}
		_ = json.Unmarshal(b, &p) // a damaged file is as good as none
		// The old pair is read only for files without pane_tab. In those
		// files foreshadowings took precedence over the remembered talk.
		var old struct {
			PaneTab *paneTab `json:"pane_tab"`
			TalkTab bool     `json:"talk_tab"`
			ForeTab bool     `json:"fore_tab"`
		}
		_ = json.Unmarshal(b, &old)
		if old.PaneTab == nil {
			if old.ForeTab {
				p.PaneTab = paneFore
			} else if old.TalkTab {
				p.PaneTab = paneTalk
			}
		}
	}
	a.doc.hidden = p.SettingHidden
	a.sideHidden = p.SideHidden
	a.write.node, a.write.tabs, a.write.blank = p.Chapter, p.Tabs, p.Blank && p.Chapter == ""
	a.leftWriting = p.Writing
	if p.Writing {
		a.page = 3
	}
	a.write.autoPredict = p.AutoPredict
	a.write.day, a.write.dayStart, a.write.goal = p.Day, p.DayStart, p.Goal
	a.write.goalText, a.write.carets, a.write.days, a.write.size = "", p.Carets, p.Days, p.TextSize
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
	a.doc.width, a.doc.tab, a.doc.attach = p.PanelWidth, p.PaneTab, !p.NoAttach
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
	b, err := json.Marshal(prefs{Shelved: shelved, SettingHidden: a.doc.hidden, SideHidden: a.sideHidden, Chapter: a.write.node, Tabs: a.write.tabs, Blank: a.write.blank, Writing: a.leftWriting, AutoPredict: a.write.autoPredict,
		PanelWidth: a.doc.width, PaneTab: a.doc.tab, LateAfter: a.write.lateAfter, CheckWhen: a.ck.when, NoAttach: !a.doc.attach,
		Day: a.write.day, DayStart: a.write.dayStart, Goal: a.write.goal, Carets: a.write.carets, Days: a.write.days, TextSize: a.write.size,
		Ignored: a.write.ignoredKeys()})
	if err == nil {
		// .writer/ is kept out of git: a work fetched has none till now.
		err = os.MkdirAll(filepath.Dir(a.prefsPath()), 0755)
	}
	if err == nil {
		err = atomicfile.Write(a.prefsPath(), b, 0644)
	}
	a.warn(err)
}
