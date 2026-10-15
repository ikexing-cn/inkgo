package desk

import (
	"strings"
	"testing"

	"writer/workspace"
)

// Under each reply, the tokens it took and gave as the provider counted.
// What a reply was sent shows each part's tokens as estimated, what the
// provider counted, and the setting: whole with a tick each, or as its
// index; a reply from before lists what it left out.
func TestWhatWasSent(t *testing.T) {
	a, tt, e := fixture(t)
	u, err := a.Store.Append(a.session, workspace.Event{Role: workspace.RoleUser, Text: "说说"})
	if err != nil {
		t.Fatal(err)
	}
	sent := &workspace.Sent{Rules: "ab12cd34", Setting: 1, Parts: []workspace.Part{
		{Name: workspace.PartRules, Tokens: 1100}, {Name: workspace.PartSetting, Tokens: 1000},
		{Name: workspace.PartTalk, Tokens: 7460}, {Name: workspace.PartInput, Tokens: 40}}}
	whole, err := a.Store.Append(a.session, workspace.Event{Role: workspace.RoleAssistant, ReplyTo: u.ID, Text: "可以。",
		Context: []workspace.ContextItem{{Text: "所在章：第一章"}, {ID: e.ID, Text: "主角怕黑"}},
		Usage:   &workspace.Usage{Input: 9900, CacheRead: 6000, Output: 268}, Sent: sent})
	if err != nil {
		t.Fatal(err)
	}
	u2, _ := a.Store.Append(a.session, workspace.Event{Role: workspace.RoleUser, Text: "再说说"})
	index, err := a.Store.Append(a.session, workspace.Event{Role: workspace.RoleAssistant, ReplyTo: u2.ID, Text: "好。",
		Context: []workspace.ContextItem{{ID: e.ID, Text: "人物：主角怕黑"}},
		Usage:   &workspace.Usage{Input: 900, Output: 20},
		Sent:    &workspace.Sent{Parts: []workspace.Part{{Name: workspace.PartRules, Tokens: 1100}, {Name: workspace.PartIndex, Tokens: 30}}}})
	if err != nil {
		t.Fatal(err)
	}
	u3, _ := a.Store.Append(a.session, workspace.Event{Role: workspace.RoleUser, Text: "旧的"})
	old, err := a.Store.Append(a.session, workspace.Event{Role: workspace.RoleAssistant, ReplyTo: u3.ID, Text: "旧的回复。",
		Context: []workspace.ContextItem{{ID: e.ID, Text: "正文：主角怕黑"}},
		Omitted: []workspace.ContextItem{{ID: "x", Text: "正文：主角会飞", Reason: "超出上下文预算"}}})
	if err != nil {
		t.Fatal(err)
	}
	a.setError(a.refresh())
	tt.Frame()
	footers := 0
	for _, s := range tt.Texts() {
		if strings.HasSuffix(s, " · 10.2K token") || strings.HasSuffix(s, " · 920 token") {
			footers++
		}
	}
	if footers != 2 || strings.Count(strings.Join(tt.Texts(), "\n"), " token") != 2 {
		t.Fatal("the tokens of each reply", tt.Texts())
	}

	open := func(id string, want ...string) {
		t.Helper()
		if err := tt.Click("上下文-" + id); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		for _, s := range want {
			if !tt.HasText(s) {
				t.Fatal(s, tt.Texts())
			}
		}
	}
	open(whole.ID, "本次发送约 9.6K token", "规则 1.1K · 设定 1 条 1K · 之前的对话 7.5K · 本次发言 40",
		"实际用量：输入 9.9K，其中缓存命中 6K；输出 268", "所在章：第一章", "附带全部 1 条设定", "主角怕黑")
	open(index.ID, "本次发送约 1.1K token", "规则 1.1K · 设定目录 30", "实际用量：输入 900；输出 20", "设定较多，本次只附目录", "人物：主角怕黑")
	if tt.HasText("附带全部 1 条设定") {
		t.Fatal("two replies show what they were sent")
	}
	open(old.ID, "回复时附带 1 条设定，因上下文上限省略 1 条", "正文：主角会飞 · 超出上下文预算")
	if tt.HasText("本次发送约") || tt.HasText("实际用量") {
		t.Fatal("a reply that recorded no parts shows some")
	}
}
