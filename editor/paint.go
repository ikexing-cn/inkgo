package editor

import (
	"github.com/egoist/mygo/ui"
)

// Completion popup metrics, in DIPs.
const (
	popRowH, popPad, popW = 28, 4, 240
)

var (
	colorAI    = ui.RGB(90, 140, 230)
	colorFound = ui.RGB(240, 190, 40)
)

// paint draws the text, the marks, the selection and the caret, inside
// the element: a line scrolled half out must not draw over what is above.
func (e *Editor) paint(p *ui.Painter, r ui.Rect) {
	p.Clip(r, 0, func() { e.paintIn(p, r) })
}

func (e *Editor) paintIn(p *ui.Painter, r ui.Rect) {
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
		switch {
		case e.typewriter && e.Doc.version != e.centered:
			// only the text changing moves it, not a click or an arrow
			e.centered = e.Doc.version
			e.scrollY = cy + l.lineH/2 - r.H/2
		case cy < e.scrollY:
			e.scrollY = cy - l.lineH
		case cy+l.lineH > e.scrollY+r.H:
			e.scrollY = cy + 2*l.lineH - r.H
		}
	}
	e.scrollY = clampf(e.scrollY, 0, e.maxScroll())
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
				dotted(p, ox+x0, ox+x1, base, t.TextMuted.Alpha(0.75))
			case MarkAIAccepted:
				p.Line(ox+x0, base, ox+x1, base, 1, colorAI.Alpha(0.55))
			case MarkAIDraft:
				p.Line(ox+x0, base, ox+x1, base, 1, colorAI.Alpha(0.3))
			}
		})
	}

	// What a search found.
	for i, f := range e.found {
		color := colorFound.Alpha(0.28)
		if i == e.current {
			color = colorFound.Alpha(0.6)
		}
		l.ranges(l.toDisp(f[0], true), l.toDisp(f[1], false), func(x0, x1, y float32) {
			if y >= top && y <= bottom {
				p.Fill(ui.Rect{X: ox + x0, Y: oy + y + 3, W: x1 - x0, H: l.lineH - 6}, color, 2)
			}
		})
	}

	// The placeholder of an empty text.
	if e.Doc.Len() == 0 && e.preedit == "" && e.opts.Placeholder != "" && len(l.lines) > 0 {
		p.Text(ox+l.lines[0].dx, oy+l.lines[0].y+(l.lineH-e.opts.Font.Size*1.3)/2, e.opts.Placeholder, e.opts.Font.Size, t.TextMuted.Alpha(0.6))
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

	// A rewrite proposed: its words struck through, the new ones tinted.
	if e.proposing() {
		l.ranges(l.toDisp(e.prop.a, true), l.toDisp(e.prop.b, false), func(x0, x1, y float32) {
			if y >= top && y <= bottom {
				p.Fill(ui.Rect{X: ox + x0, Y: oy + y + 3, W: x1 - x0, H: l.lineH - 6}, t.Danger.Alpha(0.1), 0)
				mid := oy + y + l.lineH/2
				p.Line(ox+x0, mid, ox+x1, mid, 1.2, t.Danger.Alpha(0.8))
			}
		})
		l.ranges(l.preEnd, l.extraEnd, func(x0, x1, y float32) {
			if y >= top && y <= bottom {
				p.Fill(ui.Rect{X: ox + x0, Y: oy + y + 3, W: x1 - x0, H: l.lineH - 6}, t.Success.Alpha(0.14), 0)
			}
		})
	}

	// The text, by runs of one color.
	ghostColor := t.TextMuted.Alpha(0.75)
	if e.proposing() {
		ghostColor = t.Success
	}
	for i := range l.lines {
		ln := &l.lines[i]
		if ln.y < top || ln.y > bottom {
			continue
		}
		x := ox + ln.dx - ln.sh.xs[ln.start-ln.pstart]
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
	if full := e.maxScroll() + r.H; full > r.H {
		h := max(r.H*r.H/full, 24)
		y := r.Y + (r.H-h)*e.scrollY/max(full-r.H, 1)
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
		if en.Of != "" {
			w, _ := p.MeasureText(0, ui.Span{Text: en.Name, Size: 14})
			x := row.X + 8 + w + 6
			p.Text(x, row.Y+7, "· "+en.Of, 12, fg.Alpha(0.7))
		}
		p.Text(row.X+row.W-44, row.Y+7, en.Kind, 12, fg.Alpha(0.7))
	}
}

// dotted draws a light dotted underline, the way a foreshadowing is marked:
// it stays out of the way of reading.
func dotted(p *ui.Painter, x0, x1, y float32, c ui.Color) {
	const d, gap = 1.5, 3
	for x := x0; x+d <= x1; x += gap {
		p.Fill(ui.Rect{X: x, Y: y - d/2, W: d, H: d}, c, d/2)
	}
}
