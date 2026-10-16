package desk

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"writer/editor"
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
// read once, all in one pass over the book, and kept in w.texts until the
// search is closed.
func (a *App) bookHits(q string) []bookHit {
	w := &a.write
	if q == "" {
		return nil
	}
	if w.texts == nil {
		texts, err := a.Store.Texts()
		if a.warn(err); texts == nil {
			texts = map[string]string{}
		}
		w.texts = texts
	}
	if w.ed != nil {
		w.texts[w.node] = w.ed.Doc.String()
	}
	var hits []bookHit
	w.tree.Walk(func(n *workspace.Node, _ int) {
		if n.Folder {
			return
		}
		text, ok := w.texts[n.ID]
		if !ok {
			ch, err := a.Store.ReadChapter(n.ID)
			if err != nil {
				return
			}
			text = ch.Text
			w.texts[n.ID] = text
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
	before, after := []rune(text[:at]), []rune(text[at+len(q):])
	pre, post := "", ""
	if len(before) > n {
		before, pre = before[len(before)-n:], "…"
	}
	if len(after) > n {
		after, post = after[:n], "…"
	}
	s := pre + string(before) + q + string(after) + post
	return strings.Join(strings.Fields(s), " ")
}

// bookResults lists the chapters where the query occurs; choosing one
// opens it with its first match selected.
func (a *App) bookResults(c *ui.Context) {
	t := a.pal.theme
	w := &a.write
	if w.query == "" {
		return
	}
	key := [3]any{w.query, w.node, -1}
	if w.ed != nil {
		key[2] = w.ed.Doc.Version()
	}
	if w.texts == nil || key != w.hitsFor {
		w.hits, w.hitsFor = a.bookHits(w.query), key
	}
	hits := w.hits
	total := 0
	for _, h := range hits {
		total += h.Count
	}
	if len(hits) == 0 {
		ui.Text(c, "全书未找到").FontSize(fsNote).TextColor(a.pal.faint)
		return
	}
	ui.Textf(c, "全书 %d 章、共 %d 处", len(hits), total).FontSize(fsNote).TextColor(t.TextMuted)
	ui.Scroll(c.Key("bookhits")).FillWidth().MaxHeight(200).Children(func() {
		ui.Column(c).FillWidth().Gap(2).Children(func() {
			for _, h := range hits {
				row := ui.ButtonBase(c.Key(h.ID)).Column().AlignItems(ui.Start).FillWidth().Gap(2).Padding(6, 8).Radius(rControl).Label("全书结果-" + h.Title)
				if h.ID == w.node {
					row.Background(t.SurfacePressed)
				} else if row.Hovered() {
					row.Background(t.SurfaceHover)
				}
				row.Children(func() {
					ui.Row(c).FillWidth().Gap(8).Children(func() {
						ui.Text(c, h.Title).FontSize(fsControl).FontWeight(600).SingleLine().Ellipsis("…").Shrink(1)
						ui.Textf(c, "%d 处", h.Count).FontSize(fsNote).TextColor(a.pal.faint).Shrink(0)
					})
					ui.Text(c, h.Around).FontSize(fsNote).TextColor(t.TextMuted).SingleLine().Ellipsis("…")
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
func (a *App) openHit(id string) {
	w := &a.write
	if id != w.node {
		a.openNode(id)
	}
	if w.ed == nil {
		return
	}
	w.current = 0
	if found := w.ed.Doc.Find(w.query); len(found) > 0 {
		w.ed.Select(found[0][0], found[0][1])
	}
}

// confirmBookReplace asks before a replace that reaches chapters the undo
// cannot.
func (a *App) confirmBookReplace(c *ui.Context) {
	t := a.pal.theme
	w := &a.write
	total := 0
	for _, h := range w.hits {
		total += h.Count
	}
	with := "“" + w.with + "”"
	if w.with == "" {
		with = "空（删除）"
	}
	ui.Row(c).FillWidth().Gap(8).Padding(8, 10).Radius(rCard).AlignItems(ui.Center).Background(t.Danger.Alpha(0.08)).Children(func() {
		ui.Textf(c, "将全书 %d 章中的 %d 处“%s”替换为%s？仅修改正文，设定和章纲不变；各章替换前均保存快照。", len(w.hits), total, w.query, with).
			FontSize(fsNote).LineHeight(lhBody).Grow(1).Shrink(1)
		if a.pal.secondary(c, "取消").Clicked() {
			w.sure = false
		}
		if a.pal.danger(c, "确定替换").Clicked() {
			a.replaceInBook()
		}
	})
}

// replaceInBook replaces the query in every chapter. Each chapter is
// snapshotted first; the chapter open is replaced in its editor, where it
// can be undone, the others on the disk with their marks moved along.
func (a *App) replaceInBook() {
	w := &a.write
	w.sure = false
	if err := a.saveText(); err != nil {
		a.warn(err)
		return
	}
	w.texts = nil
	hits := a.bookHits(w.query)
	chapters, total, here := 0, 0, false
	for _, h := range hits {
		var n int
		if h.ID == w.node && w.ed != nil {
			if _, err := a.Store.TakeSnapshot(h.ID, workspace.SnapReplace, w.ed.Doc.String(), storedMarks(w.ed.Doc.Marks())); err != nil {
				a.warn(err)
				break
			}
			n, here = w.ed.ReplaceAll(w.query, w.with), true
			a.warn(a.saveText())
		} else {
			ch, err := a.Store.ReadChapter(h.ID)
			if err == nil {
				_, err = a.Store.TakeSnapshot(h.ID, workspace.SnapReplace, ch.Text, ch.Marks)
			}
			if err != nil {
				a.warn(err)
				break
			}
			doc := editor.NewDoc(ch.Text)
			doc.LoadMarks(editorMarks(ch.Marks))
			n = doc.ReplaceAll(w.query, w.with)
			// Over the text read just now, and no other.
			if _, err := a.Store.WriteChapterOver(h.ID, doc.String(), storedMarks(doc.Marks()), ch.Stamp); err != nil {
				a.warn(err)
				break
			}
			if w.counts != nil {
				w.counts[h.ID] = workspace.Count(doc.String())
			}
		}
		chapters, total = chapters+1, total+n
	}
	w.texts, w.hits = nil, nil
	w.hitsFor = [3]any{}
	a.notice = fmt.Sprintf("已在 %d 章中替换 %d 处，各章替换前的版本已存入快照", chapters, total)
	if here {
		a.notice += "；本章也可直接撤销"
	}
}
