package desk

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/prefs"
	"writer/workspace"
)

// countAsTyped keeps the open chapter's count up with its text, so the
// counts move as the author types rather than when the text is saved.
func (a *App) countAsTyped() {
	w := &a.write
	if w.ed == nil || w.counts == nil || w.lost || w.external != nil {
		return
	}
	if w.ed.Doc == w.counted && w.ed.Doc.Version() == w.countedVer {
		return
	}
	w.counted, w.countedVer = w.ed.Doc, w.ed.Doc.Version()
	w.counts[w.node] = workspace.Count(w.ed.Doc.String())
}

func (a *App) total() int {
	n := 0
	for _, k := range a.write.counts {
		n += k
	}
	return n
}

func (a *App) today(now time.Time) int {
	if a.write.counts == nil {
		return 0 // not read yet: a start of nothing would count the whole book
	}
	return a.write.counter.Today(a.total(), now, a.savePrefs)
}

func (a *App) lastDays(now time.Time, n int) []int {
	if a.write.counts == nil {
		days := make([]int, n)
		for i := range n - 1 {
			day := now.AddDate(0, 0, i-n+1).Format(time.DateOnly)
			days[i] = a.write.counter.Days[day]
		}
		return days
	}
	return a.write.counter.LastDays(a.total(), now, n, a.savePrefs)
}

func (a *App) setGoal(text string) { a.write.counter.SetGoal(text, a.savePrefs) }

// words says how many characters, the way the design writes them: grouped
// by thousands, and in 万 from ten thousand on.
func words(n int) string {
	if n >= 10000 {
		return strings.TrimSuffix(fmt.Sprintf("%.1f", float64(n)/10000), ".0") + " 万字"
	}
	return grouped(n) + " 字"
}

// grouped writes n with a comma between each three digits.
func grouped(n int) string {
	if n < 0 {
		return "-" + grouped(-n)
	}
	s := strconv.Itoa(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}

// signed is grouped with its sign.
func signed(n int) string {
	if n < 0 {
		return grouped(n)
	}
	return "+" + grouped(n)
}

// wordCount is at the right of the foot: the words of what is shown or
// selected, of the book, and of today against the goal. It opens the last
// seven days and the goal. roomy and wide say how much of it there is room
// for.
func (a *App) wordCount(c *ui.Context, roomy, wide bool) {
	t := a.pal.theme
	w := &a.write
	n, ok := w.tree.Node(w.node)
	here, unit := 0, "本章"
	if ok && n.Folder {
		unit = "本卷"
	}
	if ok {
		here = w.wordsIn(n)
	}
	if w.ed != nil {
		if sel := w.ed.SelectedText(); sel != "" {
			here, unit = workspace.Count(sel), "选中"
		}
	}
	today, tip := "", "全书 "+words(a.total())
	k := a.today(time.Now())
	switch {
	case w.counter.Goal > 0:
		today = fmt.Sprintf(" · 今天 %s / %s", signed(k), grouped(w.counter.Goal))
		tip += "，今天 " + signed(k) + " 字，目标 " + words(w.counter.Goal)
	case k != 0:
		today = " · 今天 " + signed(k)
		tip += "，今天 " + signed(k) + " 字"
	}
	if !wide {
		today = ""
	}
	color := a.pal.faint // as the design has it, met or not: the numbers say so
	// Whole, as in the prototype: the check line beside it gives way.
	counts := ui.ButtonBase(c).Row().Shrink(0).Label("字数").Tooltip(tip + "。点击设置每日目标")
	if counts.Hovered() {
		color = t.Text
	}
	counts.Children(func() {
		if ok && !roomy && unit == "选中" {
			ui.Text(c, "选中 "+words(here)).FontSize(fsNote).TextColor(color).SingleLine()
		} else if ok && !roomy {
			ui.Text(c, words(here)).FontSize(fsNote).TextColor(color).SingleLine()
		} else if ok {
			ui.Text(c, unit+" "+words(here)+" · 全书 "+words(a.total())+today).FontSize(fsNote).TextColor(color).SingleLine()
		} else {
			ui.Text(c, "全书 "+words(a.total())+today).FontSize(fsNote).TextColor(color).SingleLine()
		}
	})
	if counts.Clicked() {
		w.goalOpen = !w.goalOpen
	}
	// It opens upward: the foot is the last line of the window.
	ui.PopoverBase(c, counts, &w.goalOpen, func(panel ui.Element) {
		panel.AttachTo(counts, ui.AnchorTopRight, ui.AnchorBottomRight).Margin(0, 0, 4, 0).Width(250).Radius(rCard).Padding(8).Gap(2).
			Background(a.pal.raised).Border(1, t.Border)
		a.pal.pop.on(panel)
		a.lastWeek(c)
		ui.Box(c).FillWidth().Height(1).Margin(4, 0).Background(a.pal.line).Shrink(0)
		ui.Column(c).FillWidth().Gap(6).Padding(4, 6).Children(func() {
			ui.Text(c, "每日目标").FontSize(fsNote).TextColor(t.TextMuted)
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
		ui.Text(c, note).FontSize(fsNote).TextColor(ink).LineHeight(lhBody).FillWidth().Padding(0, 6, 4, 6)
	})
}

// lastWeek lists how much was written on each of the last seven days, today
// first.
func (a *App) lastWeek(c *ui.Context) {
	now := time.Now()
	days := a.lastDays(now, 7)
	ui.Text(c, "最近 7 天").FontSize(fsSmall).TextColor(a.pal.faint).Padding(6, 10, 2, 10)
	for i := len(days) - 1; i >= 0; i-- {
		day, k := now.AddDate(0, 0, i-len(days)+1), days[i]
		got := "0 字"
		if k != 0 {
			got = signed(k) + " 字"
		}
		ui.Row(c.Key(i)).FillWidth().Padding(4, 10).Label(fmt.Sprintf("%s 写了 %d 字", dayOf(day, now), k)).Children(func() {
			ui.Text(c, dayOf(day, now)).FontSize(fsNote).TextColor(a.pal.theme.TextMuted).Grow(1)
			ui.Text(c, got).FontSize(fsNote).TextColor(a.pal.theme.TextMuted)
		})
	}
}

// wordsIn is a chapter's count, or for a folder all its chapters' together.
func (w *writeView) wordsIn(n *workspace.Node) int {
	if !n.Folder {
		return w.counts[n.ID]
	}
	k := 0
	for _, ch := range n.Children {
		k += w.wordsIn(ch)
	}
	return k
}
