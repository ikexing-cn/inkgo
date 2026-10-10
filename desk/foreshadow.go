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

var foreLabels = map[foreStatus]string{foreOpen: "未回收", foreDone: "已回收", foreDropped: "已放弃", foreLost: "原句被删"}

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

// meta tells where it was buried and how long ago, as the list does.
func (f foreshadow) meta() string {
	switch {
	case len(f.pays) > 0:
		return fmt.Sprintf("第%d章埋，第%d章回收", f.ch, f.pays[0].ch)
	case f.gap == 0:
		return fmt.Sprintf("第%d章 · 本章埋下", f.ch)
	}
	return fmt.Sprintf("第%d章 · %d 章前埋下", f.ch, f.gap)
}

// chapterName names the ch-th chapter with its title: 第3章《井》, or the
// title alone when it numbers itself.
func chapterName(ch int, title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return fmt.Sprintf("第%d章", ch)
	}
	if numbered.MatchString(title) {
		return title
	}
	return fmt.Sprintf("第%d章《%s》", ch, title)
}

// foreState is what the foreshadowings' column and popovers show.
type foreState struct {
	status int    // the part of the list shown: open, paid off, dropped
	query  string // what the list is searched for
	recent bool   // the newest first, not the chapters' order
	shown  string // the foreshadowing whose details are shown

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

func (a *App) foreshadowOf(id string) (foreshadow, bool) {
	if id == "" {
		return foreshadow{}, false
	}
	for _, f := range a.foreshadows() {
		if f.id == id {
			return f, true
		}
	}
	return foreshadow{}, false
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
	w.rewriting, w.finding = false, false
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
		a.notice = "先在正文里选中回收伏笔的那一段"
		return
	}
	w.fore.paying, w.fore.noting, w.fore.from, w.fore.to, w.fore.payQuery, w.fore.pick = true, false, s, e, "", id
	w.rewriting, w.finding = false, false
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

// candidates are the foreshadowings a passage may pay off, those matching
// the search: all but the lost and the dropped.
func (a *App) candidates() []foreshadow {
	q := strings.TrimSpace(a.write.fore.payQuery)
	var out []foreshadow
	for _, f := range a.foreshadows() {
		if st := f.status(); st == foreLost || st == foreDropped {
			continue
		}
		if q == "" || strings.Contains(f.note, q) || strings.Contains(f.quote, q) {
			out = append(out, f)
		}
	}
	return out
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
	_, err = a.Store.WriteChapter(node, ch.Text, marks)
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
		a.notice = "先在正文里选中要挂上去的句子"
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
func (a *App) foreMenu(m *ui.Menu) {
	w := &a.write
	s, e := w.ed.Selection()
	busy := w.ed.ReadOnly()
	m.Separator()
	if m.Item("记为伏笔…").Disabled(s == e || busy).Chosen() {
		a.openBury()
	}
	if m.Item("这里回收了伏笔…").Disabled(s == e || busy).Chosen() {
		a.openPay("")
	}
	if id := a.foreAt(); id != "" && m.Item("查看这条伏笔").Chosen() {
		a.showFore(id)
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
				ui.Text(c, "把这句记为伏笔，写一句备注").FontSize(fsNote).TextColor(t.TextMuted)
			})
			in := ui.TextInput(c, &f.note).Label("伏笔备注").Placeholder("比如：小祥为什么突然改口").FillWidth().AutoFocus()
			cancel := in.Shortcut(0, ui.KeyEscape)
			ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Spacer(c)
				if ghost(c, "取消", ui.Color{}).Label("取消记伏笔").Clicked() {
					cancel = true
				}
				if a.pal.primary(c, "记下").Clicked() || in.Submitted() {
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
	list := a.candidates()
	if !slices.ContainsFunc(list, func(x foreshadow) bool { return x.id == f.pick }) {
		f.pick = ""
		if len(list) > 0 {
			f.pick = list[0].id
		}
	}
	box.Padding(10).Gap(6).Radius(rControl).Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Padding(2, 4).Children(func() {
			glyph(c, icLink, iconXs, t.TextMuted)
			ui.Text(c, "这段回收的是哪个伏笔？").FontSize(fsNote).TextColor(t.TextMuted)
		})
		in := ui.TextInput(c, &f.payQuery).Label("找伏笔").Placeholder("搜备注或原句").FillWidth().AutoFocus()
		cancel := in.Shortcut(0, ui.KeyEscape)
		at := slices.IndexFunc(list, func(x foreshadow) bool { return x.id == f.pick })
		if in.Shortcut(0, ui.KeyDown) && at+1 < len(list) {
			f.pick = list[at+1].id
		}
		if in.Shortcut(0, ui.KeyUp) && at > 0 {
			f.pick = list[at-1].id
		}
		if len(list) == 0 {
			words := "没有叫这个的伏笔"
			if strings.TrimSpace(f.payQuery) == "" {
				words = "还没有埋下的伏笔。先选中一句，右键「记为伏笔」。"
			}
			ui.Text(c, words).FontSize(fsNote).TextColor(a.pal.faint).Padding(8).LineHeight(lhBody)
		}
		ui.Scroll(c).Key("pay-list").FillWidth().MaxHeight(200).Children(func() {
			for _, x := range list {
				row := ui.ButtonBase(c).Column().FillWidth().AlignItems(ui.Stretch).Padding(8).Gap(2).Radius(rControl).Label("回收-" + x.name())
				if x.id == f.pick {
					row.Background(a.pal.bubble)
				} else if row.Hovered() {
					row.Background(t.SurfaceHover)
				}
				row.Children(func() {
					ui.Text(c, x.name()).FontSize(fsBody).FontWeight(500).SingleLine().Ellipsis("…").FillWidth()
					ui.Text(c, x.meta()).FontSize(fsSmall).TextColor(t.TextMuted).SingleLine()
				})
				if row.Clicked() {
					f.pick = x.id
				}
			}
		})
		a.pal.hairline(c)
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Padding(2, 4, 0, 4).Children(func() {
			ui.Spacer(c)
			if ghost(c, "取消", ui.Color{}).Label("取消回收选择").Clicked() {
				cancel = true
			}
			if a.pal.primary(c, "确定").Disabled(f.pick == "").Clicked() || in.Submitted() && f.pick != "" {
				a.pay(f.pick)
			}
		})
		if cancel {
			f.paying = false
			w.ed.Focus()
		}
	})
}

