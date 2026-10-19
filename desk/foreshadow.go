package desk

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/editor"
	"writer/workspace"
)

// lateDefault is how many chapters may follow a foreshadowing before it is
// listed as buried long, until the author sets another number.
const lateDefault = 30

// The least and the most the author may set that number to, and the step.
const lateMin, lateMax, lateStep = 10, 120, 5

type foreStatus int

const (
	foreOpen    foreStatus = iota // buried, not paid off
	foreDone                      // paid off somewhere
	foreDropped                   // given up
	foreLost                      // its sentence was deleted
)

var foreLabels = map[foreStatus]string{foreOpen: "未回收", foreDone: "已回收", foreDropped: "已放弃", foreLost: "原句已删"}

// payoff is a place that pays a foreshadowing off.
type payoff struct {
	id, node, title, quote, at string
	ch, start, end             int // ch counts the chapters from 1
	here                       bool
}

// foreshadow is a foreshadowing of the book with what pays it off, in the
// order of the chapters.
type foreshadow struct {
	id, node, title, note, quote, at string
	ch, start, end                   int
	here, lost, dropped              bool
	pays                             []payoff
	// gap is how many chapters lie between its chapter and the first that
	// pays it off, or the last of the book while none does.
	gap int
}

func (f foreshadow) status() foreStatus {
	switch {
	case f.dropped:
		return foreDropped
	case f.lost:
		return foreLost
	case len(f.pays) > 0:
		return foreDone
	}
	return foreOpen
}

// name is what the author called it, or the sentence when they wrote
// nothing.
func (f foreshadow) name() string {
	if strings.TrimSpace(f.note) != "" {
		return f.note
	}
	return clip(f.quote, 24)
}

// meta tells where it was buried and how long before chapter here, which
// pays it off, as the picker lists it; the chapter open is 本章.
func (f foreshadow) meta(here int) string {
	paid := len(f.pays) > 0
	switch {
	case f.here && paid:
		return "本章 · 已回收"
	case f.here:
		return "本章埋下"
	case paid:
		return f.title + " · 已回收"
	}
	return fmt.Sprintf("%s · %d 章前埋下", f.title, max(here-f.ch, 0))
}

