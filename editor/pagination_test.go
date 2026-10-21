package editor

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

const paginatedLongText = "雨水顺着屋檐往下淌，巷子尽头那盏灯晃了两下，终于灭了。她数着自己的脚步，一步，两步，三步，数到第十七步的时候，听见身后有人喊她的名字，声音很轻，像是怕惊动了什么。她没有回头，只是把手电攥得更紧，指节都发了白。"

func TestPaginatedParagraphCache(t *testing.T) {
	s := &PaginatedText{}
	opts := paginationOptions(353, 766)
	text := paginatedLongText + "\n" + paginatedLongText
	s.Layout(text, "第一章", opts)
	old := s.Paras[0].Shape
	text += "雨"
	s.Layout(text, "第一章", opts)
	if s.Paras[0].Shape != old || s.Paras[1].Shape == old {
		t.Fatal("unchanged paragraph not reused, or changed paragraph reused")
	}
	screens := map[string][2]float32{"narrow": {360, 780}, "standard": {393, 852}, "wide": {430, 932}}
	for _, key := range []string{"narrow", "wide", "standard"} {
		opts.Width, opts.Height = screens[key][0]-40, screens[key][1]-86
		s.Layout(text, "第一章", opts)
		if s.TextWidth != screens[key][0]-40 {
			t.Fatal("wrong screen width")
		}
	}
	if s.Paras[0].Shape == old {
		t.Fatal("layout with another width was reused after eviction")
	}
	for _, p := range s.Paras {
		for i, line := range p.Shape.Lines {
			if p.Shape.X(i, line.End) > s.TextWidth+1 {
				t.Fatal("line does not fit the phone")
			}
		}
	}
}

func paginationOptions(width, height float32) PaginationOptions {
	return PaginationOptions{
		Width: width, Height: height,
		Font: ui.Font{Size: 20}, TitleFont: ui.Font{Size: 26, Weight: 700},
		LineHeight: 1.8, TitleLineHeight: 1.4,
		Indent: 40, ParagraphGap: 0.5, TitleTop: 26 * 0.4, TitleBottom: 26 * 1.2,
		MarkLong: true,
	}
}

func TestPaginatedTextPagesAndOffsets(t *testing.T) {
	for _, text := range []string{"", "甲乙丙丁戊己\n\n庚辛", strings.Repeat(paginatedLongText+"\n", 4)} {
		t.Run(fmt.Sprintf("%d runes", len([]rune(text))), func(t *testing.T) {
			var s PaginatedText
			opts := paginationOptions(160, 140)
			s.Layout(text, "章名", opts)
			if len(s.Paras) != strings.Count(text, "\n")+1 || s.Pages < 1 {
				t.Fatal("paragraphs or pages", len(s.Paras), s.Pages)
			}
			start := 0
			for i, paragraph := range strings.Split(text, "\n") {
				if s.Paras[i].Start != start {
					t.Fatal("paragraph rune offset", i, s.Paras[i].Start, start)
				}
				start += len([]rune(paragraph)) + 1
			}
			lastPage, lastRoll := 0, float32(-1)
			for _, l := range s.Lines {
				var shape *Paragraph
				if l.Para < 0 {
					shape = s.TitleShape
				} else {
					shape = s.Paras[l.Para].Shape
				}
				if l.Page < lastPage || l.RollY <= lastRoll || l.Y < 0 || l.Y+shape.LineHeight > s.PageHeight {
					t.Fatalf("invalid line placement: %+v", l)
				}
				lastPage, lastRoll = l.Page, l.RollY
				if s.LineY(l, 123) != l.Y {
					t.Fatal("flip line moved with scroll")
				}
				if l.Para < 0 {
					continue
				}
				p := s.Paras[l.Para]
				ln := p.Shape.Lines[l.Line]
				if got := s.PageOf(p.Start + ln.Start); got != l.Page {
					t.Fatalf("line start on page %d, want %d", got, l.Page)
				}
				if l.Line == len(p.Shape.Lines)-1 && s.PageOf(p.Start+ln.End) != l.Page {
					t.Fatal("paragraph end moved to next page")
				}
			}
			if s.PageOf(len([]rune(text))) != s.Pages-1 {
				t.Fatal("end of text not on last page")
			}
		})
	}
}

func TestPaginatedTextHit(t *testing.T) {
	text := strings.Repeat(paginatedLongText+"\n", 3) + "末尾"
	for _, scroll := range []bool{false, true} {
		var s PaginatedText
		opts := paginationOptions(160, 140)
		opts.Scroll = scroll
		s.Layout(text, "章名", opts)
		for _, l := range s.Lines {
			if l.Para < 0 {
				continue
			}
			p := s.Paras[l.Para]
			ln := p.Shape.Lines[l.Line]
			scrollY := float32(0)
			if scroll {
				scrollY = l.RollY
			}
			y := s.LineY(l, scrollY) + p.Shape.LineHeight/2
			for off := ln.Start; off < ln.End; off++ {
				x0, x1 := p.Shape.X(l.Line, off), p.Shape.X(l.Line, off+1)
				if x1 <= x0 {
					continue
				}
				para, offset, ok := s.Hit((x0+x1)/2, y, l.Page, scrollY)
				if !ok || para != l.Para || offset != p.Start+off+1 {
					t.Fatalf("scroll %v: hit %d/%d/%v for %+v rune %d", scroll, para, offset, ok, l, off)
				}
			}
			for x, want := range map[float32]int{0: p.Start + ln.Start, s.TextWidth: p.Start + ln.End} {
				if _, got, ok := s.Hit(x, y, l.Page, scrollY); !ok || got != want {
					t.Fatalf("blank space: %d/%v, want %d", got, ok, want)
				}
			}
		}
		if _, _, ok := s.Hit(0, opts.TitleTop+1, 0, 0); ok {
			t.Fatal("title hit as prose")
		}
		if scroll {
			if s.Pages != max(1, int(math.Ceil(float64(s.Height/s.PageHeight)))) {
				t.Fatal("scroll page estimate")
			}
			for _, l := range s.Lines {
				if s.LineY(l, 10) != l.RollY-10 {
					t.Fatal("scroll line position")
				}
			}
		}
	}
}

func TestPaginatedLongParagraphs(t *testing.T) {
	var s PaginatedText
	opts := paginationOptions(160, 140)
	text := strings.Repeat(paginatedLongText, 2) + "\n短段\n" + strings.Repeat(paginatedLongText, 2)
	s.Layout(text, "章名", opts)
	if s.LongCount != 2 || len(s.LongPages) != 2 || s.LongPages[0] >= s.LongPages[1] {
		t.Fatal("long paragraph pages", s.LongCount, s.LongPages)
	}
	opts.MarkLong = false
	s.Layout(text, "章名", opts)
	if s.LongCount != 0 || len(s.LongPages) != 0 {
		t.Fatal("disabled long paragraph markers")
	}
}