// forePanel is the foreshadowings beside the prose: the list, the details
// of one, or the one being picked for a passage.
func (a *App) forePanel(c *ui.Context) {
	w := &a.write
	paneTop(c).Children(func() {
		a.panelTabs(c)
		ui.Spacer(c)
		a.bookDot(c)
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

// foreList is the book's foreshadowings by state, those waiting on the
// author first.
func (a *App) foreList(c *ui.Context, all []foreshadow) {
	t := a.pal.theme
	s := &a.write.fore
	late := a.late()
	counts := map[foreStatus]int{}
	for _, f := range all {
		counts[f.status()]++
	}
	if len(all) == 0 {
		ui.Column(c).FillWidth().Grow(1).Center().Gap(8).Padding(0, 20).Children(func() {
			glyph(c, icFlag, iconLg, a.pal.faint)
			ui.Text(c, "还没有伏笔").FontSize(fsBody).FontWeight(600)
			ui.Text(c, "在正文里选中一句，右键「记为伏笔」，写一句备注。以后回收时再选中回收的那段，右键「这里回收了伏笔」。").
				FontSize(fsNote).TextColor(t.TextMuted).LineHeight(lhLoose).TextAlign(ui.Center)
		})
		return
	}
	ui.TextInput(c, &s.query).Label("搜索伏笔").Placeholder("搜索").FillWidth()
	ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
		for i, st := range []foreStatus{foreOpen, foreDone, foreDropped} {
			label := fmt.Sprintf("%s %d", foreLabels[st], counts[st])
			b := ui.ButtonBase(c).Row().Padding(4, 8).Radius(rControl).Shrink(0).Label("伏笔-" + foreLabels[st])
			fg := t.TextMuted
			if i == s.status {
				b.Padding(4, 10).Background(a.pal.ink)
				fg = a.pal.onInk
			} else {
				b.Border(1, t.Border)
				if b.Hovered() {
					b.Background(t.SurfaceHover)
				}
			}
			b.Children(func() { ui.Text(c, label).FontSize(fsNote).TextColor(fg).SingleLine() })
			if b.Clicked() {
				s.status = i
			}
		}
		ui.Spacer(c)
		order := "按章节顺序"
		if s.recent {
			order = "最近埋下的在前"
		}
		sort := ui.ButtonBase(c).Row().Gap(4).AlignItems(ui.Center).Padding(4, 0).Shrink(1).Label("伏笔排序")
		sort.Children(func() {
			ui.Text(c, order).FontSize(fsNote).TextColor(t.TextMuted).SingleLine()
			glyph(c, icChevronDown, iconXs, a.pal.faint)
		})
		sort.Menu(func(m *ui.Menu) {
			if m.Item("按章节顺序").Checked(!s.recent).Chosen() {
				s.recent = false
			}
			if m.Item("最近埋下的在前").Checked(s.recent).Chosen() {
				s.recent = true
			}
		})
	})
	q := strings.TrimSpace(s.query)
	pick := func(want func(foreshadow) bool) []foreshadow {
		var out []foreshadow
		for _, f := range all {
			if want(f) && (q == "" || strings.Contains(f.note, q) || strings.Contains(f.quote, q)) {
				out = append(out, f)
			}
		}
		if s.recent {
			slices.Reverse(out)
		}
		return out
	}
	is := func(st foreStatus) func(foreshadow) bool {
		return func(f foreshadow) bool { return f.status() == st }
	}
	head := func(text string, n int, first bool) {
		top := float32(14)
		if first {
			top = 4
		}
		ui.Textf(c, "%s · %d", text, n).FontSize(fsNote).TextColor(t.TextMuted).Padding(top, 0, 4, 0)
	}
	ui.Scroll(c).Key(fmt.Sprintf("fore-%d", s.status)).FillWidth().Grow(1).Shrink(1).Children(func() {
		ui.Column(c).FillWidth().Gap(2).Children(func() {
			shown := 0
			switch s.status {
			case 0:
				lost := pick(is(foreLost))
				over := pick(func(f foreshadow) bool { return f.status() == foreOpen && f.gap >= late })
				fresh := pick(func(f foreshadow) bool { return f.status() == foreOpen && f.gap < late })
				if !s.recent {
					slices.Reverse(fresh) // the newest of these first, as the design lists them
				}
				shown = len(lost) + len(over) + len(fresh)
				if len(lost) > 0 {
					head("等你确认", len(lost), true)
					for _, f := range lost {
						a.foreLostCard(c, f)
					}
				}
				if len(over) > 0 {
					head(fmt.Sprintf("埋了 %d 章以上", late), len(over), len(lost) == 0)
					for _, f := range over {
						a.foreRow(c, f, true)
					}
				}
				if len(fresh) > 0 {
					text := "其他未回收"
					if len(over) == 0 {
						text = "未回收"
					}
					head(text, len(fresh), len(lost)+len(over) == 0)
					for _, f := range fresh {
						a.foreRow(c, f, false)
					}
				}
			case 1:
				list := pick(is(foreDone))
				shown = len(list)
				for _, f := range list {
					a.foreRow(c, f, false)
				}
			default:
				list := pick(is(foreDropped))
				shown = len(list)
				for _, f := range list {
					a.foreRow(c, f, false)
				}
			}
			if shown == 0 {
				words := "这里还没有"
				if q != "" {
					words = "没有找到"
				}
				ui.Text(c, words).FontSize(fsNote).TextColor(a.pal.faint).Padding(20, 0)
			}
		})
	})
}

// foreLostCard is a foreshadowing that waits on the author: its sentence
// was deleted.
func (a *App) foreLostCard(c *ui.Context, f foreshadow) {
	t := a.pal.theme
	b := ui.ButtonBase(c).Column().FillWidth().AlignItems(ui.Stretch).Padding(10).Gap(4).Radius(rControl).
		Background(a.pal.canvas).Border(1, a.pal.line).Label("伏笔-" + f.name())
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.Children(func() {
		ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Text(c, f.name()).FontSize(fsBody).FontWeight(500).SingleLine().Ellipsis("…").Grow(1).Shrink(1)
			ui.Text(c, foreLabels[foreLost]).FontSize(fsSmall).TextColor(t.TextMuted).SingleLine()
		})
		ui.Textf(c, "第%d章埋伏笔的那句被删了", f.ch).FontSize(fsNote).TextColor(t.TextMuted)
	})
	if b.Clicked() {
		a.showFore(f.id)
	}
}

