package desk

import (
	"testing"
	"time"

	"writer/workspace"
)

// The AI's settings show what the work spent on each feature, read as
// they open: the requests, the tokens, the share read from the cache and
// the cost estimated, or that a model has no price. There is nothing to
// clear it with. 查看全部记录 lists each request, the newest first, by
// feature and where it was for; a line opens on what it sent and what the
// provider counted. With the AI off, none of it shows.
func TestTheWorkShowsWhatItSpent(t *testing.T) {
	a, tt, _ := fixture(t)
	a.openPrefs("AI")
	tt.Frame()
	if !tt.HasText("本作品用量") || tt.HasText("按内置价格估算") || !tt.HasText("尚未使用 AI。") {
		t.Fatal("nothing spent yet", tt.Texts())
	}
	chapter, err := a.Store.NewChapter("", "第一章", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.Store.Append(a.session, workspace.Event{Role: workspace.RoleUser, Text: "灯为什么会灭"}); err != nil {
		t.Fatal(err)
	}
	a.setError(a.refresh())
	a.setError(a.loadTree())
	at := time.Now().Add(-time.Hour)
	for i, r := range []workspace.Record{
		{Feature: workspace.FeatureTalk, Talk: a.session, Model: "anthropic/claude-opus-5-5", Usage: workspace.Usage{Input: 1_682_400, CacheRead: 1_211_300, Output: 59_200}, Cost: 4.444, Currency: "$", Outcome: workspace.OutcomeDone,
			Parts: []workspace.Part{{Name: workspace.PartRules, Tokens: 1100}, {Name: workspace.PartTalk, Tokens: 9600}}},
		{Feature: workspace.FeatureTalk, Talk: a.session, Model: "deepseek/deepseek-flash", Usage: workspace.Usage{Input: 142_000, CacheRead: 85_200, Output: 4100}, Cost: 0.021, Currency: "$", Outcome: workspace.OutcomeDone},
		{Feature: workspace.FeatureContinue, Chapter: chapter, Model: "local/qwen3", Effort: "none", Usage: workspace.Usage{Input: 900, Output: 30}, Outcome: workspace.OutcomeCancelled},
		{Feature: workspace.FeatureCheck, Chapter: "gone", Model: "deepseek/deepseek-flash", Effort: "high", Usage: workspace.Usage{Input: 7000}, Cost: 0.002, Currency: "$", Outcome: workspace.OutcomeFailed, Error: "无法连接模型服务",
			Parts: []workspace.Part{{Name: workspace.PartRules, Tokens: 900}, {Name: workspace.PartProse, Tokens: 4400}}},
	} {
		r.At = at.Add(time.Duration(i) * time.Minute)
		if err := a.Store.Log(r); err != nil {
			t.Fatal(err)
		}
	}
	tt.Frame()
	if !tt.HasText("尚未使用 AI。") {
		t.Fatal("read again each frame")
	}
	a.bk.prefs = false
	a.openPrefs("AI")
	tt.Frame()
	for _, s := range []string{
		"讨论", "2 次请求 · 输入 1.8M，缓存命中 71% · 输出 63.3K", "约 $4.46",
		"续写", "1 次请求 · 输入 900，缓存命中 0% · 输出 30", "无价格",
		"检查", "1 次请求 · 输入 7K，缓存命中 0% · 输出 0", "不到 $0.01",
		"查看全部记录（4 次请求）",
	} {
		if !tt.HasText(s) {
			t.Fatal(s, tt.Texts())
		}
	}
	if tt.HasText("尚未使用 AI。") || tt.HasText("清零") || tt.HasText("起草") {
		t.Fatal(tt.Texts())
	}

	if err := tt.Click("查看全部记录（4 次请求）"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	for _, s := range []string{"AI 记录", "只记发了哪几部分、各多大，不存内容。",
		"‹ 本作品用量", "检查 · 已删除的章节", "失败：无法连接模型服务", "续写 · 第一章", "已取消", "讨论 · 灯为什么会灭", "完成", "1.7M", "约 $4.44", "—"} {
		if !tt.HasText(s) {
			t.Fatal(s, tt.Texts())
		}
	}
	if tt.HasText("关闭 AI") || tt.HasText("本作品的模型") || tt.HasText("本作品每次向模型发的请求") {
		t.Fatal("the record shows with the rest", tt.Texts())
	}
	if texts := tt.Texts(); indexOf(texts, "检查 · 已删除的章节") > indexOf(texts, "讨论 · 灯为什么会灭") {
		t.Fatal("not the newest first", texts)
	}
	if err := tt.Click("记录-" + when(at.Add(3*time.Minute), time.Now()) + " 检查 · 已删除的章节"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	for _, s := range []string{"deepseek-flash · High", "发送：规则 900 · 本章正文 4.4K", "实际用量：输入 7K；输出 0"} {
		if !tt.HasText(s) {
			t.Fatal(s, tt.Texts())
		}
	}
	if err := tt.Click("记录-" + when(at.Add(2*time.Minute), time.Now()) + " 续写 · 第一章"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("qwen3 · 不思考") || tt.HasText("发送：规则 900 · 本章正文 4.4K") {
		t.Fatal("one line opens at a time", tt.Texts())
	}
	a.bk.logOnly = 5 // 检查
	tt.Frame()
	if !tt.HasText("检查 · 已删除的章节") || tt.HasText("续写 · 第一章") || tt.HasText("讨论 · 灯为什么会灭") {
		t.Fatal("the filter", tt.Texts())
	}
	a.bk.logOnly = 2 // 起草
	tt.Frame()
	if !tt.HasText("没有这类请求。") {
		t.Fatal(tt.Texts())
	}
	if err := tt.Click("‹ 本作品用量"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("本作品用量") || tt.HasText("AI 记录") {
		t.Fatal(tt.Texts())
	}
	a.bk.log = true
	a.turnAI(false)
	tt.Frame()
	if tt.HasText("AI 记录") || tt.HasText("本作品用量") || tt.HasText("检查 · 已删除的章节") {
		t.Fatal("the record shows with the AI off", tt.Texts())
	}
}

func indexOf(texts []string, s string) int {
	for i, t := range texts {
		if t == s {
			return i
		}
	}
	return -1
}
