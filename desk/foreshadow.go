package desk

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/desk/book"
	"writer/desk/kit"
	"writer/editor"
	"writer/workspace"
)

// The least and the most the author may set that number to, and the step.
const lateMin, lateMax, lateStep = 10, 120, 5

var foreLabels = map[workspace.ForeshadowStatus]string{workspace.ForeOpen: "未回收", workspace.ForeDone: "已回收", workspace.ForeDropped: "已放弃", workspace.ForeLost: "原句已删"}

// name is what the author called it, or the sentence when they wrote
// nothing.
func foreName(f workspace.Foreshadow) string {
	if strings.TrimSpace(f.Note) != "" {
		return f.Note
	}
	return kit.Clip(f.Quote, 24)
}

// meta tells where it was buried and how long before chapter here, which
// pays it off, as the picker lists it; the chapter open is 本章.
func foreMeta(f workspace.Foreshadow, here int) string {
	paid := len(f.Payoffs) > 0
	switch {
	case f.Here && paid:
		return "本章 · 已回收"
	case f.Here:
		return "本章埋下"
	case paid:
		return f.Title + " · 已回收"
	}
	return fmt.Sprintf("%s · %d 章前埋下", f.Title, max(here-f.Chapter, 0))
}

// foreState is what the foreshadowings' column and popovers show.
type foreState struct {
	// status is the list shown: workspace.ForeOpen for those not paid off, with
	// those that wait on the author over them, or the list of another.
	status workspace.ForeshadowStatus
	query  string // what the list is searched for
	shown  string // the foreshadowing whose page is shown

	// Burying: the range to mark and its note.
	noting   bool
	from, to int
	note     string

	// Paying off: the range that does, what the foreshadowings are
	// searched for, and the one picked.
	paying   bool
	payQuery string
	pick     string

	renoting bool // the note of the one shown is being changed
	renote   string
}

// foreshadowKey is everything foreshadows reads: the book, and the chapter
// open with its editor's text.
type foreshadowKey struct {
	book    *book.Book
	version int
	ed      *editor.Editor
	doc     int
	node    string
}

func (a *App) foreshadowKey() foreshadowKey {
	w := &a.write
	doc := 0
	if w.cur().ed != nil {
		doc = w.cur().ed.Doc.Version()
	}
	return foreshadowKey{a.book, a.book.Version(), w.cur().ed, doc, w.node}
}

// foreshadows are the book's foreshadowings and what pays them off
// (workspace.Tree.Foreshadows), the open chapter's as it is being written.
func (a *App) foreshadows() []workspace.Foreshadow {
	w := &a.write
	key := a.foreshadowKey()
	return w.fores.get(key, func() []workspace.Foreshadow {
		current := ""
		var live []workspace.ChapterMark
		if w.cur().ed != nil {
			current, live = w.node, liveMarks(w.cur().ed.Doc)
		}
		return a.book.Tree().Foreshadows(a.book.Marks(), current, live)
	})
}

