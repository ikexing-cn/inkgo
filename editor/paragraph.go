package editor

import "github.com/egoist/mygo/ui"

// Paragraph is immutable measured prose. It shares the editor's shaping,
// rune positions and punctuation-aware wrapping, without editor view state.
// Callers may retain it while its text, font, width and indent are unchanged.
type Paragraph struct {
	Lines              []TextLine
	LineHeight, Height float32
	sh                 shaped
	ascent             float32
}

// TextLine indexes runes of its paragraph, excluding the line break.
type TextLine struct {
	Start, End int
	X          float32
	glyphs     []ui.Glyph
}

func LayoutParagraph(text string, font ui.Font, width, lineHeight, indent float32) *Paragraph {
	if font.Family == "" {
		font.Family = defaultFontFamily
	}
	sh := shapeText(text, font, 0)
	runes := []rune(text)
	starts := breakLines(runes, sh.xs, width, indent)
	m := font.Metrics()
	p := &Paragraph{sh: sh, LineHeight: font.Size * lineHeight}
	p.ascent = (p.LineHeight-m.Ascent-m.Descent)/2 + m.Ascent
	for i, s := range starts {
		end := len(runes)
		if i+1 < len(starts) {
			end = starts[i+1]
		}
		ln := TextLine{Start: s, End: end}
		if i == 0 {
			ln.X = indent
		}
		for _, g := range sh.glyphs {
			if g.Cluster >= s && g.Cluster < end {
				ln.glyphs = append(ln.glyphs, g)
			}
		}
		p.Lines = append(p.Lines, ln)
	}
	p.Height = float32(len(p.Lines)) * p.LineHeight
	return p
}

// X is the rune boundary on a measured line, relative to the paragraph.
func (p *Paragraph) X(line, offset int) float32 {
	l := p.Lines[line]
	return l.X + p.sh.xs[clamp(offset, l.Start, l.End)] - p.sh.xs[l.Start]
}

// HitAfter places the caret after the glyph under x. Blank space before
// and after a line maps to its first and last boundary respectively.
func (p *Paragraph) HitAfter(line int, x float32) int {
	l := p.Lines[line]
	if x <= l.X {
		return l.Start
	}
	for i := l.Start; i < l.End; i++ {
		if x < p.X(line, i+1) {
			return i + 1
		}
	}
	return l.End
}

// DrawLine draws cached glyphs with the line's top at y.
func (p *Paragraph) DrawLine(paint *ui.Painter, line int, x, y float32, color ui.Color) {
	l := p.Lines[line]
	paint.Glyphs(l.glyphs, x+l.X-p.sh.xs[l.Start], y+p.ascent, color)
}

// ScrollAnchor matches a fraction through the prose viewport to a
// paragraph and a fraction of its height. A second layout can place that
// same point at that same viewport fraction, including both endpoints.
func (e *Editor) ScrollAnchor() (fraction float32, paragraph int, into float32) {
	e.ensureLayout()
	if most := e.maxScroll(); most > 0 {
		fraction = clampf(e.scrollY/most, 0, 1)
	}
	y := e.scrollY + fraction*e.height
	lines := e.lay.lines
	for i := 0; i < len(lines); {
		j := i + 1
		for j < len(lines) && lines[j].pstart == lines[i].pstart {
			j++
		}
		bottom := lines[j-1].y + e.lay.lineH
		if bottom >= y || j == len(lines) {
			// Proposed paragraphs and IME text are display-only. Locate
			// this point in the source document before naming its paragraph.
			paragraph = 0
			for _, r := range e.Doc.text[:clamp(e.lay.toDoc(lines[i].pstart), 0, e.Doc.Len())] {
				if r == '\n' {
					paragraph++
				}
			}
			return fraction, paragraph, clampf((y-lines[i].y)/max(bottom-lines[i].y, 1), 0, 1)
		}
		i = j
	}
	return
}
