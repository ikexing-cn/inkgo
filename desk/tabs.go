package desk

import (
	"reflect"
	"slices"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/editor"
	"writer/workspace"
)

// The chapters open stand in a row of tabs under the toolbar, as many as
// the author opened. One is shown. The others keep their editors, so that
// coming back to one finds what can be undone, the caret and the scroll
// as they were left; what they hold is saved as they are left, so only
// the tab shown can have words that are not on the disk.

// sheetMax is the most a tab is wide: a longer name is cut.
const sheetMax = 200

// sheet is what a tab not shown keeps of its chapter.
type sheet struct {
	ed       *editor.Editor
	saved    string
	savedVer int
	stamp    workspace.Stamp
	lastSnap time.Time
	// Left with the disk's text changed outside and not yet chosen
	// between, or with the chapter gone from the disk.
	external  *workspace.Chapter
	lost      bool
	lostTitle string
}

// unsettled says the tab was left with words that are not on the disk:
// they could not be saved, or the disk has other words to choose between.
func (s *sheet) unsettled() bool {
	return s.external != nil || s.lost || s.ed.Doc.Version() != s.savedVer
}

// same says the disk has what the tab's editor has, so the editor goes on.
func (s *sheet) same(ch workspace.Chapter) bool {
	marks := storedMarks(s.ed.Doc.Marks())
	return ch.Text == workspace.CleanProse(s.ed.Doc.String()) && (len(marks) == 0 && len(ch.Marks) == 0 || reflect.DeepEqual(marks, ch.Marks))
}

// park keeps the editor of the chapter shown for its tab, as another is
// shown.
func (a *App) park() {
	w := &a.write
	if w.ed == nil || !slices.Contains(w.tabs, w.node) {
		return
	}
	if w.parked == nil {
		w.parked = map[string]*sheet{}
	}
	w.ed.SetFound(nil, 0)
	w.parked[w.node] = &sheet{ed: w.ed, saved: w.saved, savedVer: w.savedVer, stamp: w.stamp, lastSnap: w.lastSnap,
		external: w.external, lost: w.lost, lostTitle: w.lostTitle}
}

// unpark shows a tab's chapter with the editor it kept.
func (a *App) unpark(s *sheet) {
	w := &a.write
	w.ed, w.saved, w.savedVer, w.stamp, w.lastSnap = s.ed, s.saved, s.savedVer, s.stamp, s.lastSnap
	w.external, w.lost, w.lostTitle = s.external, s.lost, s.lostTitle
	w.ed.SetFontSize(float32(w.textSize()))
	w.ed.SetPredictOnDemand(!w.autoPredict)
	w.blank = false
}

// keepTabs drops the tabs of what is no longer in the tree. The one shown
// stays while its words are still here, gone from the disk; so does one
// left that way.
func (a *App) keepTabs() {
	w := &a.write
	w.tabs = slices.DeleteFunc(w.tabs, func(id string) bool {
		if _, ok := w.tree.Node(id); ok {
			return false
		}
		if id == w.node && w.lost {
			return false
		}
		if s := w.parked[id]; s != nil && s.lost {
			return false
		}
		delete(w.parked, id)
		return true
	})
}

// keepUnsettled keeps, as the work is left, the words of the tabs whose
// chapter was changed outside and the author had not chosen between the
// two: the disk's stay, and the editor's become a snapshot.
func (a *App) keepUnsettled() {
	if a.Store == nil {
		return
	}
	w := &a.write
	keep := func(id string, ed *editor.Editor, disk *workspace.Chapter) {
		if ed == nil || disk == nil || workspace.CleanProse(ed.Doc.String()) == disk.Text {
			return
		}
		_, err := a.Store.TakeSnapshot(id, workspace.SnapExternal, ed.Doc.String(), storedMarks(ed.Doc.Marks()))
		a.warn(err)
	}
	keep(w.node, w.ed, w.external)
	for id, s := range w.parked {
		keep(id, s.ed, s.external)
	}
}

// unsaved says the node shown has words that are not on the disk: its
// prose, its outline or its summary, or all of it when the chapter is gone
// from there, or was changed there and the author has not chosen between
// the two.
func (a *App) unsaved() bool {
	w := &a.write
	if w.node == "" {
		return false
	}
	return w.lost || w.external != nil || w.ed != nil && (w.ed.Doc.Version() != w.savedVer || w.summary != w.summarySaved) || w.outline != w.outlineSaved
}

