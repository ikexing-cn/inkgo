package desk

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/workspace"
)

// docView is the state of the setting panel.
type docView struct {
	hidden  bool
	history bool   // the revisions are shown
	query   string // what the list is searched for
	shown   string // the entry whose page is shown
	todo    bool   // what waits on the author is shown
	dropped bool   // the deleted entries are listed
	editing string // the entry being edited, "new" for a new one
	asking  string // with editing "new", the AI's question it answers
	// The form: the entry's text, category, name, and what else it is
	// called as the author writes them, apart by spaces or 、
	text     string
	category string
	name     string
	names    string
	// The right column: its width as dragged, and, on the writing page,
	// whether it shows the talk and sends the chapter with it.
	width    float32
	resizing bool
	talk     bool
	// The talks are listed there, not the one open, and what the list is
	// searched for.
	talks     bool
	talkQuery string
	fore      bool // the writing page shows the foreshadowings in it
	attach    bool
}

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
	// The right column's width, whether the writing page shows the talk
	// in it, and whether a turn from there leaves the chapter out.
	PanelWidth float32 `json:"panel_width,omitempty"`
	TalkTab    bool    `json:"talk_tab,omitempty"`
	ForeTab    bool    `json:"fore_tab,omitempty"`
	NoAttach   bool    `json:"no_attach,omitempty"`
	// NoAutoSummary leaves a chapter's summary alone as the author leaves
	// it changed.
	NoAutoSummary bool `json:"no_auto_summary,omitempty"`
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
	var p prefs
	if b, err := os.ReadFile(a.prefsPath()); err == nil {
		_ = json.Unmarshal(b, &p) // a damaged file is as good as none
	}
	a.doc.hidden = p.SettingHidden
	a.sideHidden = p.SideHidden
	a.write.node, a.write.tabs, a.write.blank = p.Chapter, p.Tabs, p.Blank && p.Chapter == ""
	a.leftWriting = p.Writing
	if p.Writing {
		a.page = 3
	}
	a.write.autoPredict = p.AutoPredict
	a.write.autoSummary = !p.NoAutoSummary
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
	a.doc.width, a.doc.talk, a.doc.attach = p.PanelWidth, p.TalkTab, !p.NoAttach
	a.doc.fore, a.write.lateAfter, a.write.fore = p.ForeTab, p.LateAfter, foreState{}
	a.ck.when = checkOnLeave
	if p.CheckWhen == checkOnDemand || p.CheckWhen == checkOnPause {
		a.ck.when = p.CheckWhen
	}
}

func (a *App) savePrefs() {
	shelved := slices.Sorted(maps.Keys(a.shelved))
	b, err := json.Marshal(prefs{Shelved: shelved, SettingHidden: a.doc.hidden, SideHidden: a.sideHidden, Chapter: a.write.node, Tabs: a.write.tabs, Blank: a.write.blank, Writing: a.leftWriting, AutoPredict: a.write.autoPredict,
		PanelWidth: a.doc.width, TalkTab: a.doc.talk, ForeTab: a.doc.fore, LateAfter: a.write.lateAfter, CheckWhen: a.ck.when, NoAttach: !a.doc.attach, NoAutoSummary: !a.write.autoSummary,
		Day: a.write.day, DayStart: a.write.dayStart, Goal: a.write.goal, Carets: a.write.carets, Days: a.write.days, TextSize: a.write.size,
		Ignored: a.write.ignoredKeys()})
	if err == nil {
		// Whole or not at all: a file cut short would be read as none, and
		// the tabs, what was ignored and the days' counts would be gone.
		temp := a.prefsPath() + ".tmp"
		if err = os.WriteFile(temp, b, 0644); err == nil {
			err = os.Rename(temp, a.prefsPath())
		}
	}
	a.warn(err)
}

// setSideHidden puts the sidebar away, on every page, or brings it back.
func (a *App) setSideHidden(hidden bool) {
	a.sideHidden = hidden
	a.savePrefs()
}