// foreRow is a foreshadowing in the list: its note, its sentence, and
// where and how long ago it was buried.
func (a *App) foreRow(c *ui.Context, f foreshadow, over bool) {
	t := a.pal.theme
	b := ui.ButtonBase(c).Column().FillWidth().AlignItems(ui.Stretch).Margin(0, -10).Padding(8, 10).Gap(2).Radius(rControl).Label("伏笔-" + f.name())
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	meta := a.pal.faint
	if over {
		meta = t.Text
	}
	b.Children(func() {
		ui.Text(c, f.name()).FontSize(fsBody).FontWeight(500).SingleLine().Ellipsis("…").FillWidth()
		ui.Text(c, f.quote).FontSize(fsNote).TextColor(t.TextMuted).SingleLine().Ellipsis("…").FillWidth()
		ui.Text(c, f.meta()).FontSize(fsSmall).TextColor(meta).SingleLine()
	})
	if b.Clicked() {
		a.showFore(f.id)
	}
}

// foreQuote is a passage of the book quoted in the column.
func (a *App) foreQuote(c *ui.Context, text string, gone bool) {
	t := a.pal.theme
	q := ui.Text(c, text).FontSize(fsBody).LineHeight(lhRead).FillWidth().Padding(10, 12).Radius(rControl).
		Background(a.pal.bubble).Border(1, t.SurfacePressed).Selectable()
	if gone {
		q.TextColor(a.pal.faint).Strikethrough()
	}
}

