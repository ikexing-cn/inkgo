package editor

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestMarksFollowEdits(t *testing.T) {
	d := NewDoc("甲乙丙丁戊")
	id := d.AddMark(MarkForeshadow, 1, 3, "") // 乙丙
	d.replace(0, 0, []rune("前"))              // before the mark
	m := d.Marks()[0]
	if m.Start != 2 || m.End != 4 || d.Slice(m.Start, m.End) != "乙丙" {
		t.Fatalf("after insert before: %+v %q", m, d.String())
	}
	d.replace(2, 2, []rune("X")) // at the mark's start: stays outside
	d.replace(5, 5, []rune("Y")) // at the mark's end: stays outside
	m = d.Marks()[0]
	if d.Slice(m.Start, m.End) != "乙丙" {
		t.Fatalf("edge inserts grew the mark: %q in %q", d.Slice(m.Start, m.End), d.String())
	}
	d.replace(m.Start, m.End, nil)
	m = d.Marks()[0]
	if !m.Orphaned || m.ID != id {
		t.Fatalf("deleted text should orphan the mark: %+v", m)
	}
}

func TestNormalize(t *testing.T) {
	if got := normalize("a\r\nb\rc\td\x01"); got != "a\nb\nc d" {
		t.Fatalf("normalize = %q", got)
	}
}

func TestBreakLinesKinsoku(t *testing.T) {
	r := []rune("一二三四，五六")
	xs := make([]float32, len(r)+1)
	for i := range xs {
		xs[i] = float32(i * 10)
	}
	// Four characters fit; "，" must not start the second line, so "四"
	// goes down with it.
	got := breakLines(r, xs, 40, 0)
	if !slices.Equal(got, []int{0, 3}) {
		t.Fatalf("breakLines = %v, want [0 3]", got)
	}
}

func TestBreakLinesLatinWords(t *testing.T) {
	r := []rune("中文 hello world")
	xs := make([]float32, len(r)+1)
	for i := range xs {
		xs[i] = float32(i * 10)
	}
	for _, s := range breakLines(r, xs, 60, 0)[1:] {
		if isWordRune(r[s-1]) && isWordRune(r[s]) {
			t.Fatalf("broke inside a word at %d: %v", s, breakLines(r, xs, 60, 0))
		}
	}
}

func TestCandidates(t *testing.T) {
	ents := []Entity{{Name: "林晚"}, {Name: "林晚舟"}, {Name: "沈砚"}}
	c, k := candidates([]rune("他看见林"), ents)
	if k != 1 || len(c) != 2 {
		t.Fatalf("林: %v %d", c, k)
	}
	c, k = candidates([]rune("他看见林晚"), ents)
	if k != 2 || len(c) != 1 || c[0].Name != "林晚舟" {
		t.Fatalf("林晚: %v %d", c, k)
	}
	if c, _ = candidates([]rune("他说"), ents); c != nil {
		t.Fatalf("no name typed, got %v", c)
	}
}

func TestUndoRedoTypingMerges(t *testing.T) {
	e := New(NewDoc(""), Options{})
	e.Insert("今")
	e.Insert("天")
	e.Backspace()
	e.Undo()
	if e.Doc.String() != "今天" {
		t.Fatalf("undo backspace: %q", e.Doc.String())
	}
	e.Undo()
	if e.Doc.String() != "" {
		t.Fatalf("typing should undo together: %q", e.Doc.String())
	}
	e.Redo()
	if e.Doc.String() != "今天" || e.Caret() != 2 {
		t.Fatalf("redo: %q caret %d", e.Doc.String(), e.Caret())
	}
}

// Replacing every match is one step to undo, and marks between matches
// keep to their words.
func TestReplaceAllUndoesAsOne(t *testing.T) {
	e := New(NewDoc("林晚说，林晚走了，林晚"), Options{})
	id := e.Doc.AddMark(MarkForeshadow, 4, 6, "")
	if n := e.ReplaceAll("林晚", "沈砚舟"); n != 3 {
		t.Fatalf("replaced %d", n)
	}
	if got := e.Doc.String(); got != "沈砚舟说，沈砚舟走了，沈砚舟" {
		t.Fatalf("doc %q", got)
	}
	for _, m := range e.Doc.Marks() {
		if m.ID == id && (m.Orphaned || e.Doc.Slice(m.Start, m.End) != "沈砚舟") {
			t.Fatalf("mark %+v covers %q", m, e.Doc.Slice(m.Start, m.End))
		}
	}
	e.Undo()
	if got := e.Doc.String(); got != "林晚说，林晚走了，林晚" {
		t.Fatalf("undo %q", got)
	}
	e.Redo()
	if got := e.Doc.String(); got != "沈砚舟说，沈砚舟走了，沈砚舟" {
		t.Fatalf("redo %q", got)
	}
	e.Undo()
	e.Insert("雨")
	e.Undo()
	if got := e.Doc.String(); got != "林晚说，林晚走了，林晚" {
		t.Fatalf("a step after a replacement undid with it: %q", got)
	}
}

// Prose keeps indents out of the text: Tab adds none, and a paste loses
// its own.
func TestProseKeepsIndentsOut(t *testing.T) {
	e := New(NewDoc("雨"), Options{Prose: true})
	tt := newTester(e)
	tt.Key(0, ui.KeyTab)
	if e.Doc.String() != "雨" {
		t.Fatalf("Tab indented: %q", e.Doc.String())
	}
	if got := unindent("　　第一段\r\n  第二段"); got != "第一段\n第二段" {
		t.Fatalf("unindent %q", got)
	}
}

// Replacing every match in a document moves its marks with the text.
func TestDocReplaceAllMovesMarks(t *testing.T) {
	d := NewDoc("林晚说，林晚走了。")
	id := d.AddMark(MarkForeshadow, 6, 8, "") // 走了
	if n := d.ReplaceAll("林晚", "苏"); n != 2 {
		t.Fatal(n)
	}
	if d.String() != "苏说，苏走了。" {
		t.Fatal(d.String())
	}
	for _, m := range d.Marks() {
		if m.ID == id && d.Slice(m.Start, m.End) != "走了" {
			t.Fatal("the mark did not move", m)
		}
	}
}