// foreState is what the foreshadowings' column and popovers show.
type foreState struct {
	// status is the list shown: foreOpen for those not paid off, with
	// those that wait on the author over them, or the list of another.
	status foreStatus
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

// late is how many chapters may follow a foreshadowing before it counts as
// buried long.
func (a *App) late() int {
	if n := a.write.lateAfter; n > 0 {
		return n
	}
	return lateDefault
}

// foreshadows gathers the book's foreshadowings and what pays them off:
// the open chapter's as it is being written, the others' as of the disk.
func (a *App) foreshadows() []foreshadow {
	w := &a.write
	var out []foreshadow
	pays := map[string][]payoff{}
	n := 0
	w.tree.Walk(func(nd *workspace.Node, _ int) {
		if nd.Folder {
			return
		}
		n++
		marks := w.marks[nd.ID]
		here := nd.ID == w.node && w.ed != nil
		if here {
			marks = liveMarks(w.ed.Doc)
		}
		for _, m := range marks {
			switch {
			case m.Kind == workspace.MarkForeshadow:
				out = append(out, foreshadow{id: m.ID, node: nd.ID, title: nd.Title, note: m.Note, quote: m.Quote, at: m.At,
					ch: n, start: m.Start, end: m.End, here: here, lost: m.Orphaned, dropped: m.Dropped})
			case m.Kind == workspace.MarkPayoff && !m.Orphaned && m.Of != "":
				pays[m.Of] = append(pays[m.Of], payoff{id: m.ID, node: nd.ID, title: nd.Title, quote: m.Quote, at: m.At,
					ch: n, start: m.Start, end: m.End, here: here})
			}
		}
	})
	for i := range out {
		f := &out[i]
		f.pays = pays[f.id]
		f.gap = n - f.ch
		if len(f.pays) > 0 {
			f.gap = max(f.pays[0].ch-f.ch, 0)
		}
	}
	return out
}

func markID() string {
	var b [5]byte
	rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// openBury asks for the note of a foreshadowing on the selection.
func (a *App) openBury() {
	w := &a.write
	s, e := w.ed.Selection()
	if s == e || w.ed.ReadOnly() {
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
	if !f.noting || w.ed == nil {
		return
	}
	id := markID()
	w.ed.AddMark(editor.Mark{Kind: editor.MarkForeshadow, Start: f.from, End: f.to, Note: strings.TrimSpace(f.note),
		Key: id, At: time.Now().Format(time.RFC3339)})
	f.noting = false
	a.warn(a.saveText())
	w.ed.Focus()
}

// openPay asks which foreshadowing the selection pays off, the one of id
// picked when there is one.
func (a *App) openPay(id string) {
	w := &a.write
	if w.ed == nil || w.ed.ReadOnly() {
		return
	}
	s, e := w.ed.Selection()
	if s == e {
		a.hint = "请先在正文中选中回收伏笔的段落"
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
	if !f.paying || w.ed == nil || id == "" {
		return
	}
	w.ed.AddMark(editor.Mark{Kind: editor.MarkPayoff, Start: f.from, End: f.to, Key: markID(), Ref: id,
		At: time.Now().Format(time.RFC3339)})
	f.paying = false
	a.warn(a.saveText())
	a.showFore(id)
	w.ed.Focus()
}

// candidates are the foreshadowings a passage of the chapter open may pay
// off, those matching the search: all buried in it or before it but the
// lost and the dropped. here counts the chapters up to the open one.
func (a *App) candidates() (out []foreshadow, here int) {
	w := &a.write
	q := strings.TrimSpace(w.fore.payQuery)
	n := 0
	w.tree.Walk(func(nd *workspace.Node, _ int) {
		if !nd.Folder {
			n++
			if nd.ID == w.node {
				here = n
			}
		}
	})
	for _, f := range a.foreshadows() {
		if st := f.status(); st == foreLost || st == foreDropped || f.ch > here {
			continue
		}
		if q == "" || strings.Contains(f.note, q) || strings.Contains(f.quote, q) {
			out = append(out, f)
		}
	}
	return out, here
}

// editMark changes the mark of id in chapter node as fn does, removing it
// when fn says not to keep it: in the editor when the chapter is open, on
// the disk otherwise.
func (a *App) editMark(node, id string, fn func(m *workspace.ChapterMark) (keep bool)) {
	w := &a.write
	if node == w.node && w.ed != nil {
		for _, m := range w.ed.Doc.Marks() {
			if m.Key != id {
				continue
			}
			cm := storedMarks([]editor.Mark{m})[0]
			if fn(&cm) {
				w.ed.Doc.EditMark(m.ID, func(x *editor.Mark) { x.Note, x.Dropped = cm.Note, cm.Dropped })
			} else {
				w.ed.Doc.RemoveMark(m.ID)
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
	_, err = a.Store.WriteChapterOver(node, ch.Text, marks, ch.Stamp) // over the text read just now, and no other
	a.warn(err)
	a.scanMarks()
}

// renote gives the foreshadowing another note.
func (a *App) renoteFore(f foreshadow, note string) {
	a.editMark(f.node, f.id, func(m *workspace.ChapterMark) bool {
		m.Note = strings.TrimSpace(note)
		return true
	})
}

// dropFore gives the foreshadowing up, or takes it up again.
func (a *App) dropFore(f foreshadow, dropped bool) {
	a.editMark(f.node, f.id, func(m *workspace.ChapterMark) bool {
		m.Dropped = dropped
		return true
	})
}

// unpay takes back all that pays the foreshadowing off.
func (a *App) unpay(f foreshadow) {
	for _, p := range f.pays {
		a.editMark(p.node, p.id, func(*workspace.ChapterMark) bool { return false })
	}
}

// rehang puts a foreshadowing whose sentence was deleted on the selection,
// in whichever chapter is open.
func (a *App) rehang(f foreshadow) {
	w := &a.write
	if w.ed == nil || w.ed.ReadOnly() {
		return
	}
	s, e := w.ed.Selection()
	if s == e {
		a.hint = "请先在正文中选中要关联的句子"
		return
	}
	if f.node == w.node {
		for _, m := range w.ed.Doc.Marks() {
			if m.Key == f.id {
				w.ed.Doc.EditMark(m.ID, func(x *editor.Mark) { x.Start, x.End, x.Orphaned = s, e, false })
			}
		}
	} else {
		a.editMark(f.node, f.id, func(*workspace.ChapterMark) bool { return false })
		w.ed.AddMark(editor.Mark{Kind: editor.MarkForeshadow, Start: s, End: e, Note: f.note, Key: f.id, At: f.at})
	}
	a.warn(a.saveText())
}

// showFore shows the foreshadowing's details in the right column.
func (a *App) showFore(id string) {
	a.write.fore.shown, a.write.fore.renoting = id, false
	if !a.doc.fore || a.doc.hidden {
		a.doc.fore, a.doc.hidden = true, false
		a.savePrefs()
	}
}

// showMark opens chapter node and selects what its mark of id covers.
func (a *App) showMark(node, id string) {
	w := &a.write
	if w.node != node {
		a.openNode(node)
	}
	if w.node != node || w.ed == nil {
		return
	}
	for _, m := range w.ed.Doc.Marks() {
		if m.Key == id {
			w.ed.Select(m.Start, m.End)
			w.ed.Focus()
		}
	}
}

// foreAt is the foreshadowing the caret is in, or that what it is in pays
// off.
func (a *App) foreAt() string {
	w := &a.write
	at := w.ed.Caret()
	id, size := "", -1
	for _, m := range w.ed.Doc.Marks() {
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
func (a *App) foreMenu(m *menu) {
	w := &a.write
	s, e := w.ed.Selection()
	busy := w.ed.ReadOnly()
	m.Item("记为伏笔…").Disabled(s == e || busy).Do(a.openBury)
	m.Item("记为回收…").Disabled(s == e || busy).Do(func() { a.openPay("") })
	if id := a.foreAt(); id != "" {
		m.Item("查看伏笔").Do(func() { a.showFore(id) })
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
	if w.ed == nil || w.ed.ReadOnly() || w.lost || w.external != nil {
		f.noting, f.paying = false, false
		return
	}
	r, ok := w.ed.RangeBox(f.from, f.to)
	if !ok {
		return
	}
	t := a.pal.theme
	width, tall := float32(360), float32(132)
	if f.paying {
		width, tall = 380, 340
	}
	top := r.Y + r.H + 6
	if top+tall > w.ed.Height() && r.Y-tall-6 >= 4 {
		top = r.Y - tall - 6
	}
	left := max(8, min(r.X, w.ed.Width()-width-8))
	box := ui.Column(c).Absolute().Left(left).Top(top).Width(width).Background(a.pal.canvas).Border(1, t.Border)
	a.pal.pop.on(box)
	if f.noting {
		box.Padding(12).Gap(10).Radius(rCard).Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				glyph(c, icFlag, iconXs, t.TextMuted)
				ui.Text(c, "记为伏笔，填写备注").FontSize(fsNote).TextColor(t.TextMuted)
			})
			in := ui.TextInput(c, &f.note).Label("伏笔备注").Placeholder("例如：他为什么突然改口").FillWidth().AutoFocus()
			cancel := in.Shortcut(0, ui.KeyEscape)
			ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Spacer(c)
				if a.pal.secondary(c, "取消").Label("取消记伏笔").Clicked() {
					cancel = true
				}
				if a.pal.primary(c, "确定").Clicked() || in.Submitted() {
					a.bury()
				}
			})
			if cancel {
				f.noting = false
				w.ed.Focus()
			}
		})
		return
	}
	list, here := a.candidates()
	if !slices.ContainsFunc(list, func(x foreshadow) bool { return x.id == f.pick }) {
		f.pick = ""
		if len(list) > 0 {
			f.pick = list[0].id
		}
	}
	box.Padding(10).Gap(6).Radius(rControl).Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Padding(2, 4).Children(func() {
			glyph(c, icLink, iconXs, t.TextMuted)
			ui.Text(c, "选择要回收的伏笔").FontSize(fsNote).TextColor(t.TextMuted)
		})
		in := ui.TextInput(c, &f.payQuery).Label("查找伏笔").Placeholder("搜索备注或原句").FillWidth().AutoFocus()
		cancel := in.Shortcut(0, ui.KeyEscape)
		at := slices.IndexFunc(list, func(x foreshadow) bool { return x.id == f.pick })
		if in.Shortcut(0, ui.KeyDown) && at+1 < len(list) {
			f.pick = list[at+1].id
		}
		if in.Shortcut(0, ui.KeyUp) && at > 0 {
			f.pick = list[at-1].id
		}
		if len(list) == 0 {
			words := "没有匹配的伏笔"
			if strings.TrimSpace(f.payQuery) == "" {
				words = "暂无已埋下的伏笔。请先选中一句，右键选择「记为伏笔」。"
			}
			ui.Text(c, words).FontSize(fsNote).TextColor(a.pal.faint).Padding(8).LineHeight(lhBody)
		}
		ui.Scroll(c.Key("pay-list")).FillWidth().MaxHeight(200).Children(func() {
			for _, x := range list {
				row := ui.ButtonBase(c).Column().FillWidth().AlignItems(ui.Stretch).Padding(8).Gap(2).Radius(rControl).Label("回收-" + x.name())
				if x.id == f.pick {
					row.Background(a.pal.bubble)
				} else if row.Hovered() {
					row.Background(t.SurfaceHover)
				}
				row.Children(func() {
					ui.Text(c, x.name()).FontSize(fsBody).FontWeight(500).SingleLine().Ellipsis("…").FillWidth()
					ui.Text(c, x.meta(here)).FontSize(fsSmall).TextColor(t.TextMuted).SingleLine().Ellipsis("…").FillWidth()
				})
				if row.Clicked() {
					f.pick = x.id
				}
			}
		})
		a.pal.hairline(c)
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Padding(2, 4, 0, 4).Children(func() {
			ui.Spacer(c)
			if a.pal.secondary(c, "取消").Label("取消回收选择").Clicked() {
				cancel = true
			}
			if a.pal.primary(c, "记为回收").Label("确定回收").Disabled(f.pick == "").Clicked() || in.Submitted() && f.pick != "" {
				a.pay(f.pick)
			}
		})
		if cancel {
			f.paying = false
			w.ed.Focus()
		}
	})
}

// forePanel is the foreshadowings beside the prose: the list, the page of
// one, or the one being picked for a passage.
func (a *App) forePanel(c *ui.Context) {
	w := &a.write
	paneTop(c).Children(func() {
		a.panelTabs(c)
		ui.Spacer(c)
		if iconButton(c, icPanel, "收起右栏", ui.Color{}).Tooltip("收起右栏").Clicked() {
			a.setDocHidden(true)
		}
	})
	all := a.foreshadows()
	find := func(id string) (foreshadow, bool) {
		i := slices.IndexFunc(all, func(f foreshadow) bool { return f.id == id })
		if i < 0 || id == "" {
			return foreshadow{}, false
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
func (a *App) foreList(c *ui.Context, all []foreshadow) {
	s := &a.write.fore
	late := a.late()
	counts := map[foreStatus]int{}
	for _, f := range all {
		counts[f.status()]++
	}
	row := func(f foreshadow) {
		side := f.title
		if f.here {
			side = "本章"
		}
		if a.paneRow(c.Key(f.id), "伏笔-"+f.name(), f.name(), "", side, "", false).Clicked() {
			a.showFore(f.id)
		}
	}
	if s.status != foreOpen {
		if back, _ := a.paneBack(c, "全部伏笔", ""); back {
			s.status = foreOpen
		}
		a.paneRows(c, fmt.Sprintf("fore-%d", s.status), func() {
			a.paneGroup(c, foreLabels[s.status])
			for _, f := range all {
				if f.status() == s.status {
					row(f)
				}
			}
			if counts[s.status] == 0 {
				a.paneEmpty(c, "此处暂无伏笔", "")
			}
		})
		return
	}
	a.paneFind(c, &s.query, "搜索伏笔", "", func(m *menu) {
		for _, st := range []foreStatus{foreDone, foreDropped} {
			m.Item(foreLabels[st]).Hint(count(counts[st])).Do(func() { s.status = st })
		}
	})
	q := strings.TrimSpace(s.query)
	a.paneRows(c, "fore", func() {
		if len(all) == 0 {
			a.paneEmpty(c, "暂无伏笔", "在正文中选中一句，右键选择「记为伏笔」")
			return
		}
		if q != "" {
			var found []foreshadow
			for _, f := range all {
				if strings.Contains(f.note, q) || strings.Contains(f.quote, q) {
					found = append(found, f)
				}
			}
			if len(found) == 0 {
				a.paneEmpty(c, "未找到", "")
				return
			}
			a.paneGroup(c, fmt.Sprintf("找到 %d 条", len(found)))
			for _, f := range found {
				row(f)
			}
			return
		}
		var over, fresh []foreshadow
		lost := ""
		for _, f := range all {
			switch st := f.status(); {
			case st == foreLost:
				lost = f.id
			case st == foreOpen && f.gap >= late:
				over = append(over, f)
			case st == foreOpen:
				fresh = append(fresh, f)
			}
		}
		slices.Reverse(fresh) // the newest of these first, as the design lists them
		if n := counts[foreLost]; n > 0 && a.paneTodo(c, foreLabels[foreLost], n).Clicked() {
			if n == 1 {
				a.showFore(lost)
			} else {
				s.status = foreLost
			}
		}
		if len(over) > 0 {
			a.paneGroup(c, "久未回收")
			for _, f := range over {
				row(f)
			}
		}
		if len(fresh) > 0 {
			a.paneGroup(c, foreLabels[foreOpen])
			for _, f := range fresh {
				row(f)
			}
		}
		if len(over)+len(fresh)+counts[foreLost] == 0 {
			a.paneEmpty(c, "没有未回收的伏笔", "")
		}
	})
}

// foreDetail is the page of one foreshadowing: where it was buried, what
// pays it off, and the one thing to do next with it.
func (a *App) foreDetail(c *ui.Context, f foreshadow) {
	s := &a.write.fore
	st := f.status()
	act := map[foreStatus]string{foreOpen: "放弃伏笔", foreDone: "取消回收", foreLost: "删除伏笔", foreDropped: "恢复伏笔"}[st]
	back, acted := a.paneBack(c, "全部伏笔", act)
	if back {
		s.shown, s.renoting = "", false
	}
	if acted {
		switch st {
		case foreOpen:
			a.dropFore(f, true)
		case foreDone:
			a.unpay(f)
		case foreDropped:
			a.dropFore(f, false)
		case foreLost:
			a.editMark(f.node, f.id, func(*workspace.ChapterMark) bool { return false })
			s.shown = ""
		}
	}
	a.panePage(c, "fore-"+f.id, func() {
		switch {
		case s.renoting:
			ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
				in := ui.TextInput(c, &s.renote).Label("修改伏笔备注").Placeholder("备注").Grow(1).Shrink(1).AutoFocus()
				if a.paneButton(c, "保存", true).Label("保存备注").Clicked() || in.Submitted() {
					a.renoteFore(f, s.renote)
					s.renoting = false
				}
				if in.Shortcut(0, ui.KeyEscape) {
					s.renoting = false
				}
			})
		case st == foreLost:
			a.paneTitle(c, f.name(), "", "")
		default:
			if a.paneTitle(c, f.name(), "", "修改备注") {
				s.renoting, s.renote = true, f.note
			}
		}
		where, link := "埋在「"+f.title+"」", "查看原文"
		switch {
		case st == foreLost:
			where = "「" + f.title + "」里的原句已删除"
			if f.here {
				link = "" // the chapter is open, and the sentence is not in it
			}
		case st == foreOpen && f.gap > 0:
			where += fmt.Sprintf("，已过 %d 章", f.gap)
		}
		if a.paneWhere(c, where, link) {
			a.showMark(f.node, f.id)
		}
		a.paneQuote(c, f.quote, f.lost)
		a.pal.hairline(c)
		// What takes words picked in the prose waits for them, as the
		// context menu does; the words above it stay as they are, so the
		// pane does not jump while the author picks.
		unpicked := func(b ui.Element) ui.Element {
			none := true
			if ed := a.write.ed; ed != nil {
				s, e := ed.Selection()
				none = s == e || ed.ReadOnly()
			}
			if none {
				pale(b.Disabled(true))
			}
			return b
		}
		switch st {
		case foreLost:
			a.paneNote(c, "请先在正文中选中新的句子")
			paneButtons(c, func() {
				if unpicked(a.paneButton(c, "更换原句", true)).Clicked() {
					a.rehang(f)
				}
			})
		case foreOpen:
			ui.Text(c, foreLabels[st]).FontSize(fsBody).FontWeight(600)
			a.paneNote(c, "请先在正文中选中回收的段落")
			paneButtons(c, func() {
				if unpicked(a.paneButton(c, "记为回收", true)).Clicked() {
					a.openPay(f.id)
				}
			})
		case foreDropped:
			ui.Text(c, foreLabels[st]).FontSize(fsBody).FontWeight(600)
		}
		for i, p := range f.pays {
			if st != foreDone {
				break
			}
			if a.paneWhere(c, fmt.Sprintf("回收在「%s」，相隔 %d 章", p.title, max(p.ch-f.ch, 0)), fmt.Sprintf("看回收-%d", i+1)) {
				a.showMark(p.node, p.id)
			}
			a.paneQuote(c, p.quote, false)
		}
	})
}

// forePicked shows the foreshadowing picked for the passage being marked,
// so the author sees what they are about to pay off.
func (a *App) forePicked(c *ui.Context, f foreshadow) {
	a.panePage(c, "fore-picked", func() {
		a.paneTitle(c, f.name(), "", "")
		where := "埋在「" + f.title + "」"
		if f.gap > 0 && len(f.pays) == 0 {
			where += fmt.Sprintf("，已过 %d 章", f.gap)
		}
		a.paneWhere(c, where, "")
		a.paneQuote(c, f.quote, false)
		for _, p := range f.pays {
			a.paneNote(c, "「"+p.title+"」里已有一处回收")
		}
	})
}

// remindersView is the settings' 提醒与检查: when a foreshadowing counts as
// buried long, and with the AI on, when chapters are checked against the
// setting.
func (a *App) remindersView(c *ui.Context) {
	t := a.pal.theme
	w := &a.write
	late := a.late()
	n := 0
	for _, f := range a.foreshadows() {
		if f.status() == foreOpen && f.gap >= late {
			n++
		}
	}
	a.subhead(c, "伏笔", "")
	ui.Row(c).FillWidth().MinHeight(hControl).Gap(12).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).Shrink(1).Children(func() {
			ui.Text(c, "超过多少章未回收时提醒").FontSize(fsBody).LineHeight(lhBody)
			ui.Text(c, "到期后归入「久未回收」，不弹窗。").
				FontSize(fsNote).TextColor(t.TextMuted).LineHeight(lhBody).FillWidth()
		})
		set := func(v int) {
			w.lateAfter = min(max(v, lateMin), lateMax)
			a.savePrefs()
		}
		// One box: less, how many, more.
		ui.Row(c).Height(hControl).AlignItems(ui.Center).Radius(rControl).Border(1, t.Border).Shrink(0).Children(func() {
			step := func(sign, label, tip string, off bool, to int) {
				b := ui.ButtonBase(c).Width(hControlSm).FillHeight().Center().Label(label).Tooltip(tip).Disabled(off)
				fg := t.TextMuted
				switch {
				case off:
					pale(b)
				case b.Hovered():
					b.Background(t.SurfaceHover)
					fg = t.Text
				}
				b.Children(func() { ui.Text(c, sign).FontSize(fsBody).TextColor(fg) })
				if b.Clicked() {
					set(to)
				}
			}
			step("−", "提醒章数减少", fmt.Sprintf("减少 %d 章", lateStep), late <= lateMin, late-lateStep)
			ui.Textf(c, "%d 章", late).FontSize(fsControl).Padding(0, 10)
			step("+", "提醒章数增加", fmt.Sprintf("增加 %d 章", lateStep), late >= lateMax, late+lateStep)
		})
	})
	ui.Textf(c, "当前有 %d 个伏笔久未回收。", n).FontSize(fsNote).TextColor(t.TextMuted).LineHeight(lhBody)
	if !a.aiOn() {
		return
	}
	a.subhead(c, "检查正文和设定", "")
	ui.Column(c).FillWidth().Gap(2).Children(func() { a.whenRows(c) })
}