// setDocHidden folds the panel away, for the talk alone, or brings it back.
func (a *App) setDocHidden(hidden bool) {
	a.doc.hidden = hidden
	a.savePrefs()
}

var placeNames = map[string]string{workspace.PlaceBody: "正文", workspace.PlaceOpen: "待确认", workspace.PlaceDropped: "已删除"}

// revise records the author's change and shows the setting after it.
func (a *App) revise(ops ...workspace.Op) {
	_, err := a.Store.Revise(workspace.Revision{By: workspace.ByAuthor, Ops: ops})
	if err == nil {
		err = a.refresh()
	}
	a.setError(err)
}

// settingPanel is the setting in the right column: its entries a row
// each, or the page of what a row was clicked for.
func (a *App) settingPanel(c *ui.Context) {
	d := &a.doc
	paneTop(c).Children(func() {
		if a.page == 3 {
			a.panelTabs(c)
		} else {
			// The count is what waits on the author, as in the tab.
			ui.Text(c, "设定").FontSize(fsGroup).FontWeight(600)
			if open, _ := a.ideas(); len(open) > 0 {
				ui.Textf(c, "%d", len(open)).FontSize(fsNote).TextColor(a.pal.faint).Padding(0, 0, 0, 4)
			}
		}
		ui.Spacer(c)
		if iconButton(c, icPanel, "收起右栏", ui.Color{}).Tooltip("收起右栏").Clicked() {
			a.setDocHidden(true)
		}
	})
	if d.editing != "" && d.editing != "new" {
		if _, ok := a.set.Entry(d.editing); !ok {
			d.editing = ""
		}
	}
	if q, ok := a.set.Entry(d.asking); d.asking != "" && (!ok || q.Place != workspace.PlaceOpen) {
		d.editing, d.asking = "", "" // the question is gone
	}
	shown, ok := a.set.Entry(d.shown)
	if !ok {
		d.shown = ""
	}
	switch {
	case d.history:
		a.history(c)
	case d.editing == "new":
		a.entryForm(c, workspace.Entry{})
	case d.editing != "":
		e, _ := a.set.Entry(d.editing)
		a.entryForm(c, e)
	case ok:
		a.entryPage(c, shown)
	case d.todo:
		a.ideasPage(c)
	case d.dropped:
		a.droppedList(c)
	default:
		a.settingList(c)
	}
}

// bodyCount is how many entries the author has settled.
func (a *App) bodyCount() int {
	n := 0
	for _, e := range a.set.Entries {
		if e.Place == workspace.PlaceBody {
			n++
		}
	}
	return n
}

// entryName is what a row calls the entry and the line of it that
// follows: its name and its text, or its text alone when it names nothing.
func entryName(e workspace.Entry) (name, gist string) {
	text := strings.Join(strings.Fields(e.Text), " ")
	if len(e.Names) == 0 {
		return text, ""
	}
	return clip(e.Names[0], 12), text
}

// ideas are what the AI suggests and the author has not decided,
// and for each the settled entry it is about, if it is about one: the one
// that shares a name with it, or whose name it says. With the AI off,
// there are none.
func (a *App) ideas() (open []workspace.Entry, about map[string]string) {
	about = map[string]string{}
	if !a.aiOn() {
		return nil, about
	}
	for _, e := range a.set.Entries {
		if e.Place != workspace.PlaceOpen {
			continue
		}
		open = append(open, e)
		best := 0
		for _, b := range a.set.Entries {
			if b.Place != workspace.PlaceBody {
				continue
			}
			for _, name := range b.Names {
				n := len(name)
				if slices.Contains(e.Names, name) {
					n += len(e.Text) // a shared name tells more than a mention
				} else if !strings.Contains(e.Text, name) {
					continue
				}
				if n > best {
					best, about[e.ID] = n, b.ID
				}
			}
		}
	}
	return open, about
}