// tabTitle is what the tab of node id says, and what its tooltip says
// before it: the folders it is in.
func (a *App) tabTitle(id string) (title, path string, folder bool) {
	w := &a.write
	n, ok := w.tree.Node(id)
	if !ok {
		if s := w.parked[id]; s != nil {
			return s.lostTitle, "", false
		}
		return w.lostTitle, "", false
	}
	for p := n.Parent; p != nil && p.Parent != nil; p = p.Parent {
		path = p.Title + " › " + path
	}
	return n.Title, path, n.Folder
}

// sheets is the row of tabs: one for each chapter open, or outline of a
// folder, the one shown as light as the page under it and joined to it.
// The row is away when nothing is open, and in focus.
func (a *App) sheets(c *ui.Context) {
	w := &a.write
	if len(w.tabs) == 0 || w.focus {
		return
	}
	var shut []string
	row := ui.Row(c).FillWidth().Height(hControl).Background(a.pal.side).Shrink(0)
	row.Children(func() {
		a.sheetRow(c, &shut)
		// The line under the row runs on to the edge.
		ui.Box(c).Grow(1).Height(hControl).BorderWidth(0, 0, 1, 0).BorderColor(a.pal.line)
	})
	if shut != nil {
		a.shutTabs(shut)
	}
}

// sheetRow is the tabs themselves, which scroll sideways when they are
// more than fit; shut is given the tabs the author closes.
func (a *App) sheetRow(c *ui.Context, shut *[]string) {
	t := a.pal.theme
	w := &a.write
	shown := w.node
	defer func() { w.tabSeen = shown }()
	ui.ScrollHorizontal(c.Key("标签页")).Height(hControl).Shrink(1).Children(func() {
		for _, id := range w.tabs {
			on := id == w.node
			title, path, folder := a.tabTitle(id)
			dirty := on && a.unsaved()
			tip := path + title
			if dirty {
				tip += " · 未保存"
			}
			tab := ui.ButtonBase(c.Key("tab-"+id)).Row().Height(hControl).MaxWidth(sheetMax).AlignItems(ui.Center).Gap(6).Padding(0, 6, 0, 12).Shrink(0).
				BorderWidth(0, 1, 1, 0).BorderColor(a.pal.line).Label("标签页-" + title).Tooltip(tip)
			over := tab.Hovered()
			fg, weight := t.TextMuted, 400
			switch {
			case on:
				// Joined to the page: its ground, and no line under it.
				tab.Background(a.pal.canvas).BorderWidth(0, 1, 0, 0)
				fg, weight = t.Text, 600
			case over:
				tab.Background(t.SurfaceHover)
			}
			// Into view as it becomes the tab shown, and not after: the
			// author may scroll the row to the others.
			if on && w.tabSeen != id {
				tab.ScrollIntoView()
			}
			tab.Children(func() {
				pic := icDoc
				if folder {
					pic = icFolder
				}
				glyph(c, pic, iconSm, fg)
				ui.Text(c, title).FontSize(fsControl).FontWeight(weight).TextColor(fg).SingleLine().Ellipsis("…").Shrink(1)
				// The cross keeps its place: shown on the tab open and on the
				// one under the pointer. Words not saved put a dot there, till
				// the pointer comes.
				x := ui.ButtonBase(c.Key("x-"+id)).Size(hTag, hTag).Center().Radius(rXs).Shrink(0).Label("关闭标签页-" + title).Tooltip("关闭标签页")
				mark := t.TextMuted
				if x.Hovered() {
					x.Background(t.SurfacePressed)
					mark = t.Text
				}
				x.Children(func() {
					switch {
					case dirty && !over:
						ui.Box(c).Size(6, 6).Radius(rRound).Background(t.TextMuted).Label("未保存-" + title)
					case on || over:
						glyph(c, icX, iconXs, mark)
					}
				})
				if x.Clicked() {
					*shut = []string{id}
				}
			})
			a.contextMenu(c, tab, "标签页-"+id, func(m *menu) {
				at := slices.Index(w.tabs, id)
				others := slices.DeleteFunc(slices.Clone(w.tabs), func(k string) bool { return k == id })
				right := slices.Clone(w.tabs[at+1:])
				m.Item("关闭").Hint(modKey + "W").Do(func() { *shut = []string{id} })
				m.Item("关闭其他").Disabled(len(others) == 0).Do(func() { *shut = others })
				m.Item("关闭右侧").Disabled(len(right) == 0).Do(func() { *shut = right })
			})
			if tab.Clicked() && *shut == nil && !on {
				a.openNode(id)
			}
		}
	})
}

