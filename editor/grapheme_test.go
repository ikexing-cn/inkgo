package editor

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestGraphemeEditing(t *testing.T) {
	for _, cluster := range []string{"👨‍👩‍👧", "🇨🇳", "👍🏽", "1️⃣", "e\u0301"} {
		t.Run(cluster, func(t *testing.T) {
			n := len([]rune(cluster))
			for offset := 0; offset < n; offset++ {
				e := New(NewDoc("前\n甲"+cluster+"乙\n后"), Options{})
				e.MoveTo(3+offset, false)
				e.deleteForward()
				if got := e.Doc.String(); got != "前\n甲乙\n后" {
					t.Fatalf("delete at %d: %q", offset, got)
				}
				e.Undo()
				if got := e.Doc.String(); got != "前\n甲"+cluster+"乙\n后" {
					t.Fatal("undo", got)
				}
				e.MoveTo(4+offset, false)
				e.backspace()
				if got := e.Doc.String(); got != "前\n甲乙\n后" {
					t.Fatalf("backspace at %d: %q", offset, got)
				}
			}
			e := New(NewDoc("甲"+cluster+"乙"), Options{})
			e.MoveTo(1, false)
			e.plainKey(false, ui.KeyRight)
			if e.Caret() != 1+n {
				t.Fatal("right", e.Caret())
			}
			e.plainKey(true, ui.KeyLeft)
			if a, b := e.Selection(); a != 1 || b != 1+n {
				t.Fatal("shift left", a, b)
			}
			e.plainKey(false, ui.KeyRight)
			if e.Caret() != 1+n {
				t.Fatal("collapse right", e.Caret())
			}
			e.plainKey(false, ui.KeyLeft)
			e.plainKey(true, ui.KeyRight)
			if a, b := e.Selection(); a != 1 || b != 1+n {
				t.Fatal("shift right", a, b)
			}
			for i := 1; i < 1+n; i++ {
				a, b := e.wordAt(i)
				if a > 1 || b < 1+n {
					t.Fatalf("wordAt(%d) splits cluster: %d:%d", i, a, b)
				}
			}
		})
	}
}

func TestGraphemeParagraphEdges(t *testing.T) {
	e := New(NewDoc("é\n🇨🇳"), Options{})
	for _, pos := range []int{0, 2, 3, 5} {
		e.MoveTo(pos, false)
		e.plainKey(false, ui.KeyRight)
		want := map[int]int{0: 2, 2: 3, 3: 5, 5: 5}[pos]
		if e.Caret() != want {
			t.Fatalf("right %d: %d", pos, e.Caret())
		}
	}
	for _, pos := range []int{0, 2, 3, 5} {
		e.MoveTo(pos, false)
		e.plainKey(false, ui.KeyLeft)
		want := map[int]int{0: 0, 2: 0, 3: 2, 5: 3}[pos]
		if e.Caret() != want {
			t.Fatalf("left %d: %d", pos, e.Caret())
		}
	}
	e.MoveTo(3, false)
	e.backspace()
	if e.Doc.String() != "é🇨🇳" {
		t.Fatal(e.Doc.String())
	}
}

func TestUnicodeLineOpportunities(t *testing.T) {
	for _, tc := range []struct {
		text  string
		width float32
		want  []int
	}{
		{"甲é乙", 20, []int{0, 1, 3}},
		{"甲👨‍👩‍👧乙", 20, []int{0, 1, 6}},
		{"甲🇨🇳乙", 20, []int{0, 1, 3}},
		{"甲乙\u00a0丙丁", 30, []int{0, 1, 4}},
		{"甲乙\u2028丙丁", 100, []int{0, 3}},
		{"甲乙（丙丁", 30, []int{0, 2}},
		{"abcdef", 20, []int{0, 2, 4}}, // emergency wrap for overwide words
	} {
		r := []rune(tc.text)
		xs := make([]float32, len(r)+1)
		for i := range xs {
			xs[i] = float32(i * 10)
		}
		if got := breakLines(r, xs, tc.width, 0); !slices.Equal(got, tc.want) {
			t.Errorf("%q: %v want %v", tc.text, got, tc.want)
		}
	}
}