func markID() string {
	var b [5]byte
	rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// openBury asks for the note of a foreshadowing on the selection.
func (a *App) openBury() {
	w := &a.write
	s, e := w.cur().ed.Selection()
	if s == e || w.cur().ed.ReadOnly() {
		return
	}
	w.fore.noting, w.fore.paying, w.fore.from, w.fore.to, w.fore.note = true, false, s, e, ""
	a.closeFind()
	w.rewriting = false
}

// bury marks the range asked about as a foreshadowing with the note
// written.
func (a *App) bury() {
	w := &a.write
	f := &w.fore
	if !f.noting || w.cur().ed == nil {
		return
	}
	id := markID()
	w.cur().ed.AddMark(editor.Mark{Kind: editor.MarkForeshadow, Start: f.from, End: f.to, Note: strings.TrimSpace(f.note),
		Key: id, At: time.Now().Format(time.RFC3339)})
	f.noting = false
	a.warn(a.saveText())
	w.cur().ed.Focus()
}

// openPay asks which foreshadowing the selection pays off, the one of id
// picked when there is one.
func (a *App) openPay(id string) {
	w := &a.write
	if w.cur().ed == nil || w.cur().ed.ReadOnly() {
		return
	}
	s, e := w.cur().ed.Selection()
	if s == e {
		a.status.Hint("请先在正文中选中回收伏笔的段落")
		return
	}
	w.fore.paying, w.fore.noting, w.fore.from, w.fore.to, w.fore.payQuery, w.fore.pick = true, false, s, e, "", id
	a.closeFind()
	w.rewriting = false
}

// pay marks the range asked about as paying off the foreshadowing of id,
// and shows the two together.
func (a *App) pay(id string) {
	w := &a.write
	f := &w.fore
	if !f.paying || w.cur().ed == nil || id == "" {
		return
	}
	w.cur().ed.AddMark(editor.Mark{Kind: editor.MarkPayoff, Start: f.from, End: f.to, Key: markID(), Ref: id,
		At: time.Now().Format(time.RFC3339)})
	f.paying = false
	a.warn(a.saveText())
	a.showFore(id)
	w.cur().ed.Focus()
}

// editMark changes the mark of id in chapter node as fn does, removing it
// when fn says not to keep it: in the editor when the chapter is open, on
// the disk otherwise.
func (a *App) editMark(node, id string, fn func(m *workspace.ChapterMark) (keep bool)) {
	w := &a.write
	if node == w.node && w.cur().ed != nil {
		for _, m := range w.cur().ed.Doc.Marks() {
			if m.Key != id {
				continue
			}
			cm := storedMarks([]editor.Mark{m})[0]
			if fn(&cm) {
				w.cur().ed.Doc.EditMark(m.ID, func(x *editor.Mark) { x.Note, x.Dropped = cm.Note, cm.Dropped })
			} else {
				w.cur().ed.Doc.RemoveMark(m.ID)
			}
		}
		a.warn(a.saveText())
		return
	}
	ch, err := a.Store.ReadChapter(node)
	if err != nil {
		a.warn(err)
		return
	}
	marks := ch.Marks[:0]
	for _, m := range ch.Marks {
		if m.ID != id || fn(&m) {
			marks = append(marks, m)
		}
	}
	_, err = a.book.SaveChapterOver(node, ch.Text, marks, ch.Stamp) // over the text read just now, and no other
	a.warn(err)
}

// renote gives the foreshadowing another note.
func (a *App) renoteFore(f workspace.Foreshadow, note string) {
	a.editMark(f.Node, f.ID, func(m *workspace.ChapterMark) bool {
		m.Note = strings.TrimSpace(note)
		return true
	})
}

// dropFore gives the foreshadowing up, or takes it up again.
func (a *App) dropFore(f workspace.Foreshadow, dropped bool) {
	a.editMark(f.Node, f.ID, func(m *workspace.ChapterMark) bool {
		m.Dropped = dropped
		return true
	})
}

// unpay takes back all that pays the foreshadowing off.
func (a *App) unpay(f workspace.Foreshadow) {
	for _, p := range f.Payoffs {
		a.editMark(p.Node, p.ID, func(*workspace.ChapterMark) bool { return false })
	}
}

// rehang puts a foreshadowing whose sentence was deleted on the selection,
// in whichever chapter is open.
func (a *App) rehang(f workspace.Foreshadow) {
	w := &a.write
	if w.cur().ed == nil || w.cur().ed.ReadOnly() {
		return
	}
	s, e := w.cur().ed.Selection()
	if s == e {
		a.status.Hint("请先在正文中选中要关联的句子")
		return
	}
	if f.Node == w.node {
		for _, m := range w.cur().ed.Doc.Marks() {
			if m.Key == f.ID {
				w.cur().ed.Doc.EditMark(m.ID, func(x *editor.Mark) { x.Start, x.End, x.Orphaned = s, e, false })
			}
		}
	} else {
		a.editMark(f.Node, f.ID, func(*workspace.ChapterMark) bool { return false })
		w.cur().ed.AddMark(editor.Mark{Kind: editor.MarkForeshadow, Start: s, End: e, Note: f.Note, Key: f.ID, At: f.At})
	}
	a.warn(a.saveText())
}

// showFore shows the foreshadowing's details in the right column.
func (a *App) showFore(id string) {
	a.write.fore.shown, a.write.fore.renoting = id, false
	if a.doc.tab != paneFore || a.doc.hidden {
		a.doc.tab, a.doc.hidden = paneFore, false
		a.savePrefs()
	}
}

// showMark opens chapter node and selects what its mark of id covers.
func (a *App) showMark(node, id string) {
	w := &a.write
	if w.node != node {
		a.openNode(node)
	}
	if w.node != node || w.cur().ed == nil {
		return
	}
	for _, m := range w.cur().ed.Doc.Marks() {
		if m.Key == id {
			w.cur().ed.Select(m.Start, m.End)
			w.cur().ed.Focus()
		}
	}
}

// foreAt is the foreshadowing the caret is in, or that what it is in pays
// off.
func (a *App) foreAt() string {
	w := &a.write
	at := w.cur().ed.Caret()
	id, size := "", -1
	for _, m := range w.cur().ed.Doc.Marks() {
		if m.Kind != editor.MarkForeshadow && m.Kind != editor.MarkPayoff || m.Start > at || at > m.End || m.End == m.Start {
			continue
		}
		if size < 0 || m.End-m.Start < size {
			size = m.End - m.Start
			if id = m.Key; m.Kind == editor.MarkPayoff {
				id = m.Ref
			}
		}
	}
	return id
}

// foreMenu is what the chapter's context menu does with foreshadowings.
func (a *App) foreMenu(m *kit.Menu) {
	cmdBury.item(a, m).Do(cmdBury.act(a))
	cmdPay.item(a, m).Do(cmdPay.act(a))
	if id := a.foreAt(); id != "" {
		cmdViewFore.item(a, m).Do(func() { a.showFore(id) })
	}
}

// forePopover floats under the passage being marked: the note of a
// foreshadowing, or the choice of the one a passage pays off.
func (a *App) forePopover(c *ui.Context) {
	w := &a.write
	f := &w.fore
	if !f.noting && !f.paying {
		return
	}
	if w.cur().ed == nil || w.cur().ed.ReadOnly() || w.cur().lost || w.cur().external != nil {
		f.noting, f.paying = false, false
		return
	}
	r, ok := w.cur().ed.RangeBox(f.from, f.to)
	if !ok {
		return
	}
	t := a.pal.Theme
	width, tall := float32(360), float32(132)
	if f.paying {
		width, tall = 380, 340
	}
	top := r.Y + r.H + 6
	if top+tall > w.cur().ed.Height() && r.Y-tall-6 >= 4 {
		top = r.Y - tall - 6
	}
	left := max(8, min(r.X, w.cur().ed.Width()-width-8))
	box := ui.Column(c).Absolute().Left(left).Top(top).Width(width).Background(a.pal.Canvas).Border(1, t.Border)
	kit.PopIn(box, 0, 0)
	a.pal.Pop.On(box)
	if f.noting {
		box.Padding(12).Gap(10).Radius(kit.RCard).Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				kit.Glyph(c, kit.IcFlag, kit.IconXs, t.TextMuted)
				ui.Text(c, "记为伏笔，填写备注").FontSize(kit.FsNote).TextColor(t.TextMuted)
			})
			in := ui.TextInput(c, &f.note).Label("伏笔备注").Placeholder("例如：他为什么突然改口").FillWidth().AutoFocus()
			cancel := in.Shortcut(0, ui.KeyEscape)
			ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Spacer(c)
				if a.pal.Secondary(c, "取消").Label("取消记伏笔").Clicked() {
					cancel = true
				}
				if a.pal.Primary(c, "确定").Clicked() || in.Submitted() {
					a.bury()
				}
			})
			if cancel {
				f.noting = false
				w.cur().ed.Focus()
			}
		})
		return
	}
	list, here := a.book.Tree().ForeshadowCandidates(a.foreshadows(), w.node, f.payQuery)
	if !slices.ContainsFunc(list, func(x workspace.Foreshadow) bool { return x.ID == f.pick }) {
		f.pick = ""
		if len(list) > 0 {
			f.pick = list[0].ID
		}
	}
	box.Padding(10).Gap(6).Radius(kit.RControl).Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Padding(2, 4).Children(func() {
			kit.Glyph(c, kit.IcLink, kit.IconXs, t.TextMuted)
			ui.Text(c, "选择要回收的伏笔").FontSize(kit.FsNote).TextColor(t.TextMuted)
		})
		in := ui.TextInput(c, &f.payQuery).Label("查找伏笔").Placeholder("搜索备注或原句").FillWidth().AutoFocus()
		cancel := in.Shortcut(0, ui.KeyEscape)
		at := slices.IndexFunc(list, func(x workspace.Foreshadow) bool { return x.ID == f.pick })
		if in.Shortcut(0, ui.KeyDown) && at+1 < len(list) {
			f.pick = list[at+1].ID
		}
		if in.Shortcut(0, ui.KeyUp) && at > 0 {
			f.pick = list[at-1].ID
		}
		if len(list) == 0 {
			words := "没有匹配的伏笔"
			if strings.TrimSpace(f.payQuery) == "" {
				words = "暂无已埋下的伏笔。请先选中一句，右键选择「记为伏笔」。"
			}
			ui.Text(c, words).FontSize(kit.FsNote).TextColor(a.pal.Faint).Padding(8).LineHeight(kit.LhBody)
		}
		ui.Scroll(c.Key("pay-list")).FillWidth().MaxHeight(200).Children(func() {
			for _, x := range list {
				row := ui.ButtonBase(c).Column().FillWidth().AlignItems(ui.Stretch).Padding(8).Gap(2).Radius(kit.RControl).Label("回收-" + foreName(x))
				if x.ID == f.pick {
					row.Background(a.pal.Bubble)
				} else if row.Hovered() {
					row.Background(t.SurfaceHover)
				}
				kit.PressRow(c, row)
				row.Children(func() {
					ui.Text(c, foreName(x)).FontSize(kit.FsBody).FontWeight(500).SingleLine().Ellipsis("…").FillWidth()
					ui.Text(c, foreMeta(x, here)).FontSize(kit.FsSmall).TextColor(t.TextMuted).SingleLine().Ellipsis("…").FillWidth()
				})
				if row.Clicked() {
					f.pick = x.ID
				}
			}
		})
		a.pal.Hairline(c)
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Padding(2, 4, 0, 4).Children(func() {
			ui.Spacer(c)
			if a.pal.Secondary(c, "取消").Label("取消回收选择").Clicked() {
				cancel = true
			}
			if a.pal.Primary(c, "记为回收").Label("确定回收").Disabled(f.pick == "").Clicked() || in.Submitted() && f.pick != "" {
				a.pay(f.pick)
			}
		})
		if cancel {
			f.paying = false
			w.cur().ed.Focus()
		}
	})
}

