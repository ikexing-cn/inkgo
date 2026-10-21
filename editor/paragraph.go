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
	runes := []rune(text)
	sh, starts := layoutParagraph(runes, font, width, indent, 0, nil)
	p := &Paragraph{sh: *sh, LineHeight: font.Size * lineHeight}
	p.ascent = textAscent(font, p.LineHeight)
	for i, s := range starts {
		end := len(runes)
		if i+1 < len(starts) {
			end = starts[i+1]
		}
		ln := TextLine{Start: s, End: end}
		if i == 0 {
			ln.X = indent
		}
		ln.glyphs = sh.run(s, end)
		p.Lines = append(p.Lines, ln)
	}
	p.Height = float32(len(p.Lines)) * p.LineHeight
	return p
}

func layoutParagraph(text []rune, font ui.Font, width, indent, gap float32, sh *shaped) (*shaped, []int) {
	if sh == nil {
		s := shapeText(string(text), font, gap)
		sh = &s
	}
	return sh, breakLines(text, sh.xs, width, indent)
}

func textAscent(font ui.Font, lineHeight float32) float32 {
	m := font.Metrics()
	return (lineHeight-(m.Ascent+m.Descent))/2 + m.Ascent
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
