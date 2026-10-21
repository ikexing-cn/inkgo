package editor

import (
	"math"
	"strings"

	"github.com/egoist/mygo/ui"
)

// PaginationOptions uses DIPs for dimensions, indents and title margins.
// Line heights multiply font size; ParagraphGap multiplies body line height.
type PaginationOptions struct {
	Width, Height               float32
	Font, TitleFont             ui.Font
	LineHeight, TitleLineHeight float32
	Indent, ParagraphGap        float32
	TitleTop, TitleBottom       float32
	MarkLong, Scroll            bool
}

type pageShapeKey struct {
	text                  string
	font                  ui.Font
	width, height, indent float32
}

type PageParagraph struct {
	Start int
	Shape *Paragraph
	RollY float32
}

type PageLine struct {
	Para, Line, Page int // Para -1 is the chapter title
	Y, RollY         float32
}

// PaginatedText retains measured paragraphs between layouts. Offsets are in runes.
type PaginatedText struct {
	scroll                        bool
	cache                         map[pageShapeKey]*Paragraph
	TitleShape                    *Paragraph
	Paras                         []PageParagraph
	Lines                         []PageLine
	Pages                         int
	PageHeight, TextWidth, Height float32
	LongPages                     []int
	LongCount                     int
}

func (s *PaginatedText) Layout(text, title string, o PaginationOptions) {
	s.TextWidth, s.PageHeight, s.scroll = o.Width, o.Height, o.Scroll
	s.Paras, s.Lines, s.LongPages, s.LongCount = nil, nil, nil, 0
	used := map[pageShapeKey]*Paragraph{}
	shape := func(text string, font ui.Font, lh, indent float32) *Paragraph {
		key := pageShapeKey{text, font, s.TextWidth, lh, indent}
		v := used[key]
		if v == nil {
			v = s.cache[key]
		}
		if v == nil {
			v = LayoutParagraph(text, font, s.TextWidth, lh, indent)
		}
		used[key] = v
		return v
	}
	s.TitleShape = shape(title, o.TitleFont, o.TitleLineHeight, 0)
	y, rollY, page := o.TitleTop, o.TitleTop, 0
	add := func(para int, sh *Paragraph) {
		for i := range sh.Lines {
			if y+sh.LineHeight > s.PageHeight {
				page++
				y = 0
			}
			s.Lines = append(s.Lines, PageLine{para, i, page, y, rollY})
			y += sh.LineHeight
			rollY += sh.LineHeight
		}
	}
	add(-1, s.TitleShape)
	y += o.TitleBottom
	rollY += o.TitleBottom
	start := 0
	for i, text := range strings.Split(text, "\n") {
		sh := shape(text, o.Font, o.LineHeight, o.Indent)
		if y+sh.LineHeight > s.PageHeight {
			page++
			y = 0
		}
		s.Paras = append(s.Paras, PageParagraph{start, sh, rollY})
		if o.MarkLong && len(sh.Lines) > 5 {
			s.LongCount++
			if len(s.LongPages) == 0 || s.LongPages[len(s.LongPages)-1] != page {
				s.LongPages = append(s.LongPages, page)
			}
		}
		add(i, sh)
		y += sh.LineHeight * o.ParagraphGap
		rollY += sh.LineHeight * o.ParagraphGap
		start += len([]rune(text)) + 1
	}
	s.cache, s.Height, s.Pages = used, rollY, page+1
	if o.Scroll {
		s.Pages = max(1, int(math.Ceil(float64(s.Height/s.PageHeight))))
	}
}

func (s *PaginatedText) PageOf(offset int) int {
	for _, l := range s.Lines {
		if l.Para < 0 {
			continue
		}
		p := s.Paras[l.Para]
		ln := p.Shape.Lines[l.Line]
		end := p.Start + ln.End
		if offset < end || offset == end && l.Line == len(p.Shape.Lines)-1 {
			return l.Page
		}
	}
	return max(s.Pages-1, 0)
}

func (s *PaginatedText) LineY(l PageLine, scrollY float32) float32 {
	if s.scroll {
		return l.RollY - scrollY
	}
	return l.Y
}

// Hit takes a point relative to the page's text area, before view transforms.
func (s *PaginatedText) Hit(x, y float32, at int, scrollY float32) (para, offset int, ok bool) {
	for _, l := range s.Lines {
		if l.Para < 0 || !s.scroll && l.Page != at {
			continue
		}
		p := s.Paras[l.Para]
		ly := s.LineY(l, scrollY)
		if y >= ly && y < ly+p.Shape.LineHeight {
			return l.Para, p.Start + p.Shape.HitAfter(l.Line, x), true
		}
	}
	return
}