// forePanel is the foreshadowings beside the prose: the list, the page of
// one, or the one being picked for a passage.
func (a *App) forePanel(c *ui.Context) {
	w := &a.write
	a.pal.PaneTop(c).Children(func() {
		a.panelTabs(c)
	})
	all := a.foreshadows()
	find := func(id string) (workspace.Foreshadow, bool) {
		i := slices.IndexFunc(all, func(f workspace.Foreshadow) bool { return f.ID == id })
		if i < 0 || id == "" {
			return workspace.Foreshadow{}, false
		}
		return all[i], true
	}
	if f, ok := find(w.fore.pick); ok && w.fore.paying {
		a.forePicked(c, f)
		return
	}
	if f, ok := find(w.fore.shown); ok {
		a.foreDetail(c, f)
		return
	}
	w.fore.shown = ""
	a.foreList(c, all)
}

// foreList is the foreshadowings not paid off, a row each, under those
// that wait on the author; the paid off and the dropped are a list each,
// reached from the dots.
func (a *App) foreList(c *ui.Context, all []workspace.Foreshadow) {
	s := &a.write.fore
	counts := map[workspace.ForeshadowStatus]int{}
	for _, f := range all {
		counts[f.Status()]++
	}
	row := func(f workspace.Foreshadow) {
		side := f.Title
		if f.Here {
			side = "本章"
		}
		if a.pal.PaneRow(c.Key(f.ID), "伏笔-"+foreName(f), foreName(f), "", side, "", false).Clicked() {
			a.showFore(f.ID)
		}
	}
	if s.status != workspace.ForeOpen {
		if back, _ := a.pal.PaneBack(c, "全部伏笔", ""); back {
			s.status = workspace.ForeOpen
		}
		a.pal.PaneRows(c, fmt.Sprintf("fore-%d", s.status), func() {
			a.pal.PaneGroup(c, foreLabels[s.status])
			for _, f := range all {
				if f.Status() == s.status {
					row(f)
				}
			}
			if counts[s.status] == 0 {
				a.pal.PaneEmpty(c, "此处暂无伏笔", "")
			}
		})
		return
	}
	a.paneFind(c, &s.query, "搜索伏笔", "", func(m *kit.Menu) {
		for _, st := range []workspace.ForeshadowStatus{workspace.ForeDone, workspace.ForeDropped} {
			m.Item(foreLabels[st]).Hint(kit.Count(counts[st])).Do(func() { s.status = st })
		}
	})
	q := strings.TrimSpace(s.query)
	a.pal.PaneRows(c, "fore", func() {
		if len(all) == 0 {
			a.pal.PaneEmpty(c, "暂无伏笔", "在正文中选中一句，右键选择「记为伏笔」")
			return
		}
		if q != "" {
			var found []workspace.Foreshadow
			for _, f := range all {
				if strings.Contains(f.Note, q) || strings.Contains(f.Quote, q) {
					found = append(found, f)
				}
			}
			if len(found) == 0 {
				a.pal.PaneEmpty(c, "未找到", "")
				return
			}
			a.pal.PaneGroup(c, fmt.Sprintf("找到 %d 条", len(found)))
			for _, f := range found {
				row(f)
			}
			return
		}
		var over, fresh []workspace.Foreshadow
		lost := ""
		for _, f := range all {
			switch st := f.Status(); {
			case st == workspace.ForeLost:
				lost = f.ID
			case f.Late(a.write.lateAfter):
				over = append(over, f)
			case st == workspace.ForeOpen:
				fresh = append(fresh, f)
			}
		}
		slices.Reverse(fresh) // the newest of these first, as the design lists them
		if n := counts[workspace.ForeLost]; n > 0 && a.pal.PaneTodo(c, foreLabels[workspace.ForeLost], n).Clicked() {
			if n == 1 {
				a.showFore(lost)
			} else {
				s.status = workspace.ForeLost
			}
		}
		if len(over) > 0 {
			a.pal.PaneGroup(c, "久未回收")
			for _, f := range over {
				row(f)
			}
		}
		if len(fresh) > 0 {
			a.pal.PaneGroup(c, foreLabels[workspace.ForeOpen])
			for _, f := range fresh {
				row(f)
			}
		}
		if len(over)+len(fresh)+counts[workspace.ForeLost] == 0 {
			a.pal.PaneEmpty(c, "没有未回收的伏笔", "")
		}
	})
}

