package desk

import (
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
	"writer/desk/kit"
	"writer/locales"
)

// openFind shows the find bar, or puts the caret back in it, its words
// selected to be typed over. Words picked within a paragraph are what it
// looks for: a name picked is found where else it is.
func (a *App) openFind() {
	w := &a.write
	if w.cur().ed == nil {
		return
	}
	if s, e := w.cur().ed.Selection(); s != e {
		if q := w.cur().ed.Doc.Slice(s, e); !strings.Contains(q, "\n") {
			w.query, w.current = q, 0
			for i, f := range w.cur().ed.Doc.Find(q) {
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
	if w.cur().ed != nil {
		w.cur().ed.SetFound(nil, 0)
	}
}

func (a *App) findBar(c *ui.Context) {
	t := a.pal.Theme
	w := &a.write
	found := w.cur().ed.Doc.Find(w.query)
	if w.current >= len(found) {
		w.current = 0
	}
	w.cur().ed.SetFound(found, w.current)
	show := func() {
		if len(found) > 0 {
			w.cur().ed.Select(found[w.current][0], found[w.current][1])
		}
	}
	// A band under the tabs, over the page: finding on one line and
	// replacing on the next, so neither is squeezed in a narrow window.
	bar := ui.Column(c).FillWidth().Padding(8, 24).Gap(6).Shrink(0).Background(a.pal.Canvas)
	none := len(found) == 0
	query := func() {
		q := ui.TextInput(c, &w.query).Label(locales.FindBox()).Placeholder(locales.FindBox()).Width(150).Shrink(1).Grow(1).AutoFocus()
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
		ui.TextInput(c, &w.with).Label(locales.FindReplaceWith()).Placeholder(locales.FindReplaceWith()).Width(130).Shrink(1).Grow(1)
	}
	count := func() {
		switch {
		case w.query == "":
		case none:
			ui.Text(c, locales.FindNone()).FontSize(kit.FsNote).TextColor(a.pal.Faint).Shrink(0)
		default:
			ui.Textf(c, "%d / %d", w.current+1, len(found)).FontSize(kit.FsNote).TextColor(t.TextMuted).Shrink(0)
		}
	}
	move := func() {
		if kit.IconButton(c, kit.IcChevronUp, locales.FindPrev(), ui.Color{}).Disabled(none).Clicked() {
			w.current = (w.current + len(found) - 1) % len(found)
			show()
		}
		if kit.IconButton(c, kit.IcChevronDown, locales.FindNext(), ui.Color{}).Disabled(none).Clicked() {
			w.current = (w.current + 1) % len(found)
			show()
		}
	}
	replace := func() {
		if a.pal.Secondary(c, locales.FindReplace()).Disabled(none || w.cur().lost).Clicked() {
			if s, e := w.cur().ed.Selection(); s == found[w.current][0] && e == found[w.current][1] {
				w.cur().ed.Replace(s, e, w.with)
				if found = w.cur().ed.Doc.Find(w.query); len(found) > 0 {
					w.current %= len(found)
					show()
				}
			} else {
				show()
			}
		}
		if w.wide {
			if a.pal.Secondary(c, locales.FindReplaceBook()).Disabled(w.query == "" || len(w.hits) == 0 || w.cur().lost || w.cur().external != nil).Clicked() {
				w.sure = !w.sure
			}
			return
		}
		if a.pal.Secondary(c, locales.FindReplaceAll()).Disabled(none || w.cur().lost).Clicked() {
			n := w.cur().ed.ReplaceAll(w.query, w.with)
			a.status.Did(locales.FindReplaced(n))
		}
	}
	wide := func() {
		if a.pal.Chip(c, locales.FindBook(), w.wide).Tooltip(locales.FindAllTip()).Clicked() {
			w.wide, w.texts, w.sure = !w.wide, nil, false
		}
	}
	closer := func() {
		if kit.IconButton(c, kit.IcX, locales.FindClose(), ui.Color{}).Clicked() {
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
	ui.Box(c).FillWidth().Height(1).Background(a.pal.Line).Shrink(0)
}
