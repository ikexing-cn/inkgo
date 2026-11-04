package workspace

import (
	"strings"
	"testing"
)

func TestTalkTitle(t *testing.T) {
	for _, tc := range []struct {
		name          string
		events        []Event
		chapter, want string
	}{
		{"empty", nil, "", "新对话"},
		{"no user", []Event{{Role: RoleAssistant, Text: "回答"}}, "", "新对话"},
		{"first user", []Event{{Role: RoleAssistant, Text: "前言"}, {Role: RoleUser, Text: "雨城的故事\n第二行"}, {Role: RoleUser, Text: "后来"}}, "", "雨城的故事"},
		{"fourteen runes", []Event{{Role: RoleUser, Text: strings.Repeat("雨", 15)}}, "", strings.Repeat("雨", 14) + "…"},
		{"exact limit", []Event{{Role: RoleUser, Text: strings.Repeat("雨", 14)}}, "", strings.Repeat("雨", 14)},
		{"draft", []Event{{Role: RoleUser, Origin: AIDraft, Chapter: "chapter"}}, "第十二章 雨", "起草：第十二章 雨"},
		{"deleted draft", []Event{{Role: RoleUser, Origin: AIDraft, Chapter: "gone"}}, "", "起草：已删除的章"},
		{"rewrite", []Event{{Role: RoleUser, Origin: AIRewrite, Quotes: []Quote{{Text: strings.Repeat("雨", 9) + "\n不显示"}}}}, "", "改写：" + strings.Repeat("雨", 8) + "…"},
		{"rewrite without quote", []Event{{Role: RoleUser, Origin: AIRewrite, Text: "我的要求\n另一行"}}, "", "我的要求"},
		{"rewrite first quote", []Event{{Role: RoleUser, Origin: AIRewrite, Quotes: []Quote{{Text: "第一行\n第二行"}, {Text: "另一段"}}}}, "", "改写：第一行"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			got := TalkTitle(tc.events, func(id string) string {
				calls++
				if tc.name == "draft" && id != "chapter" {
					t.Fatal(id)
				}
				return tc.chapter
			})
			text := got.Text
			switch got.Kind {
			case AIDraft:
				if text == "" {
					text = "已删除的章"
				}
				text = "起草：" + text
			case AIRewrite:
				text = "改写：" + text
			case "":
				text = "新对话"
			}
			if text != tc.want {
				t.Fatalf("%q, want %q", got, tc.want)
			}
			if tc.name == "empty" || tc.name == "no user" {
				if calls != 0 {
					t.Fatal("resolved without user")
				}
			} else if calls != 1 {
				t.Fatal("resolved more than first user", calls)
			}
		})
	}
}
