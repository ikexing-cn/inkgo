package desk

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
)

// openFind shows the find bar, or puts the caret back in it, its words
// selected to be typed over. Words picked within a paragraph are what it
// looks for: a name picked is found where else it is.
func (a *App) openFind() {
	w := &a.write
	if w.ed == nil {
		return
	}
	if s, e := w.ed.Selection(); s != e {
		if q := w.ed.Doc.Slice(s, e); !strings.Contains(q, "\n") {
			w.query, w.current = q, 0
			for i, f := range w.ed.Doc.Find(q) {
				if f[0] == s {
					w.current = i
				}
			}
		}
	}
	w.finding, w.findAgain = true, true
}

// closeFind puts the find bar away, and what it found is no longer marked
// in the prose.
func (a *App) closeFind() {
	w := &a.write
	w.finding, w.wide, w.texts, w.sure = false, false, nil, false
	if w.ed != nil {
		w.ed.SetFound(nil, 0)
	}
}

func (a *App) findBar(c *ui.Context) {
	t := a.pal.theme
	w := &a.write
	found := w.ed.Doc.Find(w.query)
	if w.current >= len(found) {
		w.current = 0
	}
	w.ed.SetFound(found, w.current)
	show := func() {
		if len(found) > 0 {
			w.ed.Select(found[w.current][0], found[w.current][1])
		}
	}
	// A band under the tabs, over the page: finding on one line and
	// replacing on the next, so neither is squeezed in a narrow window.
	bar := ui.Column(c).FillWidth().Padding(8, 24).Gap(6).Shrink(0).Background(a.pal.side)
	none := len(found) == 0
	query := func() {
		q := ui.TextInput(c, &w.query).Label("查找").Placeholder("查找").Width(150).Shrink(1).Grow(1).AutoFocus()
		if w.findAgain {
			q.Focus().SetTextSelection(0, utf8.RuneCountInString(w.query))
			w.findAgain = false
		}
		if q.Submitted() && !none {
			w.current = (w.current + 1) % len(found)
			show()
		}
	}
	with := func() {
		ui.TextInput(c, &w.with).Label("替换为").Placeholder("替换为").Width(130).Shrink(1).Grow(1)
	}
	count := func() {
		switch {
		case w.query == "":
		case none:
			ui.Text(c, "未找到").FontSize(fsNote).TextColor(a.pal.faint).Shrink(0)
		default:
			ui.Textf(c, "%d / %d", w.current+1, len(found)).FontSize(fsNote).TextColor(t.TextMuted).Shrink(0)
		}
	}
	move := func() {
		if iconButton(c, icChevronUp, "上一个", ui.Color{}).Disabled(none).Clicked() {
			w.current = (w.current + len(found) - 1) % len(found)
			show()
		}
		if iconButton(c, icChevronDown, "下一个", ui.Color{}).Disabled(none).Clicked() {
			w.current = (w.current + 1) % len(found)
			show()
		}
	}
	replace := func() {
		if a.pal.secondary(c, "替换").Disabled(none || w.lost).Clicked() {
			if s, e := w.ed.Selection(); s == found[w.current][0] && e == found[w.current][1] {
				w.ed.Replace(s, e, w.with)
				if found = w.ed.Doc.Find(w.query); len(found) > 0 {
					w.current %= len(found)
					show()
				}
			} else {
				show()
			}
		}
		if w.wide {
			if a.pal.secondary(c, "全书替换").Disabled(w.query == "" || len(w.hits) == 0 || w.lost || w.external != nil).Clicked() {
				w.sure = !w.sure
			}
			return
		}
		if a.pal.secondary(c, "全部替换").Disabled(none || w.lost).Clicked() {
			n := w.ed.ReplaceAll(w.query, w.with)
			a.notice = fmt.Sprintf("已替换 %d 处，可撤销", n)
		}
	}
	wide := func() {
		if a.pal.chip(c, "全书", w.wide).Tooltip("在全部章节中查找").Clicked() {
			w.wide, w.texts, w.sure = !w.wide, nil, false
		}
	}
	closer := func() {
		if iconButton(c, icX, "关闭查找", ui.Color{}).Clicked() {
			a.closeFind()
		}
	}
	line := func(fn func()) { ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(fn) }
	bar.Children(func() {
		line(func() { query(); count(); move(); wide(); closer() })
		line(func() { with(); replace() })
		if w.wide && w.sure {
			a.confirmBookReplace(c)
		}
		if w.wide {
			a.bookResults(c)
		}
	})
	ui.Box(c).FillWidth().Height(1).Background(a.pal.line).Shrink(0)
}
