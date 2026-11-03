package kit

import (
	"testing"

	"writer/workspace"
)

func TestTextLabels(t *testing.T) {
	for _, c := range []struct{ name, got, want string }{
		{"trim", Trim(" \n甲乙\t "), "甲乙"},
		{"heading", Heading("第1章： 雨城 "), "雨城"},
		{"heading without a name", Heading("第1章： "), "第1章： "},
		{"heading without a number", Heading("雨城"), "雨城"},
		{"grouped zero", Grouped(0), "0"},
		{"grouped", Grouped(1234567), "1,234,567"},
		{"grouped negative", Grouped(-1234), "-1,234"},
		{"signed zero", Signed(0), "+0"},
		{"signed positive", Signed(1234), "+1,234"},
		{"signed negative", Signed(-1234), "-1,234"},
		{"empty count", Count(0), ""},
		{"count", Count(12), "12"},
	} {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", c.name, c.got, c.want)
		}
	}
}

func TestFromLabel(t *testing.T) {
	st, err := workspace.Create(t.TempDir(), "雨城", "网文")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	cases := map[string]string{"missing": "所在章节已删除"}
	for _, c := range []struct {
		title, want string
		folder      bool
	}{
		{"第87章：停电", "第87章起", false},
		{"雨城", "雨城起", false},
		{"第1卷：雨城", "第1卷：雨城起", true},
	} {
		id, err := st.NewChapter("", c.title, c.folder)
		if err != nil {
			t.Fatal(err)
		}
		cases[id] = c.want
	}
	tree, err := st.Chapters()
	if err != nil {
		t.Fatal(err)
	}
	for id, want := range cases {
		if got := FromLabel(tree, id); got != want {
			t.Errorf("FromLabel(%q) = %q, want %q", id, got, want)
		}
	}
}

func TestClipMeasure(t *testing.T) {
	for _, c := range []struct{ text, want string }{
		{"甲乙丙丁戊", "甲乙丙丁…"},
		{"abcde", "abcd…"},
		{"甲a\n乙bcdef", "甲a"},
		{"甲ab乙", "甲ab乙"},
	} {
		if got := Clip(c.text, 4); got != c.want {
			t.Errorf("clip(%q, 4) = %q, want %q", c.text, got, c.want)
		}
	}
}

func TestWords(t *testing.T) {
	for n, want := range map[int]string{0: "0 字", 54: "54 字", 2846: "2,846 字", 9999: "9,999 字", 10000: "1.0 万字", 312345: "31.2 万字", 1200000: "120.0 万字"} {
		if got := Words(n); got != want {
			t.Errorf("words(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestNearFindMeasure(t *testing.T) {
	for _, c := range []struct{ text, want string }{
		{"甲乙abcdef关键后文", "甲乙abcdef关键后文"},
		{"甲乙abcdefg关键后文", "…bcdefg关键后文"},
		{"甲\n乙abcdefg关键后文", "…bcdefg关键后文"},
	} {
		if got := NearFind(c.text, "关键"); got != c.want {
			t.Errorf("nearFind(%q) = %q, want %q", c.text, got, c.want)
		}
	}
}
