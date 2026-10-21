package editor

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestMeasuredParagraphUsesProseWrapping(t *testing.T) {
	text := "她说：“雨还没停。”英文 hello world，数字 12345，和🙂都按字形的宽排。"
	for _, width := range []float32{160, 313, 390} {
		font := ui.Font{Size: 20}
		p := LayoutParagraph(text, font, width, 1.8, 40)
		e := New(NewDoc(text), Options{Font: font, LineHeight: 1.8, Prose: true})
		e.buildLayout(width + 2*padX)
		if len(p.Lines) != len(e.lay.lines) {
			t.Fatal("preview and prose break differently")
		}
		runes := []rune(text)
		for i, ln := range p.Lines {
			if ln.Start != e.lay.lines[i].start || ln.End != e.lay.lines[i].end {
				t.Fatal("different rune boundaries")
			}
			if i > 0 && noLineStart(runes[ln.Start]) {
				t.Fatal("closing punctuation at start")
			}
			if ln.End < len(runes) && noLineEnd(runes[ln.End-1]) {
				t.Fatal("opening punctuation at end")
			}
			end := ln.End
			for end > ln.Start && runes[end-1] == ' ' {
				end--
			}
			if p.X(i, end) > width {
				t.Fatalf("line %q width %v exceeds %v", string(runes[ln.Start:ln.End]), p.X(i, end), width)
			}
			for off := ln.Start; off < ln.End; off++ {
				x0, x1 := p.X(i, off), p.X(i, off+1)
				if x1 > x0 && p.HitAfter(i, (x0+x1)/2) != off+1 {
					t.Fatal("glyph click not after source rune")
				}
			}
		}
	}
	// An empty paragraph still has a line and a caret position.
	p := LayoutParagraph("", ui.Font{Size: 20}, 313, 1.8, 40)
	if len(p.Lines) != 1 || p.HitAfter(0, 100) != 0 {
		t.Fatal("empty paragraph")
	}
}

func TestScrollAnchorMatchesParagraphs(t *testing.T) {
	e := New(NewDoc(strings.Repeat("一段长短不同的正文。\n", 50)), Options{Prose: true})
	e.width, e.height = 480, 300
	e.ensureLayout()
	f, n, into := e.ScrollAnchor()
	if f != 0 || n != 0 || into != 0 {
		t.Fatal("not anchored at top", f, n, into)
	}
	e.scrollY = e.maxScroll()
	f, _, _ = e.ScrollAnchor()
	if f != 1 {
		t.Fatal("not anchored at bottom", f)
	}
	e.scrollY = e.maxScroll() / 2
	f, n, into = e.ScrollAnchor()
	if f != 0.5 || n <= 0 || n >= 50 || into < 0 || into > 1 {
		t.Fatal("invalid interior anchor", f, n, into)
	}
}
