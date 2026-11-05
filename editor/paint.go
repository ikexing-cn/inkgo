package editor

import (
	"slices"

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
	e.paintBar(p, r)
}

func (e *Editor) paintIn(p *ui.Painter, r ui.Rect) {
	if r.W != e.width || r.H != e.height {
		e.width, e.height = r.W, r.H
		e.sv.Invalidate() // the caret given to input methods moved
	}
	e.ensureLayout()
	e.scrollIntoView(r)
	l := &e.lay
	ox, oy := r.X+l.left, r.Y-e.scrollY
	top, bottom := e.scrollY-l.lineH, e.scrollY+r.H
	e.paintDecorations(p, ox, oy, top, bottom)
	e.paintText(p, ox, oy, top, bottom)
	e.paintCaret(p, r, oy)
}

func (e *Editor) scrollIntoView(r ui.Rect) {
	l := &e.lay
	// What is proposed comes into view: its start when the host asks, its
	// end as it grows while that is in view. Asked, it is what is seen,
	// though the caret, as a tab comes back, would be.
	for _, id := range []string{e.show, e.follow} {
		d, ok := l.gaps[id]
		if !ok || len(l.lines) == 0 {
			continue
		}
		if id == e.show {
			d = d - len([]rune(e.props[slices.IndexFunc(e.props, func(p Proposal) bool { return p.ID == id })].Text))
			e.reveal = false
		}
		cy := l.lines[l.lineOf(d)].y
		switch {
		case cy < e.scrollY:
			e.scrollY = cy - l.lineH
		case cy+l.lineH > e.scrollY+r.H:
			e.scrollY = cy + 2*l.lineH - r.H
		}
	}
	e.show, e.follow = "", ""
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
}

func (e *Editor) paintDecorations(p *ui.Painter, ox, oy, top, bottom float32) {
	t := p.Theme()
	l := &e.lay
	if a, b := e.noteHighlight[0], e.noteHighlight[1]; b > a {
		l.eachVisible(l.toDisp(a, true), l.toDisp(b, false), top, bottom, func(x0, x1, y float32) {
			p.Fill(ui.Rect{X: ox + x0, Y: oy + y, W: x1 - x0, H: l.lineH}, t.SurfacePressed, 0)
		})
	}
	e.paintNames(p, ox, oy, top, bottom)

	for _, m := range e.decorations() {
		if m.End <= m.Start {
			continue
		}
		a, b := l.toDisp(m.Start, true), l.toDisp(m.End, false)
		l.eachVisible(a, b, top, bottom, func(x0, x1, y float32) {
			base := oy + y + l.lineH - 5
			switch m.Kind {
			case MarkForeshadow:
				if m.Dropped {
					dotted(p, ox+x0, ox+x1, base, t.TextMuted.Alpha(0.3))
				} else {
					dotted(p, ox+x0, ox+x1, base, t.TextMuted.Alpha(0.75))
				}
			case MarkPayoff:
				dotted(p, ox+x0, ox+x1, base, t.Text.Alpha(0.55))
			case MarkCheck:
				color := e.doubtColor
				if color.A == 0 {
					color = t.Warning
				}
				dashed(p, ox+x0, ox+x1, base, color)
			case MarkAI:
				p.Line(ox+x0, base, ox+x1, base, 1, colorAI.Alpha(0.55))
			}
		})
	}

	for i, f := range e.found {
		color := colorFound.Alpha(0.28)
		if i == e.current {
			color = colorFound.Alpha(0.6)
		}
		l.eachVisible(l.toDisp(f[0], true), l.toDisp(f[1], false), top, bottom, func(x0, x1, y float32) {
			p.Fill(ui.Rect{X: ox + x0, Y: oy + y + 3, W: x1 - x0, H: l.lineH - 6}, color, 2)
		})
	}

	if e.Doc.Len() == 0 && e.preedit == "" && e.opts.Placeholder != nil && len(l.lines) > 0 {
		p.Text(ox+l.lines[0].dx, oy+l.lines[0].y+(l.lineH-e.opts.Font.Size*1.3)/2, e.opts.Placeholder(), e.opts.Font.Size, t.TextMuted.Alpha(0.6))
	}

	if a, b := e.Selection(); a != b {
		sel := t.Selection
		if !e.focused {
			sel = t.TextMuted.Alpha(0.2)
		}
		l.eachVisible(l.toDisp(a, true), l.toDisp(b, false), top, bottom, func(x0, x1, y float32) {
			p.Fill(ui.Rect{X: ox + x0, Y: oy + y + 2, W: x1 - x0, H: l.lineH - 4}, sel, 0)
		})
	}

	for _, pr := range e.props {
		if pr.Keep {
			continue
		}
		l.eachVisible(l.toDisp(pr.A, true), l.toDisp(pr.B, false), top, bottom, func(x0, x1, y float32) {
			if x1 > x0 {
				mid := oy + y + l.lineH/2
				p.Line(ox+x0, mid, ox+x1, mid, 1.2, t.Danger)
			}
		})
	}
}

