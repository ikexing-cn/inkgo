package desk

import (
	"time"

	"github.com/egoist/mygo/ui"
	"writer/desk/book"
	"writer/desk/kit"
	"writer/locales"
	"writer/prefs"
	"writer/workspace"
)

// bookKey is everything a count read from the book depends on.
type bookKey struct {
	book    *book.Book
	version int
}

func (a *App) bookKey() bookKey { return bookKey{a.book, a.book.Version()} }

// countAsTyped keeps the open chapter's count up with its text, so the
// counts move as the author types rather than when the text is saved.
func (a *App) countAsTyped() {
	w := &a.write
	if w.cur().ed == nil || a.book.Counts() == nil || w.cur().lost || w.cur().external != nil {
		return
	}
	if w.cur().ed.Doc == w.counted && w.cur().ed.Doc.Version() == w.countedVer {
		return
	}
	w.counted, w.countedVer = w.cur().ed.Doc, w.cur().ed.Doc.Version()
	a.book.SetCount(w.node, a.Store.Work().Language.Count(w.cur().ed.Doc.String()))
}

func (a *App) total() int {
	return a.write.total.get(a.bookKey(), func() int {
		n := 0
		for _, k := range a.book.Counts() {
			n += k
		}
		return n
	})
}

// wordsIn is a chapter's count, or for a folder all its chapters' together.
func (a *App) wordsIn(n *workspace.Node) int {
	words := a.write.nodeWords.get(a.bookKey(), func() map[string]int {
		return nodeWords(a.book.Tree().Root, a.book.Counts())
	})
	return words[n.ID]
}

func (a *App) today(now time.Time) int {
	if a.book.Counts() == nil {
		return 0 // not read yet: a start of nothing would count the whole book
	}
	return a.write.counter.Today(a.total(), now)
}

func (a *App) lastDays(now time.Time, n int) []int {
	days := a.write.counter.LastDays(a.total(), now, n)
	if a.book.Counts() == nil {
		days[n-1] = 0
	}
	return days
}

func (a *App) setGoal(text string) {
	if a.write.counter.SetGoal(text) {
		a.savePrefs()
	}
}

func (a *App) rollDay(now time.Time) bool {
	if a.book.Counts() == nil {
		return false
	}
	return a.write.counter.Roll(a.total(), now)
}