// settingList is the settled entries by category, a row each, under what
// waits on the author; or those that say what is searched for.
func (a *App) settingList(c *ui.Context) {
	d := &a.doc
	open, about := a.ideas()
	waiting := map[string]bool{}
	for _, id := range about {
		waiting[id] = true
	}
	dropped := 0
	for _, e := range a.set.Entries {
		if e.Place == workspace.PlaceDropped {
			dropped++
		}
	}
	if a.paneFind(c, &d.query, "搜索设定", "新增", func(m *menu) {
		m.Item("已删除").Hint(count(dropped)).Do(func() { d.dropped = true })
		m.Item("修改记录").Do(func() { d.history = true })
	}) {
		d.editing, d.asking, d.text, d.category, d.name, d.names = "new", "", "", a.firstCategory(), "", ""
	}
	q := trim(d.query)
	row := func(e workspace.Entry) {
		name, gist := entryName(e)
		if q != "" {
			gist = nearFind(gist, q)
		}
		b := a.paneRow(c.Key(e.ID), "设定-"+e.ID, name, gist, "", q, waiting[e.ID])
		// One a talk just noted is tinted for a moment.
		if at, ok := a.flash[e.ID]; ok {
			if left := flashFor - time.Since(at); left > 0 {
				b.Background(a.pal.ink.Alpha(0.2 * float32(left) / float32(flashFor)))
				c.AnimationFrame()
			} else {
				delete(a.flash, e.ID)
			}
		}
		if b.Clicked() {
			d.shown = e.ID
		}
	}
	a.paneRows(c, "setting", func() {
		if a.renamed.count > 0 {
			ui.Box(c).FillWidth().Padding(8, 0, 0, 0).Children(func() { a.renameNotice(c) })
		}
		if q != "" {
			var found []workspace.Entry
			for _, e := range a.set.Entries {
				if e.Place == workspace.PlaceBody && (strings.Contains(e.Text, q) || slices.ContainsFunc(e.Names, func(n string) bool { return strings.Contains(n, q) })) {
					found = append(found, e)
				}
			}
			if len(found) == 0 {
				a.paneEmpty(c, "未找到", "")
				return
			}
			a.paneGroup(c, fmt.Sprintf("找到 %d 条", len(found)))
			for _, e := range found {
				row(e)
			}
			return
		}
		if len(open) > 0 && a.paneTodo(c, "待确认", len(open)).Clicked() {
			d.todo = true
		}
		if a.bodyCount() == 0 {
			how := "讨论和正文中的设定会自动收录于此，也可点击「新增」手动添加。"
			if !a.aiOn() {
				how = "可点击「新增」添加，或从正文中选中文字加入。"
			}
			a.paneEmpty(c, "暂无设定", how)
			return
		}
		for _, cat := range a.Store.Work().Categories {
			first := true
			for _, e := range a.set.Entries {
				if e.Place != workspace.PlaceBody || e.Category != cat.Name {
					continue
				}
				if first {
					a.paneGroup(c, cat.Name)
					first = false
				}
				row(e)
			}
		}
	})
}

// asks tells whether e, waiting on the author, asks them something: the
// AI writes a question with a question mark at its end.
func asks(e workspace.Entry) bool {
	text := strings.TrimSpace(e.Text)
	return strings.HasSuffix(text, "？") || strings.HasSuffix(text, "?")
}

