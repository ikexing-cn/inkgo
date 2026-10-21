package textdiff

import (
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

func TestLargeParagraphReplacement(t *testing.T) {
	old, new := strings.Repeat("旧", 2001), strings.Repeat("新", 2001)
	if got, want := Compare(old, new), []Piece{{Removed, old}, {Added, new}}; !slices.Equal(got, want) {
		t.Fatal("large paragraphs must be replaced whole")
	}
}