// foreDetail is the page of one foreshadowing: where it was buried, what
// pays it off, and the one thing to do next with it.
func (a *App) foreDetail(c *ui.Context, f workspace.Foreshadow) {
	s := &a.write.fore
	st := f.Status()
	act := map[workspace.ForeshadowStatus]string{workspace.ForeOpen: "放弃伏笔", workspace.ForeDone: "取消回收", workspace.ForeLost: "删除伏笔", workspace.ForeDropped: "恢复伏笔"}[st]
	back, acted := a.pal.PaneBack(c, "全部伏笔", act)
	if back {
		s.shown, s.renoting = "", false
	}
	if acted {
		switch st {
		case workspace.ForeOpen:
			a.dropFore(f, true)
		case workspace.ForeDone:
			a.unpay(f)
		case workspace.ForeDropped:
			a.dropFore(f, false)
		case workspace.ForeLost:
			a.editMark(f.Node, f.ID, func(*workspace.ChapterMark) bool { return false })
			s.shown = ""
		}
	}
	a.pal.PanePage(c, "fore-"+f.ID, func() {
		switch {
		case s.renoting:
			ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
				in := ui.TextInput(c, &s.renote).Label("修改伏笔备注").Placeholder("备注").Grow(1).Shrink(1).AutoFocus()
				if a.pal.PaneButton(c, "保存", true).Label("保存备注").Clicked() || in.Submitted() {
					a.renoteFore(f, s.renote)
					s.renoting = false
				}
				if in.Shortcut(0, ui.KeyEscape) {
					s.renoting = false
				}
			})
		case st == workspace.ForeLost:
			a.pal.PaneTitle(c, foreName(f), "", "")
		default:
			if a.pal.PaneTitle(c, foreName(f), "", "修改备注") {
				s.renoting, s.renote = true, f.Note
			}
		}
		where, link := "埋在「"+f.Title+"」", "查看原文"
		switch {
		case st == workspace.ForeLost:
			where = "「" + f.Title + "」里的原句已删除"
			if f.Here {
				link = "" // the chapter is open, and the sentence is not in it
			}
		case st == workspace.ForeOpen && f.Gap > 0:
			where += fmt.Sprintf("，已过 %d 章", f.Gap)
		}
		if a.pal.PaneWhere(c, where, link) {
			a.showMark(f.Node, f.ID)
		}
		a.pal.PaneQuote(c, f.Quote, f.Lost)
		a.pal.Hairline(c)
		// What takes words picked in the prose waits for them, as the
		// context menu does; the words above it stay as they are, so the
		// pane does not jump while the author picks.
		unpicked := func(b ui.Element) ui.Element {
			none := true
			if ed := a.write.cur().ed; ed != nil {
				s, e := ed.Selection()
				none = s == e || ed.ReadOnly()
			}
			if none {
				kit.Pale(b.Disabled(true))
			}
			return b
		}
		switch st {
		case workspace.ForeLost:
			a.pal.PaneNote(c, "请先在正文中选中新的句子")
			kit.PaneButtons(c, func() {
				if unpicked(a.pal.PaneButton(c, "更换原句", true)).Clicked() {
					a.rehang(f)
				}
			})
		case workspace.ForeOpen:
			ui.Text(c, foreLabels[st]).FontSize(kit.FsBody).FontWeight(600)
			a.pal.PaneNote(c, "请先在正文中选中回收的段落")
			kit.PaneButtons(c, func() {
				if unpicked(a.pal.PaneButton(c, "记为回收", true)).Clicked() {
					a.openPay(f.ID)
				}
			})
		case workspace.ForeDropped:
			ui.Text(c, foreLabels[st]).FontSize(kit.FsBody).FontWeight(600)
		}
		for i, p := range f.Payoffs {
			if st != workspace.ForeDone {
				break
			}
			if a.pal.PaneWhere(c, fmt.Sprintf("回收在「%s」，相隔 %d 章", p.Title, max(p.Chapter-f.Chapter, 0)), fmt.Sprintf("看回收-%d", i+1)) {
				a.showMark(p.Node, p.ID)
			}
			a.pal.PaneQuote(c, p.Quote, false)
		}
	})
}

