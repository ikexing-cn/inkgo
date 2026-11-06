package textdiff

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// Paragraphs cuts the pieces of a text at the ends of its lines, keeping
// the empty ones.
func TestParagraphsOfADiff(t *testing.T) {
	got := Paragraphs([]Piece{{Same, "一\n\n二"}, {Removed, "旧"}, {Added, "新\n三"}, {Same, "\n"}})
	want := [][]Piece{{{Same, "一"}}, nil, {{Same, "二"}, {Removed, "旧"}, {Added, "新"}}, {{Added, "三"}}, nil}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%v", got)
	}
}

func TestCompare(t *testing.T) {
	for _, tc := range []struct {
		name, old, new string
		want           []Piece
	}{
		{"empty", "", "", nil},
		{"added text", "", "雨", []Piece{{Added, "雨"}}},
		{"removed text", "雨", "", []Piece{{Removed, "雨"}}},
		{"trailing newline", "雨\n", "雨\n", []Piece{{Same, "雨\n"}}},
		{"changed before trailing empty paragraphs", "旧\n\n", "新\n\n", []Piece{{Removed, "旧"}, {Added, "新"}, {Same, "\n\n"}}},
		{"paired paragraphs and leftover", "甲旧乙\n丙旧丁\n删\n尾", "甲新乙\n丙新丁\n尾", []Piece{{Same, "甲"}, {Removed, "旧"}, {Added, "新"}, {Same, "乙\n丙"}, {Removed, "旧"}, {Added, "新"}, {Same, "丁\n"}, {Removed, "删\n"}, {Same, "尾"}}},
		{"same", "雨\n\n灯", "雨\n\n灯", []Piece{{Same, "雨\n\n灯"}}},
		{"word inside paragraph", "甲旧乙", "甲新乙", []Piece{{Same, "甲"}, {Removed, "旧"}, {Added, "新"}, {Same, "乙"}}},
		{"added paragraph", "首\n尾", "首\n添\n尾", []Piece{{Same, "首\n"}, {Added, "添\n"}, {Same, "尾"}}},
		{"removed paragraph", "首\n删\n尾", "首\n尾", []Piece{{Same, "首\n"}, {Removed, "删\n"}, {Same, "尾"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := Compare(tc.old, tc.new)
			if !slices.Equal(got, tc.want) {
				t.Fatalf("got %#v, want %#v", got, tc.want)
			}
			var old, new strings.Builder
			for _, p := range got {
				if p.Kind != Added {
					old.WriteString(p.Text)
				}
				if p.Kind != Removed {
					new.WriteString(p.Text)
				}
			}
			if old.String() != tc.old || new.String() != tc.new {
				t.Fatalf("reconstructed %q / %q", old.String(), new.String())
			}
		})
	}
}

func TestLargeParagraphWithoutSharedCharacters(t *testing.T) {
	old, new := strings.Repeat("旧", 2001), strings.Repeat("新", 2001)
	if got, want := Compare(old, new), []Piece{{Removed, old}, {Added, new}}; !slices.Equal(got, want) {
		t.Fatal("paragraphs without shared characters must be replaced whole")
	}
}

// Both ends are rewritten and the product of the lengths exceeds the old
// four-million-cell cutoff. The shared passage must still be found inside.
func TestLargeParagraphRewriteKeepsSharedPassage(t *testing.T) {
	oldPart, newPart := strings.Repeat("旧", 800), strings.Repeat("新", 800)
	shared := strings.Repeat("共同的往事", 180)
	old, new := oldPart+shared+oldPart, newPart+shared+newPart
	want := []Piece{{Removed, oldPart}, {Added, newPart}, {Same, shared}, {Removed, oldPart}, {Added, newPart}}
	if got := Compare(old, new); !slices.Equal(got, want) {
		t.Fatalf("rewrite did not preserve the shared passage: got %d pieces, want %d", len(got), len(want))
	}
}

func TestLongParagraphSmallChange(t *testing.T) {
	prefix, suffix := strings.Repeat("山间的风。", 300), strings.Repeat("窗外的雨。", 300)
	want := []Piece{{Same, prefix}, {Removed, "旧灯"}, {Added, "新月"}, {Same, suffix}}
	if got := Compare(prefix+"旧灯"+suffix, prefix+"新月"+suffix); !slices.Equal(got, want) {
		t.Fatalf("small change in long paragraph: got %#v", got)
	}
}

func TestThousandsOfParagraphs(t *testing.T) {
	for _, shared := range []bool{true, false} {
		t.Run(fmt.Sprintf("shared=%t", shared), func(t *testing.T) {
			var old, new strings.Builder
			for i := range 4000 {
				if i > 0 {
					old.WriteByte('\n')
					new.WriteByte('\n')
				}
				fmt.Fprintf(&old, "旧稿第%d段：山间的风。", i)
				if shared && i%10 != 0 {
					fmt.Fprintf(&new, "旧稿第%d段：山间的风。", i)
				} else {
					fmt.Fprintf(&new, "新稿第%d段：窗外的雨。", i)
				}
			}
			got := Compare(old.String(), new.String())
			var reconstructedOld, reconstructedNew strings.Builder
			for i, p := range got {
				if p.Text == "" || i > 0 && got[i-1].Kind == p.Kind {
					t.Fatalf("empty or unmerged piece at %d", i)
				}
				if p.Kind != Added {
					reconstructedOld.WriteString(p.Text)
				}
				if p.Kind != Removed {
					reconstructedNew.WriteString(p.Text)
				}
			}
			if reconstructedOld.String() != old.String() || reconstructedNew.String() != new.String() {
				t.Fatal("diff did not preserve both manuscripts")
			}
		})
	}
}

func TestSemanticCleanup(t *testing.T) {
	// One coincidental matching character should not split a rewritten phrase.
	want := []Piece{{Removed, "甲旧乙旧丙"}, {Added, "丁新乙新戊"}}
	if got := Compare("甲旧乙旧丙", "丁新乙新戊"); !slices.Equal(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
