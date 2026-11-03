package desk

import (
	"slices"

	"github.com/egoist/mygo/ui"
	"writer/desk/kit"
	"writer/editor"
	"writer/locales"
	"writer/workspace"
)

// The chapters open stand in a row of tabs under the title bar, as many as
// the author opened. One is shown. The others keep their editors, so that
// coming back to one finds what can be undone, the caret and the scroll
// as they were left; what they hold is saved as they are left, so only
// the tab shown can have words that are not on the disk.

// sheetMax is the most a tab is wide: a longer name is cut.
const sheetMax = 200

// keepTabs drops the tabs of what is no longer in the tree. The one shown
// stays while its words are still here, gone from the disk; so does one
// left that way.
func (a *App) keepTabs() {
	w := &a.write
	w.tabs = slices.DeleteFunc(w.tabs, func(id string) bool {
		if _, ok := a.book.Tree().Node(id); ok {
			return false
		}
		if s := w.open[id]; s != nil && s.lost {
			return false
		}
		delete(w.open, id)
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
	for id, s := range w.open {
		keep(id, s.ed, s.external)
	}
}

// unsaved says the node shown has words that are not on the disk: its
// prose or its outline, or all of it when the chapter is gone
// from there, or was changed there and the author has not chosen between
// the two.
func (a *App) unsaved() bool {
	w := &a.write
	if w.node == "" {
		return false
	}
	c := w.cur()
	return c.lost || c.external != nil || c.ed != nil && c.ed.Doc.Version() != c.savedVer || c.outline.Text != c.outline.Saved
}

// tabTitle is what the tab of node id says, and what its tooltip says
// before it: the folders it is in.
func (a *App) tabTitle(id string) (title, path string, folder bool) {
	w := &a.write
	n, ok := a.book.Tree().Node(id)
	if !ok {
		if s := w.open[id]; s != nil {
			return s.lostTitle, "", false
		}
		return "", "", false
	}
	for p := n.Parent; p != nil && p.Parent != nil; p = p.Parent {
		path = p.Title + " › " + path
	}
	return n.Title, path, n.Folder
}

// sheets is the row of tabs: one for each chapter open, or outline of a
// folder, the one shown in ink and underlined. Only focus hides the row.
func (a *App) sheets(c *ui.Context) {
	w := &a.write
	if w.focus {
		return
	}
	var shut []string
	row := ui.Row(c).FillWidth().Height(kit.HBar).Shrink(0).BorderWidth(0, 0, 1, 0).BorderColor(a.pal.Line).Label("标签行")
	row.Padding(0, 0, 0, a.paperInset(c)).Children(func() {
		a.slide(ui.Row(c).Gap(2).AlignItems(ui.Center).Margin(0, 8, 0, -8).Shrink(0)).Children(func() {
			if a.stepButton(c, kit.IcBack, locales.TabPrevOpened()).Disabled(len(w.back) == 0).Clicked() {
				a.stepHistory(false)
			}
			if a.stepButton(c, kit.IcForward, locales.TabNextOpened()).Disabled(len(w.forward) == 0).Clicked() {
				a.stepHistory(true)
			}
		})
		a.sheetRow(c, &shut)
	})
	if shut != nil {
		a.shutTabs(shut)
	}
}

// sheetRow is the tabs themselves, which scroll sideways when they are
// more than fit; shut is given the tabs the author closes.
func (a *App) sheetRow(c *ui.Context, shut *[]string) {
	t := a.pal.Theme
	w := &a.write
	shown := w.node
	defer func() { w.tabSeen = shown }()
	a.slide(ui.ScrollHorizontal(c.Key("标签页")).Height(kit.HBar).Shrink(1)).Children(func() {
		for _, id := range w.tabs {
			on := id == w.node
			title, path, folder := a.tabTitle(id)
			dirty := on && a.unsaved()
			tip := path + title
			if dirty {
				tip += locales.TabUnsaved()
			}
			tab := ui.ButtonBase(c.Key("tab-"+id)).Row().Height(kit.HBar).MaxWidth(sheetMax).AlignItems(ui.Center).Gap(4).Padding(0, 4, 0, 10).Shrink(0).
				Label("标签页-" + title).Tooltip(tip)
			over := tab.Hovered()
			fg, weight := t.TextMuted, 400
			switch {
			case on:
				fg, weight = t.Text, 600
			case over:
				fg = t.Text
			}
			// Into view as it becomes the tab shown, and not after: the
			// author may scroll the row to the others.
			if on && w.tabSeen != id {
				tab.ScrollIntoView()
			}
			tab.Children(func() {
				if on {
					ui.Box(c).Absolute().Left(10).Right(10).Bottom(0).Height(2).Background(a.pal.Ink).Label("当前标签线")
				}
				if folder {
					kit.Glyph(c, kit.IcFolder, kit.IconSm, fg)
				}
				ui.Text(c, title).FontSize(kit.FsControl).FontWeight(weight).TextColor(fg).SingleLine().Ellipsis("…").Shrink(1)
				// The cross keeps its place: shown on the tab open and on the
				// one under the pointer. Words not saved put a dot there, till
				// the pointer comes.
				x := ui.ButtonBase(c.Key("x-"+id)).Size(kit.HTag, kit.HTag).Center().Radius(kit.RXs).Shrink(0).Label("关闭标签页-" + title).Tooltip(locales.TabClose())
				mark := t.TextMuted
				if x.Hovered() {
					x.Background(t.SurfacePressed)
					mark = t.Text
				}
				x.Children(func() {
					switch {
					case dirty && !over:
						ui.Box(c).Size(6, 6).Radius(kit.RRound).Background(t.TextMuted).Label("未保存-" + title)
					case on || over:
						kit.Glyph(c, kit.IcX, kit.IconXs, mark)
					}
				})
				if x.Clicked() {
					*shut = []string{id}
				}
			})
			a.menu.ContextMenu(c, a.pal, tab, "标签页-"+id, func(m *kit.Menu) {
				a.tabMenu(m, id, shut)
			})
			if tab.Clicked() && *shut == nil && !on {
				a.openNode(id)
			}
		}
	})
}

// tabMenu is the menu of the tab id: closing it, the others, or those at
// its right. Each closes the tab clicked, not the one shown.
func (a *App) tabMenu(m *kit.Menu, id string, shut *[]string) {
	w := &a.write
	at := slices.Index(w.tabs, id)
	others := slices.DeleteFunc(slices.Clone(w.tabs), func(k string) bool { return k == id })
	right := slices.Clone(w.tabs[at+1:])
	cmdCloseTab.item(a, m).Do(func() { *shut = []string{id} })
	cmdCloseOthers.item(a, m).Disabled(len(others) == 0).Do(func() { *shut = others })
	cmdCloseRight.item(a, m).Disabled(len(right) == 0).Do(func() { *shut = right })
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
		s := w.open[id]
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
		delete(w.open, id)
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
	a.pal.Modal(c, &open, func() {
		ui.Column(c).Width(kit.DialogAsk).Padding(kit.DialogPad).Gap(16).Children(func() {
			a.pal.DialogHead(c, locales.DialogUnsavedTitle(title), locales.DialogUnsavedAbout())
			ui.Row(c).FillWidth().Gap(8).Justify(ui.End).Children(func() { // system order
				if a.pal.Primary(c, locales.CommonSave()).Label("保存修改").Clicked() {
					answer = "save"
				}
				if a.pal.Secondary(c, locales.DialogDontSave()).Clicked() {
					answer = "drop"
				}
				if a.pal.Secondary(c, locales.CommonCancel()).Clicked() {
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
	switch {
	case w.cur().lost:
		a.saveLost()
		return false
	case w.cur().external != nil:
		a.keepMine()
	}
	a.flush()
	return !a.unsaved()
}

// dropUnsaved puts the node shown back to what the disk has of it.
func (a *App) dropUnsaved() {
	w := &a.write
	// Without its editor nothing of it is saved as it is left.
	c := w.cur()
	c.ed, c.external, c.lost = nil, nil, false
	c.outline.Text, c.outline.Seen = c.outline.Saved, c.outline.Saved
}
