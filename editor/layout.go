package editor

import (
	"slices"

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

// PadX is the margin beside the text, for what sits over it to line up.
const PadX = padX

// shaped is a paragraph Shape placed on one line, before it is broken.
type shaped struct {
	glyphs []ui.Glyph
	xs     []float32 // x of each rune boundary
}

// line is a line of the layout. Indexes are of the display text: the
// document with the composition and the prediction inserted at the caret,
// and what an AI proposes after the words it replaces or follows.
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
	kinds               []runeKind // what each rune of disp is
	lines               []line
	lineH, ascent       float32
	left, textW, height float32
	// The text inserted at the caret, as display indexes.
	extraAt, preEnd, extraEnd int
	// All that is inserted, in the order it shows; and where the room
	// after each proposal is, by its ID.
	ins  []insert
	gaps map[string]int
}

// insert is text shown in the text without being in it: n runes at the
// document index at.
type insert struct{ at, n int }

// runeKind is what a rune of the display is, which tells how it is drawn.
type runeKind uint8

const (
	runeText   runeKind = iota // of the text, or of the composition
	runeGhost                  // of a prediction
	runeNew                    // of what an AI proposes
	runeStruck                 // of the text, to be replaced by what is proposed
	runeGap                    // the room after what is proposed
)

// buildLayout lays the text out for an element width wide.
func (e *Editor) buildLayout(width float32) {
	f := e.opts.Font
	m := f.Metrics()
	l := layout{}
	lh := e.opts.LineHeight
	if lh == 0 {
		lh = 1.8
	}
	l.lineH = f.Size * lh
	l.ascent = (l.lineH-(m.Ascent+m.Descent))/2 + m.Ascent
	l.textW = max(min(width-2*padX, maxTextWidth), 60)
	l.left = max((width-l.textW)/2, 0)

	e.display(&l)

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
				sh = shapeText(key, f, e.gap)
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

// display builds the display text of l: the document with what shows in
// it without being in it, the composition and the prediction at the caret
// and what is proposed after the words it replaces or follows, the caret's
// first where they meet.
func (e *Editor) display(l *layout) {
	text := e.Doc.text
	type piece struct {
		at    int
		caret bool
		p     Proposal
	}
	var pieces []piece
	extra, at := e.extra()
	pre, ghost := []rune(e.preedit), []rune(extra)
	if len(pre)+len(ghost) > 0 {
		pieces = append(pieces, piece{at: clamp(at, 0, len(text)), caret: true})
	}
	struck := map[int]bool{}
	for _, p := range e.props {
		pieces = append(pieces, piece{at: clamp(p.B, 0, len(text)), p: p})
		if !p.Keep {
			for i := clamp(p.A, 0, len(text)); i < clamp(p.B, 0, len(text)); i++ {
				struck[i] = true
			}
		}
	}
	slices.SortStableFunc(pieces, func(a, b piece) int { return a.at - b.at })
	l.disp = make([]rune, 0, len(text)+len(pre)+len(ghost))
	l.gaps = map[string]int{}
	add := func(r []rune, k runeKind) {
		l.disp = append(l.disp, r...)
		for range r {
			l.kinds = append(l.kinds, k)
		}
	}
	doc := func(a, b int) {
		for i := a; i < b; i++ {
			k := runeText
			if struck[i] {
				k = runeStruck
			}
			add(text[i:i+1], k)
		}
	}
	from := 0
	for _, pc := range pieces {
		doc(from, pc.at)
		from = pc.at
		start := len(l.disp)
		if pc.caret {
			l.extraAt = start
			add(pre, runeText)
			l.preEnd = len(l.disp)
			add(ghost, runeGhost)
			l.extraEnd = len(l.disp)
		} else {
			if pc.p.Apart {
				add([]rune{'\n'}, runeNew)
			}
			add([]rune(pc.p.Text), runeNew)
			l.gaps[pc.p.ID] = len(l.disp)
			add([]rune{gapRune}, runeGap)
		}
		l.ins = append(l.ins, insert{pc.at, len(l.disp) - start})
	}
	doc(from, len(text))
	if len(pre)+len(ghost) == 0 {
		at := l.toDisp(clamp(e.caret, 0, len(text)), false)
		l.extraAt, l.preEnd, l.extraEnd = at, at, at
	}
}

// shapeText shapes s, giving each rune that holds room after what is
// proposed the width gap, and no glyph to draw.
func shapeText(s string, f ui.Font, gap float32) shaped {
	gs := ui.Shape(s, f)
	pos := make([]glyphPos, len(gs))
	for i, g := range gs {
		pos[i] = glyphPos{cluster: g.Cluster, x: g.X, advance: g.Advance}
	}
	r := []rune(s)
	xs := positions(len(r), pos)
	if !slices.Contains(r, gapRune) {
		return shaped{glyphs: gs, xs: xs}
	}
	for k, c := range r {
		if c != gapRune {
			continue
		}
		d := gap - (xs[k+1] - xs[k])
		for j := k + 1; j < len(xs); j++ {
			xs[j] += d
		}
		for i := range gs {
			if gs[i].Cluster > k {
				gs[i].X += d
			}
		}
	}
	gs = slices.DeleteFunc(gs, func(g ui.Glyph) bool { return g.Cluster >= 0 && g.Cluster < len(r) && r[g.Cluster] == gapRune })
	return shaped{glyphs: gs, xs: xs}
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

// toDoc maps a display index to the document; inserted text maps to the
// index it shows at.
func (l *layout) toDoc(d int) int {
	shift := 0
	for _, in := range l.ins {
		start := in.at + shift
		if d < start {
			break
		}
		if d < start+in.n {
			return in.at
		}
		shift += in.n
	}
	return d - shift
}

// toDisp maps a document index to the display; after puts it after the
// text inserted at it.
func (l *layout) toDisp(i int, after bool) int {
	d := i
	for _, in := range l.ins {
		if in.at > i || in.at == i && !after {
			break
		}
		d += in.n
	}
	return d
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
