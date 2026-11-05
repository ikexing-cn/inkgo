package find

import (
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
	Query := func() {
		q := ui.TextInput(c, &w.Query).Label(findtext.FindBox()).Placeholder(findtext.FindBox()).Width(150).Shrink(1).Grow(1).AutoFocus()
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
		ui.TextInput(c, &w.With).Label(findtext.FindReplaceWith()).Placeholder(findtext.FindReplaceWith()).Width(130).Shrink(1).Grow(1)
	}
	count := func() {
		switch {
		case w.Query == "":
		case none:
			ui.Text(c, locales.FindNone()).FontSize(kit.FsNote).TextColor(a.Palette.Colors().Faint).Shrink(0)
		default:
			ui.Textf(c, "%d / %d", w.Current+1, len(found)).FontSize(kit.FsNote).TextColor(t.TextMuted).Shrink(0)
		}
	}
	move := func() {
		if kit.IconButton(c, kit.IcChevronUp, findtext.FindPrev(), ui.Color{}).Disabled(none).Clicked() {
			w.Current = (w.Current + len(found) - 1) % len(found)
			show()
		}
		if kit.IconButton(c, kit.IcChevronDown, findtext.FindNext(), ui.Color{}).Disabled(none).Clicked() {
			w.Current = (w.Current + 1) % len(found)
			show()
		}
	}
	replace := func() {
		if a.Palette.Colors().Secondary(c, findtext.FindReplace()).Disabled(none || a.lost()).Clicked() {
			if s, e := a.editor().Selection(); s == found[w.Current][0] && e == found[w.Current][1] {
				a.editor().Replace(s, e, w.With)
				if found = a.editor().Doc.Find(w.Query); len(found) > 0 {
					w.Current %= len(found)
					show()
				}
			} else {
				show()
			}
		}
		if w.Wide {
			if a.Palette.Colors().Secondary(c, findtext.FindReplaceBook()).KeepFocus().Disabled(w.Query == "" || len(w.Hits) == 0 || a.lost() || a.paused()).Clicked() {
				w.Sure = true
			}
			return
		}
		if a.Palette.Colors().Secondary(c, findtext.FindReplaceAll()).Disabled(none || a.lost()).Clicked() {
			n := a.editor().ReplaceAll(w.Query, w.With)
			a.Notices.Notices().Did(findtext.FindReplaced(n))
		}
	}
	Wide := func() {
		if a.Palette.Colors().Chip(c, findtext.FindBook(), w.Wide).Tooltip(findtext.FindAllTip()).Clicked() {
			w.Wide, w.Texts, w.Sure = !w.Wide, nil, false
		}
	}
	closer := func() {
		if kit.IconButton(c, kit.IcX, findtext.FindClose(), ui.Color{}).Clicked() {
			a.closeFind()
		}
	}
	line := func(fn func()) { ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(fn) }
	bar.Children(func() {
		line(func() { Query(); count(); move(); Wide(); closer() })
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