// idea is one thing the AI suggests, with what it is about when
// about says, and the author's word on it: a question is not a setting to
// take, its answer is.
func (a *App) idea(c *ui.Context, e workspace.Entry, about string) {
	d := &a.doc
	ui.Column(c.Key(e.ID)).FillWidth().Gap(8).Children(func() {
		if about != "" {
			ui.Text(c, about).FontSize(fsNote).TextColor(a.pal.theme.TextMuted).SingleLine().Ellipsis("…").FillWidth()
		}
		ui.Text(c, e.Text).FontSize(fsBody).LineHeight(lhLoose).Selectable().FillWidth()
		paneButtons(c, func() {
			if a.paneButton(c, "忽略", false).Label("忽略-" + e.ID).Clicked() {
				a.revise(workspace.Op{Op: workspace.OpMove, Entry: e.ID, Place: workspace.PlaceDropped})
			}
			if asks(e) {
				if a.paneButton(c, "回答", true).Label("回答-" + e.ID).Clicked() {
					d.editing, d.asking, d.text, d.category, d.name, d.names = "new", e.ID, "", e.Category, "", ""
				}
			} else if a.paneButton(c, "采纳", true).Label("采纳-" + e.ID).Clicked() {
				a.revise(workspace.Op{Op: workspace.OpMove, Entry: e.ID, Place: workspace.PlaceBody})
			}
		})
	})
}

// ideasPage is all that waits on the author, one after another.
func (a *App) ideasPage(c *ui.Context) {
	d := &a.doc
	if back, _ := a.paneBack(c, "全部设定", ""); back {
		d.todo = false
	}
	open, about := a.ideas()
	if len(open) == 0 {
		d.todo = false // the last was decided
	}
	a.panePage(c, "ideas", func() {
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "待确认").FontSize(fsGroup).FontWeight(600)
			ui.Spacer(c)
			ui.Textf(c, "%d 条 AI 建议", len(open)).FontSize(fsNote).TextColor(a.pal.theme.TextMuted)
		})
		for i, e := range open {
			if i > 0 {
				a.pal.hairline(c)
			}
			words := "新设定"
			if b, ok := a.set.Entry(about[e.ID]); ok {
				words = "关于 " + b.Names[0]
			}
			a.idea(c, e, words)
		}
	})
}

// droppedList is the entries the author deleted, which a click shows and
// its page puts back.
func (a *App) droppedList(c *ui.Context) {
	d := &a.doc
	if back, _ := a.paneBack(c, "全部设定", ""); back {
		d.dropped = false
	}
	a.paneRows(c, "dropped", func() {
		a.paneGroup(c, "已删除")
		n := 0
		for _, e := range a.set.Entries {
			if e.Place != workspace.PlaceDropped {
				continue
			}
			n++
			name, gist := entryName(e)
			if a.paneRow(c.Key(e.ID), "设定-"+e.ID, name, gist, "", "", false).Clicked() {
				d.shown = e.ID
			}
		}
		if n == 0 {
			a.paneEmpty(c, "没有已删除的设定", "")
		}
	})
}

// entryPage is one entry: all it says, where it comes from, and what the
// AI suggests about it.
func (a *App) entryPage(c *ui.Context, e workspace.Entry) {
	d := &a.doc
	from, act := "全部设定", "删除设定"
	switch e.Place {
	case workspace.PlaceDropped:
		from, act = "已删除", "恢复设定"
	case workspace.PlaceOpen:
		act = ""
	}
	back, acted := a.paneBack(c, from, act)
	if back {
		d.shown = ""
	}
	if acted {
		place := workspace.PlaceDropped
		if e.Place == place {
			place = workspace.PlaceBody
		}
		a.revise(workspace.Op{Op: workspace.OpMove, Entry: e.ID, Place: place})
		d.shown = ""
	}
	open, about := a.ideas()
	a.panePage(c, "entry-"+e.ID, func() {
		name, sub := e.Category, ""
		if len(e.Names) > 0 {
			name = e.Names[0]
			if len(e.Names) > 1 {
				sub = "别名 " + strings.Join(e.Names[1:], "、")
			}
		}
		if a.paneTitle(c, name, sub, "修改") {
			d.editing, d.asking, d.text, d.category, d.name, d.names = e.ID, "", e.Text, e.Category, "", ""
			if len(e.Names) > 0 {
				d.name, d.names = e.Names[0], strings.Join(e.Names[1:], "、")
			}
		}
		text := ui.Text(c, e.Text).FontSize(fsBody).LineHeight(lhLoose).Selectable().FillWidth()
		if e.Place == workspace.PlaceDropped {
			text.TextColor(a.pal.theme.TextMuted)
		}
		if e.Asked != "" {
			a.paneNote(c, "回答："+e.Asked)
		}
		if i := slices.IndexFunc(e.Sources, func(s workspace.Source) bool { return s.Chapter != "" }); i >= 0 {
			src := e.Sources[i]
			where := "“" + src.Quote + "”"
			if title, ok := a.chapterTitle(src.Chapter); ok {
				where = "「" + title + "」" + where
			}
			if a.paneWhere(c, where, "原文-"+e.ID) {
				a.findInChapter(src.Chapter, src.Quote)
			}
		}
		first := true
		for _, o := range open {
			if about[o.ID] != e.ID {
				continue
			}
			a.pal.hairline(c)
			if first {
				ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
					dot(c, a.pal.amber, 6)
					ui.Text(c, "AI 建议").FontSize(fsNote).FontWeight(600)
				})
				first = false
			}
			a.idea(c, o, "")
		}
	})
}

