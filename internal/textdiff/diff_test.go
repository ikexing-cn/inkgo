package textdiff

import (
	"reflect"
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
