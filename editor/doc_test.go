package editor

import (
	"slices"
	"testing"
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
	got := breakLines(r, xs, 40)
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
	for _, s := range breakLines(r, xs, 60)[1:] {
		if isWordRune(r[s-1]) && isWordRune(r[s]) {
			t.Fatalf("broke inside a word at %d: %v", s, breakLines(r, xs, 60))
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