// entryForm changes e's name, text and category, or writes a new entry
// when e is empty: the answer to the AI's question when one is asked.
func (a *App) entryForm(c *ui.Context, e workspace.Entry) {
	d := &a.doc
	q, asked := a.set.Entry(d.asking)
	asked = asked && d.asking != "" && e.ID == ""
	from := "全部设定"
	switch {
	case e.ID != "":
		from, _ = entryName(e)
		from = clip(from, 16)
	case asked:
		from = "待确认"
	}
	if back, _ := a.paneBack(c, from, ""); back {
		d.editing, d.asking = "", ""
	}
	a.panePage(c, "form-"+e.ID, func() {
		// A new entry starts at its text, a changed one at its name: the
		// keyboard is in that field as the form comes.
		content := func() {
			a.paneField(c, "内容", func() {
				in := ui.TextAreaBase(c, &d.text).Label("设定内容").FillWidth().Height(96).FontSize(fsBody).LineHeight(lhLoose).
					Padding(8, 10).Radius(rControl).Background(a.pal.canvas).Border(1, a.pal.theme.Border)
				if e.ID == "" {
					in.AutoFocus()
				}
			})
		}
		name, alias := "名字", "别名"
		if e.ID == "" {
			if asked {
				ui.Text(c, "回答").FontSize(fsGroup).FontWeight(600)
				a.paneNote(c, q.Text)
			} else {
				ui.Text(c, "新增设定").FontSize(fsGroup).FontWeight(600)
			}
			name, alias = "名字（选填）", "别名（选填）"
			content()
		}
		a.paneField(c, name, func() {
			in := ui.TextInput(c, &d.name).Label("名字").FillWidth().FontSize(fsBody)
			if e.ID != "" {
				in.AutoFocus()
			}
		})
		a.paneField(c, alias, func() {
			ui.TextInput(c, &d.names).Label("别名").Placeholder("多个别名用顿号分隔").FillWidth().FontSize(fsBody)
		})
		if e.ID != "" {
			content()
		}
		a.paneField(c, "分类", func() {
			ui.Select(c, &d.category, a.categoryNames()).Label("设定分类").FillWidth()
		})
		text := trim(d.text)
		names := splitNames(d.name + "、" + d.names)
		paneButtons(c, func() {
			if a.paneButton(c, "取消", false).Clicked() {
				d.editing, d.asking = "", ""
			}
			if !a.paneButton(c, "保存", true).Disabled(text == "").Clicked() {
				return
			}
			var ops []workspace.Op
			switch {
			case asked:
				ops = append(ops, workspace.Op{Op: workspace.OpAnswer, Entry: q.ID, Category: d.category, Text: text, Names: names})
			case e.ID == "":
				ops = append(ops, workspace.Op{Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: d.category, Text: text, Names: names})
			default:
				if text != e.Text {
					ops = append(ops, workspace.Op{Op: workspace.OpEdit, Entry: e.ID, Text: text})
				}
				if d.category != e.Category {
					ops = append(ops, workspace.Op{Op: workspace.OpMove, Entry: e.ID, Category: d.category})
				}
				if !slices.Equal(names, e.Names) {
					ops = append(ops, workspace.Op{Op: workspace.OpName, Entry: e.ID, Names: names})
				}
			}
			if len(ops) > 0 {
				a.revise(ops...)
			}
			if a.Error == "" {
				d.editing, d.asking = "", ""
				if len(e.Names) > 0 && len(names) > 0 {
					a.noteRename(e.Names[0], names[0])
				}
			}
		})
	})
}

