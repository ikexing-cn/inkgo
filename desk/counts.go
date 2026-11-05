package desk

import (
	"github.com/egoist/mygo/ui"
	"writer/desk/book"
	"writer/desk/kit"
	"writer/ext"
	"writer/ext/hostapi"
	"writer/locales"
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

// wordCount draws selection/chapter and book totals with contributed suffixes
// and popovers at the right of the footer.
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
	suffix, tip := "", book
	for _, item := range ext.Items(a.extensions, hostapi.WordSuffix) {
		_ = ext.Guard(a.extensions, item.Owner, item.ID, func() {
			if item.Value.Text != nil {
				suffix += item.Value.Text()
			}
			if item.Value.Tooltip != nil {
				tip = item.Value.Tooltip(book)
			}
		})
	}
	popovers := ext.Items(a.extensions, hostapi.WordPopovers)
	color := a.pal.Faint // as the design has it, met or not: the numbers say so
	var counts ui.Element
	if len(popovers) > 0 {
		counts = ui.ButtonBase(c).Tooltip(tip)
	} else {
		counts = ui.Box(c)
	}
	line := locales.StatusBookCountLine(book, suffix)
	if ok {
		line = locales.StatusCountLine(scope, book, suffix)
	}
	width, _ := c.MeasureText(0, ui.Span{Text: line, Size: kit.FsNote})
	// Allocate counts from the space left by the fixed footer items. A large
	// shrink factor still takes fractions from the status and triggers ellipsis.
	counts.Row().Basis(0).Grow(1000).MaxWidth(width).Shrink(1000).MinWidth(0).Label(locales.StatusWordCount(a.countUnit()))
	a.slideEdge(counts)
	if len(popovers) > 0 && counts.Hovered() {
		color = t.Text
	}
	counts.Children(func() {
		ui.Text(c, line).FontSize(kit.FsNote).TextColor(color).SingleLine().Ellipsis("…").Shrink(1).MinWidth(0)
	})
	for _, item := range popovers {
		_, _ = ext.Call(a.extensions, ext.HookOf(ext.CallPoint[hostapi.WordPopover, func(hostapi.WordPopover) struct{}, struct{}]{Point: hostapi.WordPopovers}, item.ID), func(p hostapi.WordPopover) struct{} { p.Draw(c, counts, counts.Clicked()); return struct{}{} }, wordPopoverCall(item.Value))
	}
}

func wordPopoverCall(p hostapi.WordPopover) func(func(hostapi.WordPopover) struct{}) struct{} {
	return func(draw func(hostapi.WordPopover) struct{}) struct{} { return draw(p) }
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