// tabKeys are the keys of the tabs: closing the one shown, and going to
// the next and the one before, round the row.
func (a *App) tabKeys(c *ui.Context) {
	w := &a.write
	if c.Shortcut(ui.Cmd, ui.KeyW) && w.node != "" {
		a.shutTabs([]string{w.node})
	}
	step := 0
	if c.Shortcut(ui.Ctrl, ui.KeyTab) {
		step = 1
	}
	if c.Shortcut(ui.Ctrl|ui.Shift, ui.KeyTab) {
		step = -1
	}
	if at := slices.Index(w.tabs, w.node); step != 0 && at >= 0 && len(w.tabs) > 1 {
		a.openNode(w.tabs[(at+step+len(w.tabs))%len(w.tabs)])
	}
}

// shutTabs closes the tabs gone. When the one shown is among them and has
// words not saved, the author is asked first, and it is their answer that
// closes them. A tab left with words that could not be saved is shown and
// asked about the same way. Closing the one shown shows the tab at its
// right, or else the one at its left; with none left the page is blank,
// and going back opens the chapter just closed.
func (a *App) shutTabs(gone []string) {
	w := &a.write
	for _, id := range gone {
		s := w.parked[id]
		if id == w.node && a.unsaved() || s != nil && s.unsettled() {
			if id != w.node {
				a.openNode(id)
			}
			w.closing = gone
			return
		}
	}
	at := slices.Index(w.tabs, w.node)
	left := slices.DeleteFunc(slices.Clone(w.tabs), func(id string) bool { return slices.Contains(gone, id) })
	for _, id := range gone {
		delete(w.parked, id)
	}
	if at < 0 || !slices.Contains(gone, w.node) {
		w.tabs = left
		a.savePrefs()
		return
	}
	next := ""
	for _, id := range w.tabs[at+1:] {
		if slices.Contains(left, id) {
			next = id
			break
		}
	}
	for i := at - 1; i >= 0 && next == ""; i-- {
		if slices.Contains(left, w.tabs[i]) {
			next = w.tabs[i]
		}
	}
	// Out of the tabs first: what is shown is left, not kept for a tab.
	w.tabs = left
	a.openNode(next)
	if next == "" {
		w.blank = true
		a.savePrefs()
	}
}

// askToSave asks, as a tab with words not saved is closed, whether to save
// them. Nothing is saved while it asks. Its three answers stand in the
// order the system asks in: save, do not, leave it open.
func (a *App) askToSave(c *ui.Context) {
	w := &a.write
	open := len(w.closing) > 0
	if !open {
		return
	}
	title, _, _ := a.tabTitle(w.node)
	answer := ""
	a.modal(c, &open, func() {
		ui.Column(c).Width(dialogAsk).Padding(dialogPad).Gap(16).Children(func() {
			a.pal.dialogHead(c, "保存对「"+title+"」的修改？", "不保存将丢失上次保存后的修改。")
			ui.Row(c).FillWidth().Gap(8).Justify(ui.End).Children(func() { // system order
				if a.pal.primary(c, "保存").Label("保存修改").Clicked() {
					answer = "save"
				}
				if a.pal.secondary(c, "不保存").Clicked() {
					answer = "drop"
				}
				if a.pal.secondary(c, "取消").Clicked() {
					answer = "stay"
				}
			})
		})
	})
	gone := w.closing
	switch {
	case answer == "save":
		w.closing = nil
		if !a.saveToClose() {
			return
		}
		a.shutTabs(gone) // the next of them with words not saved is asked about in its turn
	case answer == "drop":
		w.closing = nil
		a.dropUnsaved()
		a.shutTabs(gone)
	case answer == "stay" || !open:
		w.closing = nil
	}
}

// saveToClose saves what the node shown has, and reports that it is on
// the disk. Of two texts, the disk's changed outside, the one here is
// kept; a chapter gone from the disk is saved as a new one, which stays
// open in its place.
func (a *App) saveToClose() bool {
	w := &a.write
	a.stopAI()
	switch {
	case w.lost:
		a.saveLost()
		return false
	case w.external != nil:
		a.keepMine()
	}
	a.flush()
	return !a.unsaved()
}

// dropUnsaved puts the node shown back to what the disk has of it.
func (a *App) dropUnsaved() {
	w := &a.write
	a.stopAI()
	// Without its editor nothing of it is saved as it is left.
	w.ed, w.external, w.lost = nil, nil, false
	w.outline, w.summary = w.outlineSaved, w.summarySaved
}