// history lists the revisions, newest first: what changed and who
// changed it; what a talk noted links to the talk. With the AI off, what
// talks noted is not listed.
func (a *App) history(c *ui.Context) {
	if back, _ := a.paneBack(c, "全部设定", ""); back {
		a.doc.history = false
	}
	a.pal.hairline(c)
	ui.Scroll(c.Key("history")).Grow(1).FillWidth().Children(func() {
		ui.Column(c).FillWidth().Gap(10).Padding(16, 20, 20, 20).Children(func() {
			ui.Text(c, "修改记录").FontSize(fsGroup).FontWeight(600)
			shown := 0
			for n := len(a.set.Revisions) - 1; n >= 0; n-- {
				r := a.set.Revisions[n]
				if len(r.Ops) == 0 || r.By != workspace.ByAuthor && !a.aiOn() {
					continue // the bookkeeper's that failed changed nothing
				}
				shown++
				ui.Column(c.Key(r.ID)).FillWidth().Padding(10, 12).Gap(6).Radius(rCard).Background(a.pal.raised).Border(1, a.pal.line).Children(func() {
					ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
						who := "手动修改"
						if r.By != workspace.ByAuthor {
							who = "讨论"
						}
						ui.Text(c, who).FontSize(fsNote).FontWeight(600)
						ui.Spacer(c)
						ui.Text(c, r.At.Local().Format("01-02 15:04")).FontSize(fsSmall).TextColor(a.pal.faint)
					})
					for _, op := range r.Ops {
						ui.Text(c, a.describeOp(op)).FontSize(fsNote).LineHeight(lhBody).Selectable().FillWidth()
					}
					if r.By != workspace.ByAuthor {
						if _, ok := a.titles[r.Session]; ok {
							_, words := a.sessionParts(r.Session)
							if a.paneLink(c, "记在「"+words+"」", "查看讨论", "讨论-"+r.ID) {
								a.page = 0
								a.switchSession(r.Session)
							}
						}
					}
				})
			}
			switch {
			case shown > 0:
			case a.aiOn():
				a.paneNote(c, "暂无修改记录。讨论中 AI 记下的也列在这里。")
			default:
				a.paneNote(c, "暂无修改记录。")
			}
		})
	})
}

// describeOp says what op did, in the words of the panel.
func (a *App) describeOp(op workspace.Op) string {
	e, _ := a.set.Entry(op.Entry)
	switch op.Op {
	case workspace.OpAdd:
		return fmt.Sprintf("新增到%s · %s：%s", placeNames[op.Place], op.Category, op.Text)
	case workspace.OpAnswer:
		return fmt.Sprintf("新增到%s · %s：%s", placeNames[workspace.PlaceBody], op.Category, op.Text)
	case workspace.OpEdit:
		return "改写为：" + op.Text
	case workspace.OpName:
		if len(op.Names) == 0 {
			return "移除名称：" + clip(e.Text, 24)
		}
		return "名称改为：" + strings.Join(op.Names, "、")
	}
	text := clip(e.Text, 24)
	if op.Place != "" {
		return fmt.Sprintf("移到%s：%s", placeNames[op.Place], text)
	}
	return fmt.Sprintf("改归%s：%s", op.Category, text)
}
