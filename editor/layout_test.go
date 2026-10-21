package editor

import (
	"slices"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestParagraphShapesSurviveLineBreakEdits(t *testing.T) {
	paragraphs := []string{"第一段写雨。", "第二段写风和树。", "第三段写林晚走过长街。", "第四段写沈砚推门。", "第五段写灯火。"}
	for edited := range paragraphs {
		t.Run(paragraphs[edited], func(t *testing.T) {
			e := New(NewDoc(strings.Join(paragraphs, "\n")), Options{})
			e.width = 300
			e.ensureLayout()
			before := slices.Clone(e.shapes)
			at := len([]rune(paragraphs[edited])) / 2
			for _, text := range paragraphs[:edited] {
				at += len([]rune(text)) + 1
			}
			e.Replace(at, at, "\n")
			e.ensureLayout()
			if len(e.shapes) != len(before)+1 {
				t.Fatal("line break did not split the paragraph")
			}
			for i, para := range before {
				if i == edited {
					continue
				}
				j := i
				if i > edited {
					j++
				}
				if e.shapes[j].sh != para.sh {
					t.Fatalf("insert: paragraph %d was reshaped", i)
				}
			}
			e.Replace(at, at+1, "")
			e.ensureLayout()
			if len(e.shapes) != len(before) {
				t.Fatal("deleting the line break did not join the paragraph")
			}
			for i, para := range before {
				if i != edited && e.shapes[i].sh != para.sh {
					t.Fatalf("delete: paragraph %d was reshaped", i)
				}
			}
		})
	}
}

func TestCaretChangesKeepLayout(t *testing.T) {
	e := New(NewDoc("林晚推开门。\n雨还在下。"), Options{})
	e.width = 300
	e.ensureLayout()
	lines := &e.lay.lines[0]
	if e.lay.kinds != nil || &e.lay.disp[0] != &e.Doc.text[0] {
		t.Fatal("plain display copied the document")
	}
	for i := 0; i <= e.Doc.Len(); i++ {
		e.MoveTo(i, false)
		e.ensureLayout()
		if &e.lay.lines[0] != lines {
			t.Fatal("plain caret movement rebuilt layout")
		}
	}
	e.SetProposals([]Proposal{{ID: "p", A: 0, B: 2, Text: "沈砚"}}, 20)
	e.ensureLayout()
	lines = &e.lay.lines[0]
	e.MoveTo(2, false)
	e.ensureLayout()
	if &e.lay.lines[0] != lines {
		t.Fatal("proposal-only caret movement rebuilt layout")
	}
	e.preedit = "雨"
	e.ensureLayout()
	if got := string(e.lay.disp); got != "林晚雨沈砚"+string(gapRune)+"推开门。\n雨还在下。" {
		t.Fatal(got)
	}
	lines = &e.lay.lines[0]
	e.MoveTo(3, false)
	e.ensureLayout()
	if &e.lay.lines[0] == lines {
		t.Fatal("composition caret movement did not rebuild layout")
	}
}

func TestGlyphSlicesMatchScan(t *testing.T) {
	f := ui.Font{Family: defaultFontFamily, Size: 17}
	for _, text := range []string{"林晚说：“Hello, world!”雨还在下。", "office café e\u0301，中文（标点）…", "雨" + string(gapRune) + "A，夜。", "中文 hello שלום，مرحبا world。", ""} {
		sh := shapeText(text, f, 32)
		n := len([]rune(text))
		for a := 0; a <= n; a++ {
			for b := a; b <= n; b++ {
				var scanned []ui.Glyph
				for _, g := range sh.glyphs {
					if g.Cluster >= a && g.Cluster < b {
						scanned = append(scanned, g)
					}
				}
				if !slices.Equal(sh.run(a, b), scanned) {
					t.Fatalf("%q [%d,%d): glyph slice differs from scan", text, a, b)
				}
			}
		}
	}
}

func TestLineSearchMatchesScan(t *testing.T) {
	for _, text := range []string{"", "\n\n", "雨下了整整一夜，林晚站在檐下。\n\n沈砚推开门。\n"} {
		e := New(NewDoc(text), Options{Prose: true})
		e.width = 170
		e.ensureLayout()
		l := &e.lay
		for d := -1; d <= len(l.disp)+1; d++ {
			want := max(len(l.lines)-1, 0)
			for i, ln := range l.lines {
				if d >= ln.start && (d < ln.end || d == ln.end && ln.last) {
					want = i
					break
				}
			}
			if l.lineOf(d) != want {
				t.Fatalf("%q at %d: line %d, want %d", text, d, l.lineOf(d), want)
			}
		}
		for y := float32(-1); y < l.height+1; y++ {
			want := 0
			for i, ln := range l.lines {
				if ln.y <= y {
					want = i
				}
			}
			if l.lineAt(y) != want {
				t.Fatalf("%q at y=%v: line %d, want %d", text, y, l.lineAt(y), want)
			}
		}
		for a := 0; a <= len(l.disp); a++ {
			for b := a; b <= len(l.disp); b++ {
				var want, got [][3]float32
				for i := range l.lines {
					ln := &l.lines[i]
					s, t := max(a, ln.start), min(b, ln.end)
					brk := ln.last && b > ln.end && a <= ln.end
					if s > t || s == t && !brk {
						continue
					}
					x0, x1 := l.x(ln, s), l.x(ln, t)
					if brk {
						x1 += 6
					}
					want = append(want, [3]float32{x0, x1, ln.y})
				}
				l.ranges(a, b, func(x0, x1, y float32) { got = append(got, [3]float32{x0, x1, y}) })
				if !slices.Equal(got, want) {
					t.Fatalf("%q [%d,%d): ranges %v, want %v", text, a, b, got, want)
				}
			}
		}
	}
}
