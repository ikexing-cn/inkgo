package desk

import (
	"github.com/egoist/mygo/ui"
	"writer/desk/kit"
)

type foldLayout struct {
	width, height, sideWidth, paneWidth float32
	side, pane, focus                   bool
	page                                page
}

type foldState struct {
	requested, seen                  bool
	paneOpened                       bool
	last, from                       foldLayout
	progress, offset, left, fromLeft float32
	edge, right, fromRight           float32
}

// foldPaper runs only for the two explicit switches. Resizing, focus and
// navigation put the new layout in place immediately, including mid-fold.
func (a *App) foldPaper(c *ui.Context, body ui.Element) {
	m := &a.fold
	w, h := c.Size()
	next := foldLayout{w, h, a.sidebarWidth(w), a.panelWidth(w), a.sideShown(), a.paneOpen() && !a.focused(), a.focused(), a.page}
	m.paneOpened = m.requested && m.seen && !m.last.pane && next.pane
	left, _ := a.paperColumn(c).Layout(a.pageWidth(c))
	if next.side {
		left += next.sideWidth + 1
	}
	right := w
	if next.pane {
		right -= next.paneWidth + 1
	}
	if m.requested && m.seen && !c.Preferences().ReduceMotion {
		m.from, m.fromLeft = m.last, m.left
		m.fromRight = m.right
		body.Jump("fold", 0)
	} else if !m.seen || next != m.last || c.Preferences().ReduceMotion {
		body.Jump("fold", 1)
	}
	m.progress = body.AnimateWith("fold", 1, kit.MoveTime, ui.Linear)
	m.offset = (m.fromLeft - left) * (1 - kit.Out(m.progress))
	m.edge = (m.fromRight - right) * (1 - kit.Out(m.progress))
	m.left = left + m.offset
	m.right = right + m.edge
	m.last, m.seen, m.requested = next, true, false
	// The rule under the top row runs on across the gap a column coming in
	// has yet to cover.
	if a.foldEntering(true) || a.foldEntering(false) {
		body.DrawOver(func(p *ui.Painter, r ui.Rect) {
			y := r.Y + kit.HBar - 1
			if a.foldEntering(false) {
				p.Fill(ui.Rect{X: r.X + right, Y: y, W: m.edge, H: 1}, a.pal.Line, 0)
			}
			if a.foldEntering(true) {
				width := next.sideWidth + 1
				x := width * kit.Out(m.progress)
				p.Fill(ui.Rect{X: r.X + x, Y: y, W: width - x, H: 1}, a.pal.Line, 0)
			}
		})
	}
}

func (a *App) foldEntering(side bool) bool {
	m := &a.fold
	if side {
		return m.progress < 1 && !m.from.side && m.last.side
	}
	return m.progress < 1 && !m.from.pane && m.last.pane
}

// slide moves e, a part of the paper, with the prose as it folds: the
// editor, and what lines up with the text over and under it. The layout is
// already the new one; only the picture moves, so the lines keep their
// breaks, and the editor keeps its own edges round the text.
func (a *App) slide(e ui.Element) ui.Element {
	return e.Translate(a.fold.offset, 0)
}

// slideEdge keeps e against the paper's moving right edge.
func (a *App) slideEdge(e ui.Element) ui.Element {
	return e.Translate(a.fold.edge, 0)
}

func (a *App) foldColumn(panel ui.Element, side bool, width float32) {
	m := &a.fold
	was, on, direction := m.from.pane, m.last.pane, width+1
	if side {
		was, on, direction = m.from.side, m.last.side, -direction
	}
	if m.progress >= 1 || was == on {
		return
	}
	p := 1 - kit.Out(m.progress)
	if !on {
		p = 1 - p
	}
	panel.Translate(direction*p, 0)
}

// foldCover takes input over a departing column without disabled styling,
// which would dim its text as it leaves.
func foldCover(c *ui.Context) {
	ui.Box(c).Absolute().Left(0).Right(0).Top(0).Bottom(0).KeepFocus().HandleInput(func(ui.InputEvent) bool { return true })
}

func (a *App) foldLeaving(side bool) bool {
	m := &a.fold
	if side {
		return m.progress < 1 && m.from.side && !m.last.side
	}
	return m.progress < 1 && m.from.pane && !m.last.pane
}