// foreLink is the small ink link at the end of a line of the details.
func (a *App) foreLink(c *ui.Context, text string) bool {
	t := a.pal.theme
	b := ui.ButtonBase(c).Row().Shrink(0).Label(text)
	fg := t.Text
	if b.Hovered() {
		fg = t.AccentHover
	}
	b.Children(func() { ui.Text(c, text).FontSize(fsNote).TextColor(fg).SingleLine() })
	return b.Clicked()
}

// day is the month and day of a mark's time, or "" when it has none.
func day(at string) string {
	t, err := time.Parse(time.RFC3339, at)
	if err != nil {
		return ""
	}
	return t.Local().Format("01-02")
}

// foreDetail shows one foreshadowing: where it was buried, what pays it
// off, and what the author may do with it.
func (a *App) foreDetail(c *ui.Context, f foreshadow) {
	t := a.pal.theme
	w := &a.write
	s := &w.fore
	st := f.status()
	ui.Scroll(c).Key("fore-" + f.id).FillWidth().Grow(1).Shrink(1).Children(func() {
		ui.Column(c).FillWidth().Gap(14).Padding(0, 4, 8, 4).Children(func() {
			back := ui.ButtonBase(c).Row().Gap(6).AlignItems(ui.Center).Justify(ui.Start).Label("全部伏笔")
			fg := t.TextMuted
			if back.Hovered() {
				fg = t.Text
			}
			back.Children(func() {
				glyph(c, icBack, iconXs, fg)
				ui.Text(c, "全部伏笔").FontSize(fsNote).TextColor(fg)
			})
			if back.Clicked() {
				s.shown, s.renoting = "", false
			}
			if s.renoting {
				ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
					in := ui.TextInput(c, &s.renote).Label("改伏笔备注").Placeholder("备注").Grow(1).Shrink(1).AutoFocus()
					if a.pal.small(c, "保存", true).Label("保存备注").Clicked() || in.Submitted() {
						a.renoteFore(f, s.renote)
						s.renoting = false
					}
					if in.Shortcut(0, ui.KeyEscape) {
						s.renoting = false
					}
				})
			} else {
				ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
					ui.Text(c, f.name()).FontSize(fsGroup).FontWeight(600).Grow(1).Shrink(1)
					ui.Text(c, foreLabels[st]).FontSize(fsNote).TextColor(t.TextMuted).SingleLine()
				})
			}
			ui.Column(c).FillWidth().Gap(6).Children(func() {
				ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
					where := "埋在 " + chapterName(f.ch, f.title)
					if f.here {
						where += " · 当前"
					}
					ui.Text(c, where).FontSize(fsNote).TextColor(t.TextMuted).SingleLine().Ellipsis("…").Grow(1).Shrink(1)
					link := fmt.Sprintf("打开第%d章", f.ch)
					if f.here {
						link = "找到这句"
					}
					if f.lost && f.here {
						link = ""
					}
					if link != "" && a.foreLink(c, link) {
						a.showMark(f.node, f.id)
					}
				})
				a.foreQuote(c, f.quote, f.lost)
			})
			gap := func(text string) {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Padding(0, 0, 0, 12).Children(func() {
					ui.Box(c).Width(1).Height(22).Background(t.Border)
					ui.Text(c, text).FontSize(fsNote).TextColor(a.pal.faint)
				})
			}
			switch {
			case f.lost:
				ui.Text(c, "埋伏笔的那句被删了。在正文里选中另一句，把它挂上去；或者不要这个伏笔。").
					FontSize(fsNote).TextColor(t.TextMuted).LineHeight(lhLoose).FillWidth()
			case len(f.pays) == 0 && !f.dropped:
				gap(fmt.Sprintf("埋了 %d 章，还没回收", f.gap))
			}
			for _, p := range f.pays {
				gap(fmt.Sprintf("相隔 %d 章", max(p.ch-f.ch, 0)))
				ui.Column(c).FillWidth().Gap(6).Children(func() {
					ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
						where := "回收在 " + chapterName(p.ch, p.title)
						link := fmt.Sprintf("打开第%d章", p.ch)
						if p.here {
							where += " · 当前"
							link = "找到这段"
						}
						ui.Text(c, where).FontSize(fsNote).TextColor(t.TextMuted).SingleLine().Ellipsis("…").Grow(1).Shrink(1)
						if a.foreLink(c, link) {
							a.showMark(p.node, p.id)
						}
					})
					a.foreQuote(c, p.quote, false)
				})
			}
			ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Padding(4, 0, 0, 0).Children(func() {
				if st == foreLost {
					if a.pal.secondary(c, "挂到选中的句子上").Clicked() {
						a.rehang(f)
					}
					ui.Spacer(c)
					if ghost(c, "不要这个伏笔", ui.Color{}).Clicked() {
						a.dropFore(f, true)
					}
					return
				}
				if a.pal.secondary(c, "改备注").Clicked() {
					s.renoting, s.renote = true, f.note
				}
				switch st {
				case foreDone:
					if a.pal.secondary(c, "再加一处回收").Clicked() {
						a.openPay(f.id)
					}
					ui.Spacer(c)
					if ghost(c, "取消回收", ui.Color{}).Clicked() {
						a.unpay(f)
					}
				case foreDropped:
					ui.Spacer(c)
					if ghost(c, "重新算作未回收", ui.Color{}).Clicked() {
						a.dropFore(f, false)
					}
				default:
					if a.pal.secondary(c, "在选中处回收").Clicked() {
						a.openPay(f.id)
					}
					ui.Spacer(c)
					if ghost(c, "放弃这个伏笔", ui.Color{}).Clicked() {
						a.dropFore(f, true)
					}
				}
			})
			var dates []string
			if d := day(f.at); d != "" {
				dates = append(dates, d+" 埋下")
			}
			if len(f.pays) > 0 {
				if d := day(f.pays[0].at); d != "" {
					dates = append(dates, d+" 回收")
				}
			}
			if len(dates) > 0 {
				a.pal.hairline(c)
				ui.Text(c, strings.Join(dates, " · ")).FontSize(fsNote).TextColor(a.pal.faint).LineHeight(lhLoose)
			}
		})
	})
}