// wordCount is at the right of the foot: the words of what is shown or
// selected, of the book, and of today against the goal. It opens the last
// seven days and the goal.
func (a *App) wordCount(c *ui.Context) {
	t := a.pal.Theme
	w := &a.write
	n, ok := a.book.Tree().Node(w.node)
	here := 0
	if ok {
		here = a.wordsIn(n)
	}
	scope := locales.StatusChapterWords(here, a.countUnit())
	if ok && n.Folder {
		scope = locales.StatusVolumeWords(kit.Words(here, a.countLanguage()))
	}
	if w.cur().ed != nil {
		if sel := w.cur().ed.SelectedText(); sel != "" {
			here = a.Store.Work().Language.Count(sel)
			scope = locales.StatusSelected(here, a.countUnit())
		}
	}
	book := locales.StatusBookWords(kit.Words(a.total(), a.countLanguage()))
	today := ""
	k := a.today(time.Now())
	tip := locales.StatusCountTip(book, k, kit.Signed(k), a.countUnit())
	if w.counter.Goal > 0 {
		today = locales.StatusTodayGoal(k, w.counter.Goal)
		tip = locales.StatusCountGoalTip(book, k, kit.Signed(k), w.counter.Goal, a.countUnit())
	} else if k != 0 {
		today = locales.StatusTodayCount(k, kit.Signed(k))
	}
	color := a.pal.Faint // as the design has it, met or not: the numbers say so
	counts := ui.ButtonBase(c).Row().Shrink(1000).MinWidth(0).Label(locales.StatusWordCount(a.countUnit())).Tooltip(tip)
	a.slideEdge(counts)
	if counts.Hovered() {
		color = t.Text
	}
	counts.Children(func() {
		if ok {
			ui.Text(c, locales.StatusCountLine(scope, book, today)).FontSize(kit.FsNote).TextColor(color).SingleLine().Ellipsis("…").Shrink(1).MinWidth(0)
		} else {
			ui.Text(c, locales.StatusBookCountLine(book, today)).FontSize(kit.FsNote).TextColor(color).SingleLine().Ellipsis("…").Shrink(1).MinWidth(0)
		}
	})
	if counts.Clicked() {
		w.goalOpen = !w.goalOpen
	}
	// It opens upward: the foot is the last line of the window.
	kit.Popover(c, counts, &w.goalOpen, 1, 1, func(panel ui.Element) {
		panel.AttachTo(counts, ui.AnchorTopRight, ui.AnchorBottomRight).Margin(0, 0, 4, 0).Width(250).Radius(kit.RCard).Padding(8).Gap(2).
			Background(a.pal.Raised).Border(1, t.Border)
		a.pal.Pop.On(panel)
		a.lastWeek(c)
		ui.Box(c).FillWidth().Height(1).Margin(4, 0).Background(a.pal.Line).Shrink(0)
		ui.Column(c).FillWidth().Gap(6).Padding(4, 6).Children(func() {
			ui.Text(c, locales.StatusDailyGoal()).FontSize(kit.FsNote).TextColor(t.TextMuted)
			in := ui.TextInput(c, &w.goalText).Label(locales.StatusGoalLabel(a.countUnit())).Placeholder(locales.StatusGoalHint()).FillWidth().AutoFocus()
			if in.Changed() {
				a.setGoal(w.goalText)
			}
			if in.Submitted() {
				w.goalOpen = false
			}
		})
		// What is not a count leaves the goal be, and says so.
		note, ink := locales.StatusGoalNote(a.countUnit()), t.TextMuted
		if _, ok := prefs.CountOrNone(w.goalText); !ok {
			note, ink = locales.StatusGoalError(), t.Danger
		}
		ui.Text(c, note).FontSize(kit.FsNote).TextColor(ink).LineHeight(kit.LhBody).FillWidth().Padding(0, 6, 4, 6)
	})
}

// lastWeek lists how much was written on each of the last seven days, today
// first.
func (a *App) lastWeek(c *ui.Context) {
	now := time.Now()
	days := a.lastDays(now, 7)
	ui.Text(c, locales.StatusLast7()).FontSize(kit.FsSmall).TextColor(a.pal.Faint).Padding(6, 10, 2, 10)
	for i := len(days) - 1; i >= 0; i-- {
		day, k := now.AddDate(0, 0, i-len(days)+1), days[i]
		got := locales.StatusWords(0, a.countUnit())
		if k != 0 {
			got = locales.StatusSignedWords(k, kit.Signed(k), a.countUnit())
		}
		ui.Row(c.Key(i)).FillWidth().Padding(4, 10).Label(locales.StatusDayWritten(kit.DayOf(day, now), k, a.countUnit())).Children(func() {
			ui.Text(c, kit.DayOf(day, now)).FontSize(kit.FsNote).TextColor(a.pal.Theme.TextMuted).Grow(1)
			ui.Text(c, got).FontSize(kit.FsNote).TextColor(a.pal.Theme.TextMuted)
		})
	}
}

// nodeWords is the count of every node under root: a chapter's own, or for
// a folder all its chapters' together.
func nodeWords(root *workspace.Node, counts map[string]int) map[string]int {
	out := map[string]int{}
	var add func(n *workspace.Node) int
	add = func(n *workspace.Node) int {
		k := counts[n.ID]
		if n.Folder {
			k = 0
			for _, ch := range n.Children {
				k += add(ch)
			}
		}
		out[n.ID] = k
		return k
	}
	if root != nil {
		for _, ch := range root.Children {
			add(ch)
		}
	}
	return out
}

// countUnit identifies the current book counting unit.
func (a *App) countUnit() string { return a.countLanguage().Unit() }

// countLanguage supplies Chinese for the shelf, where no book is open.
func (a *App) countLanguage() workspace.Language {
	if a.Store == nil {
		return workspace.Chinese
	}
	return a.Store.Work().Language
}
