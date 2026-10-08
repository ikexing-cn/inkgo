package desk

import (
	"strings"

	"github.com/egoist/mygo/ui"

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
// read once and kept in w.texts until the search is closed.
func (a *App) bookHits(q string) []bookHit {
	w := &a.write
	if q == "" {
		return nil
	}
	if w.texts == nil {
		w.texts = map[string]string{}
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
		ui.Text(c, "全书都没有找到").FontSize(12.5).TextColor(a.pal.faint)
		return
	}
	ui.Textf(c, "全书 %d 章、共 %d 处", len(hits), total).FontSize(12.5).TextColor(t.TextMuted)
	ui.Scroll(c).Key("bookhits").FillWidth().MaxHeight(200).Children(func() {
		ui.Column(c).FillWidth().Gap(2).Children(func() {
			for _, h := range hits {
				row := ui.ButtonBase(c).Key(h.ID).Column().AlignItems(ui.Start).FillWidth().Gap(2).Padding(5, 8).Radius(7).Label("全书结果-" + h.Title)
				if h.ID == w.node {
					row.Background(t.SurfacePressed)
				} else if row.Hovered() {
					row.Background(t.SurfaceHover)
				}
				row.Children(func() {
					ui.Row(c).FillWidth().Gap(8).Children(func() {
						ui.Text(c, h.Title).FontSize(13).FontWeight(600).SingleLine().Ellipsis("…").Shrink(1)
						ui.Textf(c, "%d 处", h.Count).FontSize(12).TextColor(a.pal.faint).Shrink(0)
					})
					ui.Text(c, h.Around).FontSize(12.5).TextColor(t.TextMuted).SingleLine().Ellipsis("…")
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