// forePicked shows the foreshadowing picked for the passage being marked,
// so the author sees what they are about to pay off.
func (a *App) forePicked(c *ui.Context, f foreshadow) {
	t := a.pal.theme
	ui.Column(c).FillWidth().Gap(14).Padding(0, 4).Children(func() {
		ui.Text(c, "选中的伏笔").FontSize(fsNote).TextColor(t.TextMuted)
		ui.Text(c, f.name()).FontSize(fsGroup).FontWeight(600).FillWidth()
		ui.Text(c, "埋在 "+chapterName(f.ch, f.title)).FontSize(fsNote).TextColor(t.TextMuted)
		ui.Text(c, f.quote).FontSize(fsRead).LineHeight(lhRead).FillWidth().Padding(12, 14).Radius(rControl).
			Background(a.pal.canvas).Border(1, a.pal.line)
		for _, p := range f.pays {
			ui.Text(c, "已经在 "+chapterName(p.ch, p.title)+" 回收过一处").FontSize(fsNote).TextColor(a.pal.faint)
		}
	})
}

// remindersView is the settings' 提醒与检查: when a foreshadowing counts as
// buried long.
func (a *App) remindersView(c *ui.Context) {
	t := a.pal.theme
	w := &a.write
	a.prefsHead(c, "提醒与检查", "这部作品的提醒规则，改了立刻生效。")
	late := a.late()
	n := 0
	for _, f := range a.foreshadows() {
		if f.status() == foreOpen && f.gap >= late {
			n++
		}
	}
	ui.Column(c).FillWidth().Children(func() {
		ui.Text(c, "伏笔").FontSize(fsGroup).FontWeight(600).Padding(0, 0, 10, 0)
		a.pal.hairline(c)
		ui.Row(c).FillWidth().Gap(24).Padding(14, 0).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).Shrink(1).Gap(4).Children(func() {
				ui.Text(c, "埋了多久没回收，开始提醒").FontSize(fsBody).FontWeight(500)
				ui.Text(c, "到了这个章数，伏笔在列表里标「埋了很久」，待处理数加 1。不弹窗，不打断打字。").
					FontSize(fsNote).TextColor(t.TextMuted).LineHeight(lhBody).FillWidth()
			})
			set := func(v int) {
				w.lateAfter = min(max(v, lateMin), lateMax)
				a.savePrefs()
			}
			ui.Row(c).Gap(4).AlignItems(ui.Center).Shrink(0).Children(func() {
				if a.pal.secondary(c, "−").Label("提醒章数减少").Disabled(late <= lateMin).Clicked() {
					set(late - lateStep)
				}
				ui.Textf(c, "%d 章", late).FontSize(fsBody).Width(64).TextAlign(ui.Center)
				if a.pal.secondary(c, "+").Label("提醒章数增加").Disabled(late >= lateMax).Clicked() {
					set(late + lateStep)
				}
			})
		})
		a.pal.hairline(c)
		ui.Textf(c, "按你现在的设置，当前有 %d 个伏笔会被标出。", n).FontSize(fsNote).TextColor(a.pal.faint).Padding(10, 0, 0, 0)
	})
}
