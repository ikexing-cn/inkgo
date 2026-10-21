package editor

import (
	"github.com/egoist/mygo/ui"
	"math"
	"strings"
	"writer/shelf"
)

const (
	phoneTitleEM       = 1.3
	phoneTitleTopEM    = 0.4
	phoneTitleBottomEM = 1.2
	phoneIndentEM      = 2
	phoneParaGapEM     = 0.5
	lhRead             = 1.8
	lhLoose            = 1.7
	lhPaper            = 1.95
	lhTight            = 1.4
)

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
	Para, Line, Page int // para -1 is the chapter title
	Y, RollY         float32
}

type PaginatedText struct {
	cache                         map[pageShapeKey]*Paragraph
	TitleShape                    *Paragraph
	Paras                         []PageParagraph
	Lines                         []PageLine
	Pages                         int
	PageHeight, TextWidth, Height float32
	LongPages                     []int
	LongCount                     int
}

func (s *PaginatedText) Layout(text, title string, p shelf.Phone, width, height float32) {
	s.TextWidth, s.PageHeight = width, height
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
	fs := float32(p.Size)
	lh := float32(lhRead)
	switch p.Spacing {
	case "tight":
		lh = lhLoose
	case "loose":
		lh = lhPaper
	}
	titleSize := phoneTitleEM * fs
	s.TitleShape = shape(title, ui.Font{Size: titleSize, Weight: 700}, lhTight, 0)
	y, rollY, page := titleSize*phoneTitleTopEM, titleSize*phoneTitleTopEM, 0
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
	y += titleSize * phoneTitleBottomEM
	rollY += titleSize * phoneTitleBottomEM
	start := 0
	for i, text := range strings.Split(text, "\n") {
		sh := shape(text, ui.Font{Size: fs}, lh, phoneIndentEM*fs)
		if y+sh.LineHeight > s.PageHeight {
			page++
			y = 0
		}
		s.Paras = append(s.Paras, PageParagraph{start, sh, rollY})
		if !p.NoLong && len(sh.Lines) > 5 {
			s.LongCount++
			if len(s.LongPages) == 0 || s.LongPages[len(s.LongPages)-1] != page {
				s.LongPages = append(s.LongPages, page)
			}
		}
		add(i, sh)
		y += sh.LineHeight * phoneParaGapEM
		rollY += sh.LineHeight * phoneParaGapEM
		start += len([]rune(text)) + 1
	}
	s.cache, s.Height, s.Pages = used, rollY, page+1
	if p.Mode == "scroll" {
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

func (s *PaginatedText) LineY(l PageLine, mode string, scrollY float32) float32 {
	if mode == "scroll" {
		return l.RollY - scrollY
	}
	return l.Y
}

func (s *PaginatedText) Hit(x, y float32, mode string, at int, scrollY float32, body ui.Rect, ox, oy, scale float32) (para, offset int, ok bool) {
	if scale <= 0 {
		return
	}
	x, y = (x-ox)/scale, (y-oy)/scale
	if x < body.X || x > body.X+body.W || y < body.Y || y > body.Y+body.H {
		return
	}
	x, y = x-body.X, y-body.Y
	for _, l := range s.Lines {
		if l.Para < 0 || mode == "flip" && l.Page != at {
			continue
		}
		p := s.Paras[l.Para]
		ly := s.LineY(l, mode, scrollY)
		if y >= ly && y < ly+p.Shape.LineHeight {
			return l.Para, p.Start + p.Shape.HitAfter(l.Line, x), true
		}
	}
	return
}
