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
