package editor

import (
	"github.com/egoist/mygo/ui"
)

// Completion popup metrics, in DIPs.
const (
	popRowH, popPad, popW = 28, 4, 200
)

var (
	colorForeshadow = ui.RGBA(230, 160, 40, 0.22)
	colorForeLine   = ui.RGB(214, 140, 20)
	colorAI         = ui.RGB(90, 140, 230)
)

// paint draws the text, the marks, the selection and the caret.
func (e *Editor) paint(p *ui.Painter, r ui.Rect) {
	t := p.Theme()
	if r.W != e.width || r.H != e.height {
		e.width, e.height = r.W, r.H
		if e.c != nil {
			e.c.Invalidate() // the caret given to input methods moved
		}
	}
	e.ensureLayout()
	l := &e.lay
	if e.reveal {
		e.reveal = false
		_, cy := e.caretPos()
		if cy < e.scrollY {
			e.scrollY = cy - l.lineH
		} else if cy+l.lineH > e.scrollY+r.H {
			e.scrollY = cy + 2*l.lineH - r.H
		}
	}
	e.scrollY = clampf(e.scrollY, 0, max(l.height-r.H, 0))
	ox, oy := r.X+l.left, r.Y-e.scrollY
	top, bottom := e.scrollY-l.lineH, e.scrollY+r.H

	// Marks under the text.
	for _, m := range e.Doc.marks {
		if m.End <= m.Start {
			continue
		}
		a, b := l.toDisp(m.Start, true), l.toDisp(m.End, false)
		l.ranges(a, b, func(x0, x1, y float32) {
			if y < top || y > bottom {
				return
			}
			base := oy + y + l.lineH - 5
			switch m.Kind {
			case MarkForeshadow:
				p.Fill(ui.Rect{X: ox + x0, Y: oy + y + 4, W: x1 - x0, H: l.lineH - 8}, colorForeshadow, 2)
				p.Line(ox+x0, base, ox+x1, base, 1.5, colorForeLine)
			case MarkAIAccepted:
				p.Line(ox+x0, base, ox+x1, base, 1, colorAI.Alpha(0.55))
			}
		})
	}

	// The selection.
	if a, b := e.Selection(); a != b {
		sel := t.Selection
		if !e.focused {
			sel = t.TextMuted.Alpha(0.2)
		}
		l.ranges(l.toDisp(a, true), l.toDisp(b, false), func(x0, x1, y float32) {
			if y >= top && y <= bottom {
				p.Fill(ui.Rect{X: ox + x0, Y: oy + y + 2, W: x1 - x0, H: l.lineH - 4}, sel, 0)
			}
		})
	}

	// The text, by runs of one color.
	ghostColor := t.TextMuted.Alpha(0.75)
	for i := range l.lines {
		ln := &l.lines[i]
		if ln.y < top || ln.y > bottom {
			continue
		}
		x := ox - ln.sh.xs[ln.start-ln.pstart]
		y := oy + ln.y + l.ascent
		e.drawRun(p, ln, ln.start, min(ln.end, l.extraAt), x, y, t.Text)
		e.drawRun(p, ln, max(ln.start, l.extraAt), min(ln.end, l.preEnd), x, y, t.Text)
		e.drawRun(p, ln, max(ln.start, l.preEnd), min(ln.end, l.extraEnd), x, y, ghostColor)
		e.drawRun(p, ln, max(ln.start, l.extraEnd), ln.end, x, y, t.Text)
	}

	// The composition is underlined.
	if l.preEnd > l.extraAt {
		l.ranges(l.extraAt, l.preEnd, func(x0, x1, y float32) {
			base := oy + y + l.lineH - 6
			p.Line(ox+x0, base, ox+x1, base, 1.5, t.Text)
		})
	}

	// The caret.
	if e.focused {
		cx, cy := e.caretPos()
		h := e.opts.Font.Size * 1.3
		p.Fill(ui.Rect{X: r.X + cx - 0.5, Y: oy + cy + (l.lineH-h)/2, W: 1.5, H: h}, t.Accent, 0)
	}

	// A scrollbar, while the text is longer than the view.
	if l.height > r.H {
		h := max(r.H*r.H/l.height, 24)
		y := r.Y + (r.H-h)*e.scrollY/max(l.height-r.H, 1)
		p.Fill(ui.Rect{X: r.X + r.W - 8, Y: y, W: 4, H: h}, t.Scrollbar, 2)
	}
}

// drawRun draws the glyphs of display runes [a, b) of line ln.
func (e *Editor) drawRun(p *ui.Painter, ln *line, a, b int, x, y float32, c ui.Color) {
	if a >= b {
		return
	}
	ra, rb := a-ln.pstart, b-ln.pstart
	var run []ui.Glyph
	for _, g := range ln.sh.glyphs {
		if g.Cluster >= ra && g.Cluster < rb {
			run = append(run, g)
		}
	}
	if len(run) > 0 {
		p.Glyphs(run, x, y, c)
	}
}

// paintPopup draws the entity completions under the caret.
func (e *Editor) paintPopup(p *ui.Painter, r ui.Rect) {
	if !e.compOpen || len(e.comp) == 0 || !e.focused {
		e.popRect = ui.Rect{}
		return
	}
	t := p.Theme()
	cx, cy := e.caretPos()
	l := &e.lay
	x := min(cx-4, r.W-popW-8)
	y := cy - e.scrollY + l.lineH
	h := float32(len(e.comp))*popRowH + 2*popPad
	if y+h > r.H && cy-e.scrollY-h > 0 {
		y = cy - e.scrollY - h
	}
	e.popRect = ui.Rect{X: x, Y: y, W: popW, H: h}
	box := ui.Rect{X: r.X + x, Y: r.Y + y, W: popW, H: h}
	p.Shadow(box, 6, 0, 2, 12, 0, ui.RGBA(0, 0, 0, 0.18))
	p.Fill(box, t.Surface, 6)
	p.Stroke(box, t.Border, 6, 1)
	for i, en := range e.comp {
		row := ui.Rect{X: box.X + popPad, Y: box.Y + popPad + float32(i)*popRowH, W: popW - 2*popPad, H: popRowH}
		fg := t.Text
		if i == e.compSel {
			p.Fill(row, t.Accent, 4)
			fg = t.AccentText
		}
		p.Text(row.X+8, row.Y+5, en.Name, 14, fg)
		p.Text(row.X+row.W-44, row.Y+7, en.Kind, 12, fg.Alpha(0.7))
	}
}
