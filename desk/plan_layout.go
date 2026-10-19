package desk

import (
	"math"
	"slices"
	"strconv"

	"github.com/egoist/mygo/ui"
	"writer/workspace"
)

// Geometry contains data and sizes only, never elements from a previous frame.
// Cache it until cards, outlines, search, or the open outline editor change.
type planGeometry struct {
	root           *workspace.Node
	query, editing string
	rows           []planRow
	lanes          map[string]float32
	height         float32
}

type planRow struct {
	node      *workspace.Node
	kind      int // chapter, volume, or "not in a volume"
	y, height float32
	count     int
	when      string
}

func (a *App) planGeometry(c *ui.Context) *planGeometry {
	p := &a.plan
	if g := p.geometry; g != nil && g.root == a.write.tree.Root && g.query == trim(p.query) && g.editing == p.outlineID {
		return g
	}
	g := &planGeometry{root: a.write.tree.Root, query: trim(p.query), editing: p.outlineID, lanes: map[string]float32{}}
	_, unit := c.MeasureText(0, ui.Span{Text: "字", Size: fsBody})
	textHeight := func(text string, width float32, limit int) float32 {
		spans := marked(text, trim(p.query), a.pal.ink)
		for i := range spans {
			spans[i].Size = fsBody
		}
		_, h := c.MeasureText(width, spans...)
		return float32(max(1, min(limit, int(math.Round(float64(h/unit)))))) * fsBody * lhBody
	}
	cardHeight := func(e workspace.Card, timeline bool) float32 {
		h := float32(12+2) + textHeight(e.Title, planCardTextWidth, 2)
		if timeline || e.When != "" {
			h += 2 + fsNote*lhBody
		}
		return h
	}
	// One cell includes padding, its bottom border, and the add button.
	base := float32(20 + 1 + hControl)
	for _, col := range p.index.columns {
		heights := map[string]float32{}
		for _, e := range col.Cards {
			heights[e.Line] += cardHeight(e, true) + 8
		}
		for line, h := range heights {
			g.lanes[line] = max(g.lanes[line], base+h)
		}
	}
	appendRow := func(row planRow) {
		// The layout rounds each row to a pixel. Round before summing too,
		// so one large placeholder has exactly the same extent as its rows.
		row.height = float32(math.Ceil(float64(row.height)))
		row.y = g.height
		g.rows = append(g.rows, row)
		g.height += row.height
	}
	var block func(*workspace.Node)
	block = func(folder *workspace.Node) {
		if folder == a.write.tree.Root {
			for _, n := range folder.Children {
				if n.Folder {
					block(n)
				}
			}
		}
		var chapters, shown []*workspace.Node
		for _, n := range folder.Children {
			if !n.Folder {
				chapters = append(chapters, n)
				if a.planNodeFound(n) || slices.ContainsFunc(a.cardsOf(n.ID), a.cardFound) {
					shown = append(shown, n)
				}
			}
		}
		if folder != a.write.tree.Root && (g.query == "" || len(shown) > 0 || a.planNodeFound(folder)) {
			var cards []workspace.Card
			for _, n := range chapters {
				cards = append(cards, a.cardsOf(n.ID)...)
			}
			h := float32(16+12+1+4+4) + fsGroup*lhTight + textHeight(p.gists[folder.ID], planGistWidth-8, 3)
			if p.outlineID == folder.ID {
				h += hControl * 3
			}
			appendRow(planRow{node: folder, kind: 1, height: h, count: len(chapters), when: storyWhen(cards)})
		}
		if folder == a.write.tree.Root && len(shown) > 0 {
			appendRow(planRow{kind: 2, height: 16 + 12 + fsGroup*lhTight})
		}
		for _, n := range shown {
			cards := a.cardsOf(n.ID)
			when := storyWhen(cards)
			h := float32(32+4+4+1) + fsBody*lhBody + textHeight(p.gists[n.ID], planChapterWidth-24-12-8, 3)
			if when != "" {
				h += 4 + fsNote*lhBody
			}
			if slices.ContainsFunc(cards, func(e workspace.Card) bool { return e.Line == "" }) {
				h += 4 + fsNote*lhBody
			}
			if p.outlineID == n.ID {
				h += hControl * 3
			}
			heights := map[string]float32{}
			for _, e := range cards {
				if e.Line != "" {
					heights[e.Line] += cardHeight(e, false) + 8
				}
			}
			for _, ch := range heights {
				h = max(h, base+ch)
			}
			appendRow(planRow{node: n, height: max(planRowMinHeight, h), when: when})
		}
		for _, n := range folder.Children {
			if n.Folder && folder != a.write.tree.Root {
				block(n)
			}
		}
	}
	block(a.write.tree.Root)
	p.geometry = g
	return g
}

// Move first, then build: ScrollIntoView can now find a previously virtual card.
func (a *App) planReveal(g *planGeometry, width, height float32) {
	p := &a.plan
	if !p.reveal {
		return
	}
	e, ok := p.data.Card(p.seen)
	if !ok {
		return
	}
	if p.mode == 0 {
		chapter := a.cardChapter(e)
		if chapter == "" {
			return
		} // the tray is built separately
		for _, row := range g.rows {
			if row.node != nil && row.node.ID == chapter {
				p.scroll[0].Y = planOffset(p.scroll[0].Y, height, row.y, row.height)
				break
			}
		}
		line := slices.IndexFunc(p.data.Lines, func(l workspace.PlotLine) bool { return l.ID == e.Line })
		if line >= 0 {
			p.scroll[0].X = planOffset(p.scroll[0].X, width-planChapterWidth, float32(line*planCardWidth), planCardWidth)
		}
	} else {
		for i, col := range p.index.columns {
			if slices.ContainsFunc(col.Cards, func(x workspace.Card) bool { return x.ID == e.ID }) {
				p.scroll[1].X = planOffset(p.scroll[1].X, width-planLaneWidth, float32(i*planCardWidth), planCardWidth)
				break
			}
		}
	}
}

func planOffset(offset, viewport, start, size float32) float32 {
	if start < offset {
		return max(0, start)
	}
	if start+size > offset+viewport {
		return max(0, start+size-viewport)
	}
	return offset
}

// Gaps keep the full geometry while each row builds only the shared column set.
func planColumns(c *ui.Context, count, first, end, held int, build func(int)) {
	gap := 0
	for i := 0; i < count; i++ {
		if (i < first || i >= end) && i != held {
			gap++
			continue
		}
		if gap > 0 {
			ui.Box(c.Key("gap-before-" + strconv.Itoa(i))).Width(float32(gap * planCardWidth)).FillHeight().Shrink(0)
			gap = 0
		}
		build(i)
	}
	if gap > 0 {
		ui.Box(c.Key("gap-end")).Width(float32(gap * planCardWidth)).FillHeight().Shrink(0)
	}
}
