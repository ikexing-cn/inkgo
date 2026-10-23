package kit

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestMarked(t *testing.T) {
	color := ui.Hex("#123456")
	for _, c := range []struct {
		text, find string
		want       []ui.Span
	}{
		{"甲乙甲", "甲", []ui.Span{{Text: "甲", Weight: 600, Color: color}, {Text: "乙"}, {Text: "甲", Weight: 600, Color: color}}},
		{"甲乙", "丙", []ui.Span{{Text: "甲乙"}}},
		{"甲乙", "", []ui.Span{{Text: "甲乙"}}},
		{"", "甲", nil},
	} {
		if got := Marked(c.text, c.find, color); !slices.Equal(got, c.want) {
			t.Errorf("Marked(%q, %q) = %+v, want %+v", c.text, c.find, got, c.want)
		}
	}
}
