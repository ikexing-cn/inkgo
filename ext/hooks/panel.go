package hooks

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"
	"time"
	"writer/ext/hooks/hookstext"

	"github.com/egoist/mygo/ui"
	"writer/desk/book"
	"writer/desk/kit"
	"writer/editor"
	"writer/locales"
	"writer/workspace"
)

// The least and the most the author may set that number to, and the step.
const lateMin, lateMax, lateStep = 10, 120, 5

var foreLabels = map[workspace.ForeshadowStatus]func() string{workspace.ForeOpen: hookstext.HooksUnpaid, workspace.ForeDone: hookstext.HooksDone, workspace.ForeDropped: locales.HooksDropped, workspace.ForeLost: hookstext.HooksLost}

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
		return hookstext.EditorMetaHerePaid()
	case f.Here:
		return hookstext.EditorMetaHere()
	case paid:
		return hookstext.EditorMetaPaid(f.Title)
	}
	return hookstext.EditorMetaBefore(f.Title, max(here-f.Chapter, 0))
}

// State is what the foreshadowings' column and popovers show.
type State struct {
	// Status is the list Shown: workspace.ForeOpen for those not paid off, with
	// those that wait on the author over them, or the list of another.
	Status workspace.ForeshadowStatus
	Query  string // what the list is searched for
	Shown  string // the foreshadowing whose page is Shown

	// Burying: the range To mark and its Note.
	Noting   bool
	From, To int
	Note     string

	// Paying off: the range that does, what the foreshadowings are
	// searched for, and the one picked.
	Paying   bool
	PayQuery string
	Pick     string

	Renoting bool // the Note of the one Shown is being changed
	Renote   string
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

func (a *Panel) foreshadowKey() foreshadowKey {
	doc := 0
	if a.current().ed != nil {
		doc = a.current().ed.Doc.Version()
	}
	return foreshadowKey{a.book(), a.book().Version(), a.current().ed, doc, a.node()}
}

// foreshadows are the book's foreshadowings and what pays them off
// (workspace.Tree.Foreshadows), the open chapter's as it is being written.
func (a *Panel) foreshadows() []workspace.Foreshadow {
	if a.book() == nil {
		return nil
	}
	key := a.foreshadowKey()
	return a.fores.get(key, func() []workspace.Foreshadow {
		current := ""
		var live []workspace.ChapterMark
		if a.current().ed != nil {
			current, live = a.node(), liveMarks(a.current().ed.Doc)
		}
		return a.book().Tree().Foreshadows(a.book().Marks(), current, live)
	})
}

func markID() string {
	var b [5]byte
	rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// openBury asks for the note of a foreshadowing on the selection.
func (a *Panel) openBury() {
	w := &a.State
	s, e := a.current().ed.Selection()
	if s == e || a.current().ed.ReadOnly() {
		return
	}
	a.services.Prose.CloseSelectionTools()
	(*w).Noting, (*w).Paying, (*w).From, (*w).To, (*w).Note = true, false, s, e, ""
}

// bury marks the range asked about as a foreshadowing with the note
// written.
func (a *Panel) bury() {
	w := &a.State
	f := &(*w)
	if !f.Noting || a.current().ed == nil {
		return
	}
	id := markID()
	a.current().ed.AddMark(editor.Mark{Kind: editor.MarkForeshadow, Start: f.From, End: f.To, Note: strings.TrimSpace(f.Note),
		Key: id, At: time.Now().Format(time.RFC3339)})
	f.Noting = false
	a.warn(a.services.Book.SaveText())
	a.current().ed.Focus()
}

// openPay asks which foreshadowing the selection pays off, the one of id
// picked when there is one.
func (a *Panel) openPay(id string) {
	w := &a.State
	if a.current().ed == nil || a.current().ed.ReadOnly() {
		return
	}
	s, e := a.current().ed.Selection()
	if s == e {
		a.services.Notices.Notices().Hint(hookstext.EditorSelectFirst())
		return
	}
	a.services.Prose.CloseSelectionTools()
	(*w).Paying, (*w).Noting, (*w).From, (*w).To, (*w).PayQuery, (*w).Pick = true, false, s, e, "", id
}

// pay marks the range asked about as paying off the foreshadowing of id,
// and shows the two together.
func (a *Panel) pay(id string) {
	w := &a.State
	f := &(*w)
	if !f.Paying || a.current().ed == nil || id == "" {
		return
	}
	a.current().ed.AddMark(editor.Mark{Kind: editor.MarkPayoff, Start: f.From, End: f.To, Key: markID(), Ref: id,
		At: time.Now().Format(time.RFC3339)})
	f.Paying = false
	a.warn(a.services.Book.SaveText())
	a.showFore(id)
	a.current().ed.Focus()
}

// editMark changes the mark of id in chapter node as fn does, removing it
// when fn says not to keep it: in the editor when the chapter is open, on
// the disk otherwise.
func (a *Panel) editMark(node, id string, fn func(m *workspace.ChapterMark) (keep bool)) {
	if node == a.node() && a.current().ed != nil {
		for _, m := range a.current().ed.Doc.Marks() {
			if m.Key != id {
				continue
			}
			cm := storedMarks([]editor.Mark{m})[0]
			if fn(&cm) {
				a.current().ed.Doc.EditMark(m.ID, func(x *editor.Mark) { x.Note, x.Dropped = cm.Note, cm.Dropped })
			} else {
				a.current().ed.Doc.RemoveMark(m.ID)
			}
		}
		a.warn(a.services.Book.SaveText())
		return
	}
	ch, err := a.services.Book.Store().ReadChapter(node)
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
	_, err = a.book().SaveChapterOver(node, ch.Text, marks, ch.Stamp) // over the text read just now, and no other
	a.warn(err)
}

// renote gives the foreshadowing another note.
func (a *Panel) renoteFore(f workspace.Foreshadow, note string) {
	a.editMark(f.Node, f.ID, func(m *workspace.ChapterMark) bool {
		m.Note = strings.TrimSpace(note)
		return true
	})
}

// dropFore gives the foreshadowing up, or takes it up again.
func (a *Panel) dropFore(f workspace.Foreshadow, dropped bool) {
	a.editMark(f.Node, f.ID, func(m *workspace.ChapterMark) bool {
		m.Dropped = dropped
		return true
	})
}

// unpay takes back all that pays the foreshadowing off.
func (a *Panel) unpay(f workspace.Foreshadow) {
	for _, p := range f.Payoffs {
		a.editMark(p.Node, p.ID, func(*workspace.ChapterMark) bool { return false })
	}
}

// rehang puts a foreshadowing whose sentence was deleted on the selection,
// in whichever chapter is open.
func (a *Panel) rehang(f workspace.Foreshadow) {
	if a.current().ed == nil || a.current().ed.ReadOnly() {
		return
	}
	s, e := a.current().ed.Selection()
	if s == e {
		a.services.Notices.Notices().Hint(hookstext.HooksSelectFirst())
		return
	}
	if f.Node == a.node() {
		for _, m := range a.current().ed.Doc.Marks() {
			if m.Key == f.ID {
				a.current().ed.Doc.EditMark(m.ID, func(x *editor.Mark) { x.Start, x.End, x.Orphaned = s, e, false })
			}
		}
	} else {
		a.editMark(f.Node, f.ID, func(*workspace.ChapterMark) bool { return false })
		a.current().ed.AddMark(editor.Mark{Kind: editor.MarkForeshadow, Start: s, End: e, Note: f.Note, Key: f.ID, At: f.At})
	}
	a.warn(a.services.Book.SaveText())
}

// showFore shows the foreshadowing's details in the right column.
func (a *Panel) showFore(id string) {
	a.State.Shown, a.State.Renoting = id, false
	a.services.Pane.Show("fore")
}

// showMark opens chapter node and selects what its mark of id covers.
func (a *Panel) showMark(node, id string) {
	if a.node() != node {
		a.services.Desktop.OpenWriting(node)
	}
	if a.node() != node || a.current().ed == nil {
		return
	}
	for _, m := range a.current().ed.Doc.Marks() {
		if m.Key == id {
			a.current().ed.Select(m.Start, m.End)
			a.current().ed.Focus()
		}
	}
}

// foreAt is the foreshadowing the caret is in, or that what it is in pays
// off.
func (a *Panel) foreAt() string {
	at := a.current().ed.Caret()
	id, size := "", -1
	for _, m := range a.current().ed.Doc.Marks() {
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

// forePopover floats under the passage being marked: the note of a
// foreshadowing, or the choice of the one a passage pays off.
func (a *Panel) forePopover(c *ui.Context) {
	w := &a.State
	f := &(*w)
	if !f.Noting && !f.Paying {
		return
	}
	if a.current().ed == nil || a.current().ed.ReadOnly() || a.lost() || a.services.Desktop.ChapterExternal() {
		f.Noting, f.Paying = false, false
		return
	}
	r, ok := a.current().ed.RangeBox(f.From, f.To)
	if !ok {
		return
	}
	t := a.pal().Theme
	width, tall := float32(360), float32(132)
	if f.Paying {
		width, tall = 380, 340
	}
	top := r.Y + r.H + 6
	if top+tall > a.current().ed.Height() && r.Y-tall-6 >= 4 {
		top = r.Y - tall - 6
	}
	left := max(8, min(r.X, a.current().ed.Width()-width-8))
	box := ui.Column(c).Absolute().Left(left).Top(top).Width(width).Background(a.pal().Canvas).Border(1, t.Border)
	kit.PopIn(box, 0, 0)
	a.pal().Pop.On(box)
	if f.Noting {
		box.Padding(12).Gap(10).Radius(kit.RCard).Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				kit.Glyph(c, kit.IcFlag, kit.IconXs, t.TextMuted)
				ui.Text(c, hookstext.EditorBuryTitle()).FontSize(kit.FsNote).TextColor(t.TextMuted)
			})
			in := ui.TextInput(c, &f.Note).Label(hookstext.EditorBuryLabel()).Placeholder(hookstext.EditorBuryHint()).FillWidth().AutoFocus()
			cancel := in.Shortcut(0, ui.KeyEscape)
			ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Spacer(c)
				if a.pal().Secondary(c, locales.CommonCancel()).Label("取消记伏笔").Clicked() {
					cancel = true
				}
				if a.pal().Primary(c, hookstext.CommonConfirm()).Clicked() || in.Submitted() {
					a.bury()
				}
			})
			if cancel {
				f.Noting = false
				a.current().ed.Focus()
			}
		})
		return
	}
	list, here := a.book().Tree().ForeshadowCandidates(a.foreshadows(), a.node(), f.PayQuery)
	if !slices.ContainsFunc(list, func(x workspace.Foreshadow) bool { return x.ID == f.Pick }) {
		f.Pick = ""
		if len(list) > 0 {
			f.Pick = list[0].ID
		}
	}
	box.Padding(10).Gap(6).Radius(kit.RControl).Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Padding(2, 4).Children(func() {
			kit.Glyph(c, kit.IcLink, kit.IconXs, t.TextMuted)
			ui.Text(c, hookstext.EditorPayTitle()).FontSize(kit.FsNote).TextColor(t.TextMuted)
		})
		in := ui.TextInput(c, &f.PayQuery).Label(hookstext.EditorPayLabel()).Placeholder(hookstext.EditorPayHint()).FillWidth().AutoFocus()
		cancel := in.Shortcut(0, ui.KeyEscape)
		at := slices.IndexFunc(list, func(x workspace.Foreshadow) bool { return x.ID == f.Pick })
		if in.Shortcut(0, ui.KeyDown) && at+1 < len(list) {
			f.Pick = list[at+1].ID
		}
		if in.Shortcut(0, ui.KeyUp) && at > 0 {
			f.Pick = list[at-1].ID
		}
		if len(list) == 0 {
			words := hookstext.EditorNoMatch()
			if strings.TrimSpace(f.PayQuery) == "" {
				words = hookstext.EditorNoMarks()
			}
			ui.Text(c, words).FontSize(kit.FsNote).TextColor(a.pal().Faint).Padding(8).LineHeight(kit.LhBody)
		}
		ui.Scroll(c.Key("pay-list")).FillWidth().MaxHeight(200).Children(func() {
			for _, x := range list {
				row := ui.ButtonBase(c).Column().FillWidth().AlignItems(ui.Stretch).Padding(8).Gap(2).Radius(kit.RControl).Label("回收-" + foreName(x))
				if x.ID == f.Pick {
					row.Background(a.pal().Bubble)
				} else if row.Hovered() {
					row.Background(t.SurfaceHover)
				}
				kit.PressRow(c, row)
				row.Children(func() {
					ui.Text(c, foreName(x)).FontSize(kit.FsBody).FontWeight(500).SingleLine().Ellipsis("…").FillWidth()
					ui.Text(c, foreMeta(x, here)).FontSize(kit.FsSmall).TextColor(t.TextMuted).SingleLine().Ellipsis("…").FillWidth()
				})
				if row.Clicked() {
					f.Pick = x.ID
				}
			}
		})
		a.pal().Hairline(c)
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Padding(2, 4, 0, 4).Children(func() {
			ui.Spacer(c)
			if a.pal().Secondary(c, locales.CommonCancel()).Label("取消回收选择").Clicked() {
				cancel = true
			}
			if a.pal().Primary(c, hookstext.HooksMarkPaid()).Label("确定回收").Disabled(f.Pick == "").Clicked() || in.Submitted() && f.Pick != "" {
				a.pay(f.Pick)
			}
		})
		if cancel {
			f.Paying = false
			a.current().ed.Focus()
		}
	})
}

