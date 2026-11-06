package textclip

import "testing"

func TestMeasures(t *testing.T) {
	for _, c := range []struct {
		text                    string
		n                       int
		first, last, clip, tail string
	}{
		{"甲乙丙丁戊", 4, "甲乙丙丁", "乙丙丁戊", "甲乙丙丁…", "…乙丙丁戊"},
		{"abcde", 4, "abcd", "bcde", "abcd…", "…bcde"},
		{"甲a\n乙b", 4, "甲a\n乙", "a\n乙b", "甲a\n乙…", "…a\n乙b"},
		{"甲ab乙", 4, "甲ab乙", "甲ab乙", "甲ab乙", "甲ab乙"},
		{"短", 4, "短", "短", "短", "短"},
		{"字", 0, "", "", "…", "…"},
		{"", 0, "", "", "", ""},
	} {
		if got := First(c.text, c.n); got != c.first {
			t.Errorf("First(%q, %d) = %q, want %q", c.text, c.n, got, c.first)
		}
		if got := Last(c.text, c.n); got != c.last {
			t.Errorf("Last(%q, %d) = %q, want %q", c.text, c.n, got, c.last)
		}
		if got := Clip(c.text, c.n); got != c.clip {
			t.Errorf("Clip(%q, %d) = %q, want %q", c.text, c.n, got, c.clip)
		}
		if got := Tail(c.text, c.n); got != c.tail {
			t.Errorf("Tail(%q, %d) = %q, want %q", c.text, c.n, got, c.tail)
		}
	}
}

func TestGraphemeClipping(t *testing.T) {
	for _, g := range []string{"👨‍👩‍👧", "🇨🇳", "👍🏽", "1️⃣", "e\u0301"} {
		s := "甲" + g + "乙"
		if got := First(s, 2); got != "甲"+g {
			t.Errorf("First: %q", got)
		}
		if got := Last(s, 2); got != g+"乙" {
			t.Errorf("Last: %q", got)
		}
		if got := Clip(s, 2); got != "甲"+g+"…" {
			t.Errorf("Clip: %q", got)
		}
		if got := Tail(s, 2); got != "…"+g+"乙" {
			t.Errorf("Tail: %q", got)
		}
		if Clip(g, 1) != g || Tail(g, 1) != g {
			t.Errorf("unnecessary ellipsis: %q", g)
		}
	}
}
