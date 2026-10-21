package workspace

import (
	"strings"
	"testing"
)

func TestSettingMentions(t *testing.T) {
	s := Setting{Entries: []Entry{
		{ID: "named", Names: []string{"林晚", "晚晚"}, Text: "不用于名字"},
		{ID: "sentence", Text: "  这是第一句。不要后一句。"},
		{ID: "line", Text: "第一行\n第二行。"},
		{ID: "long", Text: strings.Repeat("甲", 18)},
		{ID: "empty"},
		{ID: "punct", Names: []string{"[]*!"}},
	}}
	for _, tc := range []struct {
		text  string
		names []string
	}{
		{"", nil}, {"没有引用", nil}, {"[0] [7] [99999999999999999999999] [-1] [x]", nil},
		{"前文 [1]，再说 [2]。", []string{"林晚", "这是第一句"}},
		{"[3][4][5][6]", []string{"第一行", strings.Repeat("甲", 16) + "…", "", "[]*!"}},
		{"[001]和[1]", []string{"林晚", "林晚"}},
		{"[0] [1] [7]", []string{"林晚"}},
	} {
		got := s.Mentions(tc.text)
		var names []string
		end := 0
		for _, m := range got {
			if m.Start != end || m.End < m.Start || m.End > len(tc.text) {
				t.Fatal("offsets", tc, m)
			}
			raw := tc.text[m.Start:m.End]
			if m.Entry != nil {
				names = append(names, m.Text)
				if !strings.HasPrefix(raw, "[") || !strings.HasSuffix(raw, "]") {
					t.Fatal(m)
				}
			} else if m.Text != raw {
				t.Fatal(m, raw)
			}
			end = m.End
		}
		if end != len(tc.text) || strings.Join(names, "|") != strings.Join(tc.names, "|") || len(names) != len(tc.names) {
			t.Fatal(tc, got, names)
		}
	}
	got := s.Mentions("[1] [1]")
	got[0].Entry.Text = "changed"
	if s.Entries[0].Text != "不用于名字" || got[2].Entry.Text != "不用于名字" {
		t.Fatal("mentions shared their entry value", got)
	}
	if got := (Setting{}).Mentions("[1]"); len(got) != 1 || got[0].Entry != nil || got[0].Text != "[1]" {
		t.Fatal(got)
	}
}
