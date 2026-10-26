package desk

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/desk/book"
	"writer/desk/kit"
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
	a.book.SetCount(w.node, workspace.Count(w.cur().ed.Doc.String()))
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
	here, unit := 0, "本章"
	if ok && n.Folder {
		unit = "本卷"
	}
	if ok {
		here = a.wordsIn(n)
	}
	if w.cur().ed != nil {
		if sel := w.cur().ed.SelectedText(); sel != "" {
			here, unit = workspace.Count(sel), "选中"
		}
	}
	today, tip := "", "全书 "+kit.Words(a.total())
	k := a.today(time.Now())
	switch {
	case w.counter.Goal > 0:
		today = fmt.Sprintf(" · 今天 %s / %s", kit.Signed(k), kit.Grouped(w.counter.Goal))
		tip += "，今天 " + kit.Signed(k) + " 字，目标 " + kit.Words(w.counter.Goal)
	case k != 0:
		today = " · 今天 " + kit.Signed(k)
		tip += "，今天 " + kit.Signed(k) + " 字"
	}
	color := a.pal.Faint // as the design has it, met or not: the numbers say so
	counts := ui.ButtonBase(c).Row().Shrink(1000).MinWidth(0).Label("字数").Tooltip(tip + "。点击设置每日目标")
	if counts.Hovered() {
		color = t.Text
	}
	counts.Children(func() {
		if ok {
			ui.Text(c, unit+" "+kit.Words(here)+" · 全书 "+kit.Words(a.total())+today).FontSize(kit.FsNote).TextColor(color).SingleLine().Ellipsis("…").Shrink(1).MinWidth(0)
		} else {
			ui.Text(c, "全书 "+kit.Words(a.total())+today).FontSize(kit.FsNote).TextColor(color).SingleLine().Ellipsis("…").Shrink(1).MinWidth(0)
		}
	})
	if counts.Clicked() {
		w.goalOpen = !w.goalOpen
	}
	// It opens upward: the foot is the last line of the window.
	ui.PopoverBase(c, counts, &w.goalOpen, func(panel ui.Element) {
		panel.AttachTo(counts, ui.AnchorTopRight, ui.AnchorBottomRight).Margin(0, 0, 4, 0).Width(250).Radius(kit.RCard).Padding(8).Gap(2).
			Background(a.pal.Raised).Border(1, t.Border)
		a.pal.Pop.On(panel)
		a.lastWeek(c)
		ui.Box(c).FillWidth().Height(1).Margin(4, 0).Background(a.pal.Line).Shrink(0)
		ui.Column(c).FillWidth().Gap(6).Padding(4, 6).Children(func() {
			ui.Text(c, "每日目标").FontSize(kit.FsNote).TextColor(t.TextMuted)
			in := ui.TextInput(c, &w.goalText).Label("每日目标字数").Placeholder("比如 2000").FillWidth().AutoFocus()
			if in.Changed() {
				a.setGoal(w.goalText)
			}
			if in.Submitted() {
				w.goalOpen = false
			}
		})
		// What is not a count leaves the goal be, and says so.
		note, ink := "字数旁显示今日字数和目标。", t.TextMuted
		if _, ok := prefs.CountOrNone(w.goalText); !ok {
			note, ink = "每日目标须为正整数，留空则不设。", t.Danger
		}
		ui.Text(c, note).FontSize(kit.FsNote).TextColor(ink).LineHeight(kit.LhBody).FillWidth().Padding(0, 6, 4, 6)
	})
}

// lastWeek lists how much was written on each of the last seven days, today
// first.
func (a *App) lastWeek(c *ui.Context) {
	now := time.Now()
	days := a.lastDays(now, 7)
	ui.Text(c, "最近 7 天").FontSize(kit.FsSmall).TextColor(a.pal.Faint).Padding(6, 10, 2, 10)
	for i := len(days) - 1; i >= 0; i-- {
		day, k := now.AddDate(0, 0, i-len(days)+1), days[i]
		got := "0 字"
		if k != 0 {
			got = kit.Signed(k) + " 字"
		}
		ui.Row(c.Key(i)).FillWidth().Padding(4, 10).Label(fmt.Sprintf("%s 写了 %d 字", kit.DayOf(day, now), k)).Children(func() {
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
