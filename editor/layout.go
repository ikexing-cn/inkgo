package editor

import (
	"github.com/egoist/mygo/ui"
)

// Layout constants, in DIPs.
const (
	padX, padTop, padBottom = 40, 32, 120
	maxTextWidth            = 760
)

// MaxTextWidth is how wide the text grows at most; a wider view centers
// it, and what sits over the text can line up with it.
const MaxTextWidth = maxTextWidth

// shaped is a paragraph Shape placed on one line, before it is broken.
type shaped struct {
	glyphs []ui.Glyph
	xs     []float32 // x of each rune boundary
}

// line is a line of the layout. Indexes are of the display text: the
// document with the composition and the prediction inserted at the caret.
type line struct {
	start, end int // runes [start, end), without the line break
	pstart     int // where the paragraph starts
	sh         *shaped
	last       bool // the last line of its paragraph
	y          float32
	dx         float32 // the indent of a paragraph's first line
}

type layout struct {
	disp                []rune
	lines               []line
	lineH, ascent       float32
	left, textW, height float32
	// The text inserted at the caret, as display indexes.
	extraAt, preEnd, extraEnd int
}

// buildLayout lays the text out for an element width wide.
func (e *Editor) buildLayout(width float32) {
	f := e.opts.Font
	m := f.Metrics()
	l := layout{}
	l.lineH = f.Size * 1.8
	l.ascent = (l.lineH-(m.Ascent+m.Descent))/2 + m.Ascent
	l.textW = max(min(width-2*padX, maxTextWidth), 60)
	l.left = max((width-l.textW)/2, 0)

	extra, at := e.extra()
	pre, ghost := []rune(e.preedit), []rune(extra)
	text := e.Doc.text
	at = clamp(at, 0, len(text))
	l.disp = make([]rune, 0, len(text)+len(pre)+len(ghost))
	l.disp = append(append(append(append(l.disp, text[:at]...), pre...), ghost...), text[at:]...)
	l.extraAt, l.preEnd, l.extraEnd = at, at+len(pre), at+len(pre)+len(ghost)

	used := make(map[string]shaped, len(e.shapes))
	y := float32(padTop)
	paraGap := f.Size * 0.5
	for ps := 0; ps <= len(l.disp); {
		pe := ps
		for pe < len(l.disp) && l.disp[pe] != '\n' {
			pe++
		}
		key := string(l.disp[ps:pe])
		sh, ok := used[key]
		if !ok {
			if sh, ok = e.shapes[key]; !ok {
				sh = shapeText(key, f)
			}
			used[key] = sh
		}
		shp := &sh
		indent := float32(0)
		if e.opts.Prose {
			indent = 2 * f.Size
		}
		starts := breakLines(l.disp[ps:pe], sh.xs, l.textW, indent)
		for i, s := range starts {
			end := pe - ps
			if i+1 < len(starts) {
				end = starts[i+1]
			}
			ln := line{start: ps + s, end: ps + end, pstart: ps, sh: shp, last: i == len(starts)-1, y: y}
			if i == 0 {
				ln.dx = indent
			}
			l.lines = append(l.lines, ln)
			y += l.lineH
		}
		y += paraGap
		ps = pe + 1
	}
	l.height = y - paraGap + padBottom
	e.shapes = used
	e.lay = l
}

func shapeText(s string, f ui.Font) shaped {
	gs := ui.Shape(s, f)
	pos := make([]glyphPos, len(gs))
	for i, g := range gs {
		pos[i] = glyphPos{cluster: g.Cluster, x: g.X, advance: g.Advance}
	}
	return shaped{glyphs: gs, xs: positions(len([]rune(s)), pos)}
}

// x returns the x of display index d on line ln, relative to the text.
func (l *layout) x(ln *line, d int) float32 {
	return ln.dx + ln.sh.xs[d-ln.pstart] - ln.sh.xs[ln.start-ln.pstart]
}

// lineOf returns the line showing display index d.
func (l *layout) lineOf(d int) int {
	for i := range l.lines {
		ln := &l.lines[i]
		if d >= ln.start && (d < ln.end || d == ln.end && ln.last) {
			return i
		}
	}
	return max(len(l.lines)-1, 0)
}

// lineAt returns the line at y, relative to the content.
func (l *layout) lineAt(y float32) int {
	best := 0
	for i := range l.lines {
		if l.lines[i].y <= y {
			best = i
		}
	}
	return best
}

// hitLine returns the display index nearest to x on line i.
func (l *layout) hitLine(i int, x float32) int {
	ln := &l.lines[i]
	best, dist := ln.start, float32(-1)
	for d := ln.start; d <= ln.end; d++ {
		dx := l.x(ln, d) - x
		if dx < 0 {
			dx = -dx
		}
		if dist < 0 || dx < dist {
			best, dist = d, dx
		}
	}
	return best
}

// toDoc maps a display index to the document; the inserted text maps to
// the caret.
func (l *layout) toDoc(d int) int {
	switch {
	case d <= l.extraAt:
		return d
	case d < l.extraEnd:
		return l.extraAt
	}
	return d - (l.extraEnd - l.extraAt)
}

// toDisp maps a document index to the display; after puts the caret's
// own index after the inserted text.
func (l *layout) toDisp(i int, after bool) int {
	if i < l.extraAt || i == l.extraAt && !after {
		return i
	}
	return i + l.extraEnd - l.extraAt
}

// ranges calls fn with the part of display range [a, b) on each line, in
// content coordinates relative to the text's left.
func (l *layout) ranges(a, b int, fn func(x0, x1, y float32)) {
	for i := range l.lines {
		ln := &l.lines[i]
		s, t := max(a, ln.start), min(b, ln.end)
		// The line break after a paragraph's last line counts as selected
		// when the range runs past it.
		brk := ln.last && b > ln.end && a <= ln.end
		if s > t || s == t && !brk {
			continue
		}
		x0, x1 := l.x(ln, s), l.x(ln, t)
		if brk {
			x1 += 6
		}
		fn(x0, x1, ln.y)
	}
}
