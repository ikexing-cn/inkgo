package find

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
	"writer/desk/kit"
	"writer/ext/find/findtext"
	"writer/locales"
	"writer/workspace"
)

// openFind shows the find bar, or puts the caret back in it, its words
// selected to be typed over. Words picked within a paragraph are what it
// looks for: a name picked is found where else it is.
func (a *View) openFind() {
	w := a
	if a.editor() == nil {
		return
	}
	if s, e := a.editor().Selection(); s != e {
		if q := a.editor().Doc.Slice(s, e); !strings.Contains(q, "\n") {
			w.Query, w.Current = q, 0
			for i, f := range a.editor().Doc.Find(q) {
				if f[0] == s {
					w.Current = i
				}
			}
		}
	}
	w.Finding, w.FindAgain = true, true
}

// closeFind puts the find bar away, and what it found is no longer marked
// in the prose.
func (a *View) closeFind() {
	w := a
	w.Finding, w.Wide, w.Texts, w.Sure = false, false, nil, false
	if a.editor() != nil {
		a.editor().SetFound(nil, 0)
	}
}

func (a *View) findBar(c *ui.Context) {
	t := a.Palette.Colors().Theme
	w := a
	if w.Wide {
		a.refreshBookHits()
	}
	found := a.editor().Doc.Find(w.Query)
	if w.Current >= len(found) {
		w.Current = 0
	}
	a.editor().SetFound(found, w.Current)
	show := func() {
		if len(found) > 0 {
			a.editor().Select(found[w.Current][0], found[w.Current][1])
		}
	}
	// A band under the tabs, over the page: finding on one line and
	// replacing on the next, so neither is squeezed in a narrow window.
	bar := ui.Column(c).FillWidth().Padding(8, 24).Gap(6).Shrink(0).Background(a.Palette.Colors().Canvas)
	none := len(found) == 0
	replaceOne := func() {
		if none {
			return
		}
		a.editor().Replace(found[w.Current][0], found[w.Current][1], w.With)
		if next := a.editor().Doc.Find(w.Query); len(next) > 0 {
			if w.Current >= len(next) {
				w.Current = 0
			}
			a.editor().Select(next[w.Current][0], next[w.Current][1])
		}
	}
	input := func(value *string, label string, search bool) ui.Element {
		box := ui.Row(c).Grow(1).Shrink(1).MinWidth(0).Height(kit.HControl).Gap(8).Padding(0, 10).AlignItems(ui.Center).Radius(kit.RControl).Background(a.Palette.Colors().Canvas).Border(1, t.Border)
		if box.FocusWithin() {
			box.Border(1, a.Palette.Colors().Ink)
		}
		var field ui.Element
		box.Children(func() {
			if search {
				kit.Glyph(c, kit.IcSearch, kit.IconSm, a.Palette.Colors().Faint).Label("find-query-icon")
			}
			field = ui.TextInputBase(c, value).Label(label).Placeholder(label).Grow(1).Shrink(1).MinWidth(0).FontSize(kit.FsBody).PlaceholderColor(a.Palette.Colors().Faint)
		})
		return field
	}
	Query := func() {
		q := input(&w.Query, findtext.FindBox(), true).AutoFocus()
		if q.Changed() {
			w.Current = 0
		}
		if w.FindAgain {
			q.Focus().SetTextSelection(0, utf8.RuneCountInString(w.Query))
			w.FindAgain = false
		}
		if q.Submitted() && !none {
			w.Current = (w.Current + 1) % len(found)
			show()
		}
	}
	with := func() {
		if input(&w.With, findtext.FindReplaceWith(), false).Submitted() && !a.lost() && !a.paused() {
			replaceOne()
		}
	}
	count := func() {
		text := ""
		if w.Query != "" {
			switch {
			case w.Wide && w.hitTotal > 0:
				text = findtext.FindBookCount(len(w.Hits), w.hitTotal)
			case w.Wide:
				text = findtext.FindNoneBook()
			case none:
				text = locales.FindNone()
			default:
				text = fmt.Sprintf("%d / %d", w.Current+1, len(found))
			}
		}
		ui.Row(c).MinWidth(64).Shrink(0).Justify(ui.End).Label("find-count").Children(func() {
			ui.Text(c, text).FontSize(kit.FsNote).TextColor(t.TextMuted).SingleLine()
		})
	}
	move := func() {
		if kit.IconButton(c, kit.IcChevronUp, findtext.FindPrev(), ui.Color{}).Clicked() && !none {
			w.Current = (w.Current + len(found) - 1) % len(found)
			show()
		}
		if kit.IconButton(c, kit.IcChevronDown, findtext.FindNext(), ui.Color{}).Clicked() && !none {
			w.Current = (w.Current + 1) % len(found)
			show()
		}
	}
	replace := func() {
		if w.Wide {
			if a.Palette.Colors().Secondary(c, findtext.FindReplaceBook()).KeepFocus().Disabled(w.hitTotal == 0 || a.lost() || a.paused()).Clicked() {
				w.Sure = true
			}
			return
		}
		if a.Palette.Colors().Secondary(c, findtext.FindReplace()).Disabled(none || a.lost()).Clicked() {
			replaceOne()
		}
		if a.Palette.Colors().Secondary(c, findtext.FindReplaceAll()).Disabled(none || a.lost()).Clicked() {
			n := a.editor().ReplaceAll(w.Query, w.With)
			a.Notices.Notices().Did(findtext.FindReplaced(n))
		}
	}
	Wide := func() {
		if a.Palette.Colors().Chip(c, findtext.FindBook(), w.Wide).Tooltip(findtext.FindAllTip()).Clicked() {
			w.Wide, w.Texts, w.Sure, w.Current = !w.Wide, nil, false, 0
		}
	}
	closer := func() {
		if kit.IconButton(c, kit.IcX, findtext.FindClose(), ui.Color{}).Clicked() {
			a.closeFind()
		}
	}
	line := func(fn func()) { ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(fn) }
	bar.Children(func() {
		line(func() {
			Query()
			count()
			if !w.Wide {
				move()
			}
			Wide()
			closer()
		})
		line(func() { with(); replace() })
		if w.Wide {
			a.bookResults(c)
		}
	})
	ui.Box(c).FillWidth().Height(1).Background(a.Palette.Colors().Line).Shrink(0)
}

// Review opens whole-book replacement for review without changing prose.
// Call on the frame thread while the find extension is active.
func (a *View) Review(old, new string) {
	id, _, _ := a.Desktop.Selected()
	if a.editor() == nil {
		if _, ok := a.Book.Content().Tree().Node(id); !ok {
			id = ""
			a.Book.Content().Tree().Walk(func(n *workspace.Node, _ int) {
				if id == "" && !n.Folder {
					id = n.ID
				}
			})
		}
	}
	if id == "" {
		return
	}
	a.Desktop.OpenWriting(id)
	if a.editor() == nil {
		return
	}
	a.openFind()
	a.Query, a.With, a.Wide, a.Sure = old, new, true, false
	a.Texts, a.HitsFor = nil, [3]any{}
}