// forePanel is the foreshadowings beside the prose: the list, the page of
// one, or the one being picked for a passage.
func (a *Panel) forePanel(c *ui.Context) {
	w := &a.State
	all := a.foreshadows()
	find := func(id string) (workspace.Foreshadow, bool) {
		i := slices.IndexFunc(all, func(f workspace.Foreshadow) bool { return f.ID == id })
		if i < 0 || id == "" {
			return workspace.Foreshadow{}, false
		}
		return all[i], true
	}
	if f, ok := find((*w).Pick); ok && (*w).Paying {
		a.forePicked(c, f)
		return
	}
	if f, ok := find((*w).Shown); ok {
		a.foreDetail(c, f)
		return
	}
	(*w).Shown = ""
	a.foreList(c, all)
}

// foreList is the foreshadowings not paid off, a row each, under those
// that wait on the author; the paid off and the dropped are a list each,
// reached from the dots.
func (a *Panel) foreList(c *ui.Context, all []workspace.Foreshadow) {
	s := &a.State
	counts := map[workspace.ForeshadowStatus]int{}
	for _, f := range all {
		counts[f.Status()]++
	}
	row := func(f workspace.Foreshadow) {
		side := f.Title
		if f.Here {
			side = locales.ExportRangeChapter()
		}
		if a.pal().PaneRow(c.Key(f.ID), "伏笔-"+foreName(f), foreName(f), "", side, "", false).Clicked() {
			a.showFore(f.ID)
		}
	}
	if s.Status != workspace.ForeOpen {
		if back, _ := a.pal().PaneBack(c, hookstext.HooksAll(), ""); back {
			s.Status = workspace.ForeOpen
		}
		a.pal().PaneRows(c, fmt.Sprintf("fore-%d", s.Status), func() {
			a.pal().PaneGroup(c, foreLabels[s.Status]())
			for _, f := range all {
				if f.Status() == s.Status {
					row(f)
				}
			}
			if counts[s.Status] == 0 {
				a.pal().PaneEmpty(c, hookstext.HooksNoneHere(), "")
			}
		})
		return
	}
	a.services.Prose.PaneFind(c, &s.Query, hookstext.HooksSearch(), "", func(m *kit.Menu) {
		for _, st := range []workspace.ForeshadowStatus{workspace.ForeDone, workspace.ForeDropped} {
			m.Item(foreLabels[st]()).Hint(kit.Count(counts[st])).Do(func() { s.Status = st })
		}
	})
	q := strings.TrimSpace(s.Query)
	a.pal().PaneRows(c, "fore", func() {
		if len(all) == 0 {
			a.pal().PaneEmpty(c, hookstext.HooksEmpty(), hookstext.HooksEmptyHow())
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
				a.pal().PaneEmpty(c, locales.CommonNotFound(), "")
				return
			}
			a.pal().PaneGroup(c, locales.CommonFound(len(found)))
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
			case f.Late(a.late.Get()):
				over = append(over, f)
			case st == workspace.ForeOpen:
				fresh = append(fresh, f)
			}
		}
		slices.Reverse(fresh) // the newest of these first, as the design lists them
		if n := counts[workspace.ForeLost]; n > 0 && a.pal().PaneTodo(c, foreLabels[workspace.ForeLost](), n).Clicked() {
			if n == 1 {
				a.showFore(lost)
			} else {
				s.Status = workspace.ForeLost
			}
		}
		if len(over) > 0 {
			a.pal().PaneGroup(c, hookstext.HooksLong())
			for _, f := range over {
				row(f)
			}
		}
		if len(fresh) > 0 {
			a.pal().PaneGroup(c, foreLabels[workspace.ForeOpen]())
			for _, f := range fresh {
				row(f)
			}
		}
		if len(over)+len(fresh)+counts[workspace.ForeLost] == 0 {
			a.pal().PaneEmpty(c, hookstext.HooksNoneOpen(), "")
		}
	})
}

