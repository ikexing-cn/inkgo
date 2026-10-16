package desk

import (
	"strings"
	"testing"

	"writer/workspace"
)

// Under each reply, the tokens the requests of its turn took and gave as
// the provider counted: its own and those that looked something up.
// What a reply was sent is of the request that replied, saying how many
// the turn took: each part's tokens as estimated, what the provider
// counted, and the setting, whole with a tick each, or as its index. A
// reply from before each request was kept apart counts one; one from
// before parts were kept lists what it left out.
func TestWhatWasSent(t *testing.T) {
	a, tt, e := fixture(t)
	u, err := a.Store.Append(a.session, workspace.Event{Role: workspace.RoleUser, Text: "说说"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.Store.Append(a.session, workspace.Event{Role: workspace.RoleCall, ReplyTo: u.ID,
		Calls: []workspace.Call{{ID: "c1", Name: "search_book", Input: []byte(`{"query":"雨"}`), Label: "搜了全书「雨」"}}, Usage: &workspace.Usage{Input: 9200, Output: 40}}); err != nil {
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
	if _, err = a.Store.Append(a.session, workspace.Event{Role: workspace.RoleCall, ReplyTo: u2.ID,
		Calls: []workspace.Call{{ID: "c2", Name: "search_book", Input: []byte(`{"query":"灯"}`), Label: "搜了全书「灯」"}}}); err != nil {
		t.Fatal(err)
	}
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
	footers, asked := 0, 0
	for _, s := range tt.Texts() {
		if strings.HasPrefix(s, "今天 ") && (strings.HasSuffix(s, " · 19.4K token") || strings.HasSuffix(s, " · 920 token")) {
			footers++
		}
		if len(s) == len("今天 15:04") && strings.HasPrefix(s, "今天 ") {
			asked++
		}
	}
	// Each reply says its day as well as its time, as each question does:
	// three questions and the reply that counted nothing.
	if footers != 2 || strings.Count(strings.Join(tt.Texts(), "\n"), " token") != 2 || asked != 4 {
		t.Fatal("the day and tokens of each reply", tt.Texts())
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
	open(whole.ID, "本次发送约 9.6K token", "这一轮共 2 次请求，这是最后一次。", "规则 1.1K · 设定 1 条 1K · 之前的对话 7.5K · 本次发言 40",
		"实际用量：输入 9.9K，其中缓存命中 6K；输出 268", "所在章：第一章", "附带全部 1 条设定", "主角怕黑")
	open(index.ID, "本次发送约 1.1K token", "规则 1.1K · 设定目录 30", "实际用量：输入 900；输出 20", "设定较多，本次只附目录", "人物：主角怕黑")
	if tt.HasText("附带全部 1 条设定") || tt.HasText("这一轮共") {
		t.Fatal("two replies show what they were sent, or one from before counts its requests", tt.Texts())
	}
	open(old.ID, "回复时附带 1 条设定，因上下文上限省略 1 条", "正文：主角会飞 · 超出上下文预算")
	if tt.HasText("本次发送约") || tt.HasText("实际用量") {
		t.Fatal("a reply that recorded no parts shows some")
	}
}
