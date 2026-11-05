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

// bookHit is one chapter where the query occurs, for finding in the whole
// book.
type bookHit struct {
	ID, Title string
	Count     int
	Matches   []bookMatch
}

// bookMatch keeps a rune range and a snippet for one occurrence.
type bookMatch struct {
	Start, End int
	Snippet    string
}

// bookRow is either a chapter heading (Match < 0) or one occurrence.
type bookRow struct{ Chapter, Match int }

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
			h := bookHit{ID: n.ID, Title: n.Title, Count: k}
			offset, runes := 0, 0
			chars := []rune(text)
			queryLen := utf8.RuneCountInString(q)
			for {
				at := strings.Index(text[offset:], q)
				if at < 0 {
					break
				}
				at += offset
				runes += utf8.RuneCountInString(text[offset:at])
				left := runes
				for left > 0 && runes-left < 6 && chars[left-1] != '\n' {
					left--
				}
				prefix := ""
				if left > 0 && chars[left-1] != '\n' {
					prefix = "…"
				}
				right := runes + queryLen
				for right < len(chars) && right-runes-queryLen < 14 && chars[right] != '\n' {
					right++
				}
				h.Matches = append(h.Matches, bookMatch{runes, runes + queryLen, prefix + string(chars[left:right])})
				offset = at + len(q)
				runes += utf8.RuneCountInString(q)
			}
			hits = append(hits, h)
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

// refreshBookHits rebuilds results only when the query or edited chapter changes.
func (a *View) refreshBookHits() {
	key := [3]any{a.Query, a.node(), -1}
	if a.editor() != nil {
		key[2] = a.editor().Doc.Version()
	}
	if a.Texts != nil && key == a.HitsFor {
		return
	}
	a.Hits, a.HitsFor = a.bookHits(a.Query), key
	a.hitRows, a.hitTotal = nil, 0
	a.hitList = ui.ListState{}
	for i, h := range a.Hits {
		a.hitRows = append(a.hitRows, bookRow{i, -1})
		a.hitTotal += h.Count
		for j := range h.Matches {
			a.hitRows = append(a.hitRows, bookRow{i, j})
		}
	}
}

// bookResults lists each occurrence below its chapter heading.
func (a *View) bookResults(c *ui.Context) {
	if len(a.hitRows) == 0 {
		if a.Query != "" {
			ui.Text(c, findtext.FindNoneBook()).FontSize(kit.FsNote).TextColor(a.Palette.Colors().Faint)
		}
		return
	}
	pal := a.Palette.Colors()
	ui.Text(c, findtext.FindBookCount(len(a.Hits), a.hitTotal)).FontSize(kit.FsNote).TextColor(pal.Theme.TextMuted)
	ui.List(c.Key("bookhits"), &a.hitList, len(a.hitRows), func(i int) {
		r := a.hitRows[i]
		h := a.Hits[r.Chapter]
		if r.Match < 0 {
			ui.Text(c, locales.FindChapterHead(h.Title, h.Count)).FontSize(kit.FsSmall).TextColor(pal.Faint).Padding(14, 10, 4, 10)
			return
		}
		m := h.Matches[r.Match]
		row := ui.ButtonBase(c.Key(i)).Row().FillWidth().Height(kit.HControl).AlignItems(ui.Center).Justify(ui.Start).Padding(0, 10).Radius(kit.RControl).Label(fmt.Sprintf("全书结果-%s-%d", h.Title, r.Match+1))
		if row.Hovered() {
			row.Background(pal.Theme.SurfaceHover)
		}
		kit.PressRow(c, row)
		row.Children(func() {
			ui.RichText(c, kit.Marked(m.Snippet, a.Query, pal.Ink)...).FontSize(kit.FsNote).TextColor(pal.Theme.TextMuted).SingleLine().Ellipsis("…").Shrink(1)
		})
		if row.Clicked() {
			a.openHit(h.ID, r.Match)
		}
	}).FillWidth().Height(min(float32(len(a.hitRows)*kit.HControl), 190))
}

// openHit opens the chosen occurrence and closes the book-wide list.
func (a *View) openHit(id string, index int) {
	if id != a.node() {
		a.Desktop.OpenWriting(id)
	}
	if a.editor() == nil {
		return
	}
	a.Wide = false
	a.Current = index
	if found := a.editor().Doc.Find(a.Query); index < len(found) {
		a.editor().Select(found[index][0], found[index][1])
	}
}

// confirmBookReplace uses the host modal and preserves the find fields beneath it.
func (a *View) confirmBookReplace(c *ui.Context) {
	pal := a.Palette.Colors()
	pal.Modal(c, &a.Sure, func() {
		ui.Column(c).Width(kit.DialogAsk).Padding(kit.DialogPad).Gap(20).Children(func() {
			with := findtext.DialogSwapTo(a.With)
			if a.With == "" {
				with = findtext.DialogSwapNone()
			}
			pal.DialogHead(c, locales.DialogSwapTitle(), findtext.DialogSwapAbout(len(a.Hits), a.hitTotal, a.Query, with))
			ui.Row(c).FillWidth().Gap(8).Children(func() {
				ui.Spacer(c)
				if pal.Secondary(c, locales.CommonCancel()).Clicked() {
					a.Sure = false
				}
				if pal.Primary(c, findtext.DialogSwapConfirm()).Clicked() {
					a.replaceInBook()
				}
			})
		})
	})
}

// replaceInBook replaces the query in every chapter. Each chapter is
// snapshotted first; the chapter open is replaced in its editor, where it
// can be undone, the others on the disk with their marks moved along.
func (a *View) replaceInBook() {
	w := a
	w.Sure = false
	if err := a.Editing.SaveText(); err != nil {
		a.Notices.Notices().Warn(err)
		return
	}
	w.Texts = nil
	chapters, total, here, err := a.Editing.ReplaceBookText(w.Query, w.With)
	a.Notices.Notices().Warn(err)
	w.Texts, w.Hits = nil, nil
	w.HitsFor = [3]any{}
	notice := findtext.FindReplacedBook(chapters, total)
	if here {
		notice = findtext.FindReplacedBookUndo(chapters, total)
	}
	a.Notices.Notices().Did(notice)
}