// foreDetail is the page of one foreshadowing: where it was buried, what
// pays it off, and the one thing to do next with it.
func (a *Panel) foreDetail(c *ui.Context, f workspace.Foreshadow) {
	s := &a.State
	st := f.Status()
	act := map[workspace.ForeshadowStatus]string{workspace.ForeOpen: hookstext.HooksDrop(), workspace.ForeDone: hookstext.HooksCancelPayoff(), workspace.ForeLost: hookstext.HooksDelete(), workspace.ForeDropped: hookstext.HooksRestore()}[st]
	back, acted := a.pal().PaneBack(c, hookstext.HooksAll(), act)
	if back {
		s.Shown, s.Renoting = "", false
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
			s.Shown = ""
		}
	}
	a.pal().PanePage(c, "fore-"+f.ID, func() {
		switch {
		case s.Renoting:
			ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
				in := ui.TextInput(c, &s.Renote).Label(hookstext.HooksEditNote()).Placeholder(hookstext.HooksNote()).Grow(1).Shrink(1).AutoFocus()
				if a.pal().PaneButton(c, locales.CmdSave(), true).Label("保存备注").Clicked() || in.Submitted() {
					a.renoteFore(f, s.Renote)
					s.Renoting = false
				}
				if in.Shortcut(0, ui.KeyEscape) {
					s.Renoting = false
				}
			})
		case st == workspace.ForeLost:
			a.pal().PaneTitle(c, foreName(f), "", "")
		default:
			if a.pal().PaneTitle(c, foreName(f), "", hookstext.HooksEditNoteBtn()) {
				s.Renoting, s.Renote = true, f.Note
			}
		}
		where, link := hookstext.HooksPlanted(f.Title), locales.CheckSeeText()
		switch {
		case st == workspace.ForeLost:
			where = hookstext.HooksLostIn(f.Title)
			if f.Here {
				link = "" // the chapter is open, and the sentence is not in it
			}
		case st == workspace.ForeOpen && f.Gap > 0:
			where = hookstext.HooksPlantedGap(f.Title, f.Gap)
		}
		if a.pal().PaneWhere(c, where, link) {
			a.showMark(f.Node, f.ID)
		}
		a.pal().PaneQuote(c, f.Quote, f.Lost)
		a.pal().Hairline(c)
		// What takes words picked in the prose waits for them, as the
		// context menu does; the words above it stay as they are, so the
		// pane does not jump while the author picks.
		unpicked := func(b ui.Element) ui.Element {
			none := true
			if ed := a.current().ed; ed != nil {
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
			a.pal().PaneNote(c, hookstext.HooksSelectNew())
			kit.PaneButtons(c, func() {
				if unpicked(a.pal().PaneButton(c, hookstext.HooksReplaceQuote(), true)).Clicked() {
					a.rehang(f)
				}
			})
		case workspace.ForeOpen:
			ui.Text(c, foreLabels[st]()).FontSize(kit.FsBody).FontWeight(600)
			a.pal().PaneNote(c, hookstext.HooksSelectPayoff())
			kit.PaneButtons(c, func() {
				if unpicked(a.pal().PaneButton(c, hookstext.HooksMarkPaid(), true)).Clicked() {
					a.openPay(f.ID)
				}
			})
		case workspace.ForeDropped:
			ui.Text(c, foreLabels[st]()).FontSize(kit.FsBody).FontWeight(600)
		}
		for i, p := range f.Payoffs {
			if st != workspace.ForeDone {
				break
			}
			if a.pal().PaneWhere(c, hookstext.HooksPaidIn(p.Title, max(p.Chapter-f.Chapter, 0)), fmt.Sprintf("看回收-%d", i+1)) {
				a.showMark(p.Node, p.ID)
			}
			a.pal().PaneQuote(c, p.Quote, false)
		}
	})
}

