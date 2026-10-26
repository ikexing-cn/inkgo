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
	last, from                       foldLayout
	progress, offset, left, fromLeft float32
}

// foldPaper runs only for the two explicit switches. Resizing, focus and
// navigation put the new layout in place immediately, including mid-fold.
func (a *App) foldPaper(c *ui.Context, body ui.Element) {
	m := &a.fold
	w, h := c.Size()
	next := foldLayout{w, h, a.sidebarWidth(w), a.panelWidth(w), a.sideShown(), a.paneOpen() && !a.focused(), a.focused(), a.page}
	left, _ := a.paperColumn(c).Layout(a.pageWidth(c))
	if next.side {
		left += next.sideWidth + 1
	}
	if m.requested && m.seen && !c.Preferences().ReduceMotion {
		m.from, m.fromLeft = m.last, m.left
		body.Jump("fold", 0)
	} else if !m.seen || next != m.last || c.Preferences().ReduceMotion {
		body.Jump("fold", 1)
	}
	m.progress = body.AnimateWith("fold", 1, kit.MoveTime, ui.Linear)
	m.offset = (m.fromLeft - left) * (1 - kit.Out(m.progress))
	m.left = left + m.offset
	m.last, m.seen, m.requested = next, true, false
}

// slide moves e, a part of the paper, with the prose as it folds: the
// editor, and what lines up with the text over and under it. The layout is
// already the new one; only the picture moves, so the lines keep their
// breaks, and the editor keeps its own edges round the text.
func (a *App) slide(e ui.Element) ui.Element {
	return e.Translate(a.fold.offset, 0)
}

func (a *App) foldColumn(panel ui.Element, side bool) {
	m := &a.fold
	was, on, direction := m.from.pane, m.last.pane, kit.ColumnTravel
	if side {
		was, on, direction = m.from.side, m.last.side, -direction
	}
	if m.progress >= 1 || was == on {
		return
	}
	p := kit.Out(m.progress)
	if !on {
		p = 1 - kit.In(m.progress)
	}
	panel.Translate(direction*(1-p), 0).Opacity(p)
	if !on {
		panel.Disabled(true)
	}
}

func (a *App) foldLeaving(side bool) bool {
	m := &a.fold
	if side {
		return m.progress < 1 && m.from.side && !m.last.side
	}
	return m.progress < 1 && m.from.pane && !m.last.pane
}
