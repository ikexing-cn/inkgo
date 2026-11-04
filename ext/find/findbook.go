package find

import (
	"strings"

	"github.com/egoist/mygo/ui"
	"writer/desk/kit"
	"writer/ext/find/findtext"
	"writer/locales"
	"writer/workspace"
)

// bookHit is one chapter where the query occurs, for finding in the whole
// book.
type bookHit struct {
	ID, Title string
	Count     int
	Around    string // the first match with a little on each side
}

// bookHits finds q in every chapter in the tree's order. The chapter open
// is read from the editor, which may be ahead of the disk; the others are
// read once, all in one pass over the book, and kept in w.Texts until the
// search is closed.
func (a *View) bookHits(q string) []bookHit {
	w := a
	if q == "" {
		return nil
	}
	if w.Texts == nil {
		texts, err := a.Book.Store().Texts()
		if a.Notices.Notices().Warn(err); texts == nil {
			texts = map[string]string{}
		}
		w.Texts = texts
	}
	if a.editor() != nil {
		w.Texts[a.node()] = a.editor().Doc.String()
	}
	var hits []bookHit
	a.Book.Content().Tree().Walk(func(n *workspace.Node, _ int) {
		if n.Folder {
			return
		}
		text, ok := w.Texts[n.ID]
		if !ok {
			ch, err := a.Book.Store().ReadChapter(n.ID)
			if err != nil {
				return
			}
			text = ch.Text
			w.Texts[n.ID] = text
		}
		if k := strings.Count(text, q); k > 0 {
			hits = append(hits, bookHit{ID: n.ID, Title: n.Title, Count: k, Around: around(text, q, 14)})
		}
	})
	return hits
}

// around is the first q in text with up to n runes on each side, on one
// line.
func around(text, q string, n int) string {
	at := strings.Index(text, q)
	if at < 0 {
		return ""
	}
	before, after := text[:at], text[at+len(q):]
	s := tail(before, n) + q + clip(after, n)
	return strings.Join(strings.Fields(s), " ")
}

// bookResults lists the chapters where the query occurs; choosing one
// opens it with its first match selected.
func (a *View) bookResults(c *ui.Context) {
	t := a.Palette.Colors().Theme
	w := a
	if w.Query == "" {
		return
	}
	key := [3]any{w.Query, a.node(), -1}
	if a.editor() != nil {
		key[2] = a.editor().Doc.Version()
	}
	if w.Texts == nil || key != w.HitsFor {
		w.Hits, w.HitsFor = a.bookHits(w.Query), key
	}
	hits := w.Hits
	total := 0
	for _, h := range hits {
		total += h.Count
	}
	if len(hits) == 0 {
		ui.Text(c, findtext.FindNoneBook()).FontSize(kit.FsNote).TextColor(a.Palette.Colors().Faint)
		return
	}
	ui.Text(c, findtext.FindBookCount(len(hits), total)).FontSize(kit.FsNote).TextColor(t.TextMuted)
	ui.Scroll(c.Key("bookhits")).FillWidth().MaxHeight(200).Children(func() {
		ui.Column(c).FillWidth().Gap(2).Children(func() {
			for _, h := range hits {
				row := ui.ButtonBase(c.Key(h.ID)).Column().AlignItems(ui.Start).FillWidth().Gap(2).Padding(6, 8).Radius(kit.RControl).Label("全书结果-" + h.Title)
				if h.ID == a.node() {
					row.Background(t.SurfacePressed)
				} else if row.Hovered() {
					row.Background(t.SurfaceHover)
				}
				kit.PressRow(c, row)
				row.Children(func() {
					ui.Row(c).FillWidth().Gap(8).Children(func() {
						ui.Text(c, h.Title).FontSize(kit.FsControl).FontWeight(600).SingleLine().Ellipsis("…").Shrink(1)
						ui.Text(c, findtext.FindMatchCount(h.Count)).FontSize(kit.FsNote).TextColor(a.Palette.Colors().Faint).Shrink(0)
					})
					ui.Text(c, h.Around).FontSize(kit.FsNote).TextColor(t.TextMuted).SingleLine().Ellipsis("…")
				})
				if row.Clicked() {
					a.openHit(h.ID)
				}
			}
		})
	})
}

// openHit opens a chapter from the book's results with the first match
// selected, keeping the search open.
func (a *View) openHit(id string) {
	w := a
	if id != a.node() {
		a.Desktop.OpenWriting(id)
	}
	if a.editor() == nil {
		return
	}
	w.Current = 0
	if found := a.editor().Doc.Find(w.Query); len(found) > 0 {
		a.editor().Select(found[0][0], found[0][1])
	}
}

// confirmBookReplace asks before a replace that reaches chapters the undo
// cannot.
func (a *View) confirmBookReplace(c *ui.Context) {
	t := a.Palette.Colors().Theme
	w := a
	total := 0
	for _, h := range w.Hits {
		total += h.Count
	}
	with := findtext.DialogSwapTo(w.With)
	if w.With == "" {
		with = findtext.DialogSwapNone()
	}
	ui.Row(c).FillWidth().Gap(8).Padding(8, 10).Radius(kit.RCard).AlignItems(ui.Center).Background(t.Danger.Alpha(0.08)).Children(func() {
		ui.Text(c, findtext.DialogSwapAbout(len(w.Hits), total, w.Query, with)).
			FontSize(kit.FsNote).LineHeight(kit.LhBody).Grow(1).Shrink(1)
		if a.Palette.Colors().Secondary(c, locales.CommonCancel()).Clicked() {
			w.Sure = false
		}
		if a.Palette.Colors().Danger(c, findtext.DialogSwapConfirm()).Clicked() {
			a.replaceInBook()
		}
	})
}

// replaceInBook replaces the query in every chapter. Each chapter is
// snapshotted first; the chapter open is replaced in its editor, where it
// can be undone, the others on the disk with their marks moved along.
func (a *View) replaceInBook() {
	w := a
	w.Sure = false
	if err := a.Book.SaveText(); err != nil {
		a.Notices.Notices().Warn(err)
		return
	}
	w.Texts = nil
	chapters, total, here, err := a.Book.ReplaceBookText(w.Query, w.With)
	a.Notices.Notices().Warn(err)
	w.Texts, w.Hits = nil, nil
	w.HitsFor = [3]any{}
	notice := findtext.FindReplacedBook(chapters, total)
	if here {
		notice = findtext.FindReplacedBookUndo(chapters, total)
	}
	a.Notices.Notices().Did(notice)
}