// forePicked shows the foreshadowing picked for the passage being marked,
// so the author sees what they are about to pay off.
func (a *Panel) forePicked(c *ui.Context, f workspace.Foreshadow) {
	a.pal().PanePage(c, "fore-picked", func() {
		a.pal().PaneTitle(c, foreName(f), "", "")
		where := hookstext.HooksPlanted(f.Title)
		if f.Gap > 0 && len(f.Payoffs) == 0 {
			where = hookstext.HooksPlantedGap(f.Title, f.Gap)
		}
		a.pal().PaneWhere(c, where, "")
		a.pal().PaneQuote(c, f.Quote, false)
		for _, p := range f.Payoffs {
			a.pal().PaneNote(c, hookstext.HooksAlreadyPaid(p.Title))
		}
	})
}

// remindersView is the settings' 提醒与检查: when a foreshadowing counts as
// buried long, and with the AI on, when chapters are checked against the
// setting.
func (a *Panel) remindersView(c *ui.Context) {
	t := a.pal().Theme
	late := workspace.LateThreshold(a.late.Get())
	n := 0
	for _, f := range a.foreshadows() {
		if f.Late(a.late.Get()) {
			n++
		}
	}
	a.pal().Subhead(c, hookstext.PaneTabHooks(), "")
	ui.Row(c).FillWidth().MinHeight(kit.HControl).Gap(12).AlignItems(ui.Center).Children(func() {
		ui.Text(c, hookstext.SettingsLateAfter()).FontSize(kit.FsBody).LineHeight(kit.LhBody).Grow(1).Shrink(1)
		set := func(v int) {
			a.late.Set(min(max(v, lateMin), lateMax))
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
			step("−", "提醒章数减少", hookstext.SettingsLateLess(), late <= lateMin, late-lateStep)
			ui.Text(c, hookstext.SettingsLateChapters(late)).FontSize(kit.FsControl).Padding(0, 10)
			step("+", "提醒章数增加", hookstext.SettingsLateMore(), late >= lateMax, late+lateStep)
		})
	})
	ui.Text(c, hookstext.SettingsLateNow(n)).FontSize(kit.FsNote).TextColor(t.TextMuted).LineHeight(kit.LhBody)
}