func (e *Editor) paintText(p *ui.Painter, ox, oy, top, bottom float32) {
	t := p.Theme()
	l := &e.lay
	colors := [...]ui.Color{runeText: t.Text, runeGhost: t.TextMuted.Alpha(0.75), runeNew: t.Success, runeStruck: t.Danger}
	for i := range l.lines {
		ln := &l.lines[i]
		if ln.y < top || ln.y > bottom {
			continue
		}
		x := ox + ln.dx - ln.sh.xs[ln.start-ln.pstart]
		y := oy + ln.y + l.ascent
		if l.kinds == nil {
			e.drawRun(p, ln, ln.start, ln.end, x, y, t.Text)
			continue
		}
		for a := ln.start; a < ln.end; {
			k := l.kind(a)
			b := a + 1
			for b < ln.end && l.kind(b) == k {
				b++
			}
			if k != runeGap {
				e.drawRun(p, ln, a, b, x, y, colors[k])
			}
			a = b
		}
	}

	if l.preEnd > l.extraAt {
		l.ranges(l.extraAt, l.preEnd, func(x0, x1, y float32) {
			base := oy + y + l.lineH - 6
			p.Line(ox+x0, base, ox+x1, base, 1.5, t.Text)
		})
	}
}

func (e *Editor) paintCaret(p *ui.Painter, r ui.Rect, oy float32) {
	t := p.Theme()
	l := &e.lay
	if e.focused {
		cx, cy := e.caretPos()
		h := e.opts.Font.Size * 1.3
		p.Fill(ui.Rect{X: r.X + cx - 0.5, Y: oy + cy + (l.lineH-h)/2, W: 1.5, H: h}, t.Accent, 0)
	}

}

func (e *Editor) paintBar(p *ui.Painter, r ui.Rect) {
	if e.maxScroll() > 0 {
		y, h := e.thumb()
		// Wider while the pointer is on the bar or holds it.
		x, w := r.X+r.W+e.barOffset-8, float32(4)
		if e.barHot || e.barDrag {
			x, w = r.X+r.W+e.barOffset-10, 8
		}
		p.Fill(ui.Rect{X: x, Y: r.Y + y, W: w, H: h}, p.Theme().Scrollbar, w/2)
	}
}

// drawRun draws the glyphs of display runes [a, b) of line ln.
func (e *Editor) drawRun(p *ui.Painter, ln *line, a, b int, x, y float32, c ui.Color) {
	if a >= b {
		return
	}
	ra, rb := a-ln.pstart, b-ln.pstart
	var run []ui.Glyph
	if ln.sh.first == nil {
		run = ln.sh.view(ln.start-ln.pstart, ln.end-ln.pstart).run(ra, rb)
		x += ln.sh.xs[ln.start-ln.pstart]
	} else {
		run = ln.sh.run(ra, rb)
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

// dashed draws a light dashed underline, the way a doubt is shown.
func dashed(p *ui.Painter, x0, x1, y float32, c ui.Color) {
	const d, gap = 5, 3
	for x := x0; x < x1; x += d + gap {
		p.Line(x, y, min(x+d, x1), y, 1, c)
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