// forePicked shows the foreshadowing picked for the passage being marked,
// so the author sees what they are about to pay off.
func (a *App) forePicked(c *ui.Context, f workspace.Foreshadow) {
	a.pal.PanePage(c, "fore-picked", func() {
		a.pal.PaneTitle(c, foreName(f), "", "")
		where := "埋在「" + f.Title + "」"
		if f.Gap > 0 && len(f.Payoffs) == 0 {
			where += fmt.Sprintf("，已过 %d 章", f.Gap)
		}
		a.pal.PaneWhere(c, where, "")
		a.pal.PaneQuote(c, f.Quote, false)
		for _, p := range f.Payoffs {
			a.pal.PaneNote(c, "「"+p.Title+"」里已有一处回收")
		}
	})
}

// remindersView is the settings' 提醒与检查: when a foreshadowing counts as
// buried long, and with the AI on, when chapters are checked against the
// setting.
func (a *App) remindersView(c *ui.Context) {
	t := a.pal.Theme
	w := &a.write
	late := workspace.LateThreshold(a.write.lateAfter)
	n := 0
	for _, f := range a.foreshadows() {
		if f.Late(a.write.lateAfter) {
			n++
		}
	}
	a.pal.Subhead(c, "伏笔", "")
	ui.Row(c).FillWidth().MinHeight(kit.HControl).Gap(12).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "超过多少章算久未回收").FontSize(kit.FsBody).LineHeight(kit.LhBody).Grow(1).Shrink(1)
		set := func(v int) {
			w.lateAfter = min(max(v, lateMin), lateMax)
			a.savePrefs()
		}
		// One box: less, how many, more.
		ui.Row(c).Height(kit.HControl).AlignItems(ui.Center).Radius(kit.RControl).Border(1, t.Border).Shrink(0).Children(func() {
			step := func(sign, label, tip string, off bool, to int) {
				b := ui.ButtonBase(c).Width(kit.HControlSm).FillHeight().Center().Label(label).Tooltip(tip).Disabled(off)
				fg := t.TextMuted
				switch {
				case off:
					kit.Pale(b)
				case b.Hovered():
					b.Background(t.SurfaceHover)
					fg = t.Text
				}
				b.Children(func() { ui.Text(c, sign).FontSize(kit.FsBody).TextColor(fg) })
				if b.Clicked() {
					set(to)
				}
			}
			step("−", "提醒章数减少", fmt.Sprintf("减少 %d 章", lateStep), late <= lateMin, late-lateStep)
			ui.Textf(c, "%d 章", late).FontSize(kit.FsControl).Padding(0, 10)
			step("+", "提醒章数增加", fmt.Sprintf("增加 %d 章", lateStep), late >= lateMax, late+lateStep)
		})
	})
	ui.Textf(c, "当前有 %d 个伏笔久未回收。", n).FontSize(kit.FsNote).TextColor(t.TextMuted).LineHeight(kit.LhBody)
	if !a.aiOn() {
		return
	}
	a.pal.Subhead(c, "检查正文和设定", "")
	ui.Column(c).FillWidth().Gap(2).Children(func() { a.whenRows(c) })
}
