package desk

import (
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/desk/kit"
	"writer/ext/usage"
	"writer/internal/testshot"
	"writer/workspace"
)

// The next work must not inherit the previous work's records, filter,
// expanded row or virtual-list position. Leaving does not delete the log.
func TestLeavingWorkClearsSettingsAndRecords(t *testing.T) {
	for _, how := range []string{"switch", "shelf", "close"} {
		t.Run(how, func(t *testing.T) {
			a, tt, _ := fixture(t)
			oldRoot := a.Store.Root()
			for i := range 50 {
				err := a.Store.Log(workspace.Record{At: time.Now().Add(time.Duration(i) * time.Minute),
					Feature: workspace.FeatureCheck, Model: "local/old-model", Outcome: workspace.OutcomeDone})
				if err != nil {
					t.Fatal(err)
				}
			}
			a.openPrefs("AI")
			tt.Frame()
			click(t, tt, "查看全部 50 次")
			usageViews[a].Record.Only, usageViews[a].Record.Expanded = 5, 49
			tt.Frame()
			usageViews[a].Record.List.ScrollTo(35, ui.Start)
			tt.Frame()
			tt.Frame()
			if first, _ := usageViews[a].Record.List.Visible(); first == 0 {
				t.Fatal("fixture did not scroll the record list")
			}
			a.settings.askAI = true
			aiViewOf(a).ServiceForm.Key = "unsaved-test-key"
			var next *workspace.Store
			switch how {
			case "switch":
				var err error
				next, err = workspace.Create(filepath.Join(t.TempDir(), "next"), "另一本", "网文", "zh-Hans")
				if err != nil {
					t.Fatal(err)
				}
				if err := next.Log(workspace.Record{At: time.Now(), Feature: workspace.FeatureTalk,
					Model: "local/new-model", Outcome: workspace.OutcomeDone}); err != nil {
					t.Fatal(err)
				}
				if err := a.attach(next, next.Root(), false); err != nil {
					t.Fatal(err)
				}
			case "shelf":
				a.closeWork()
			case "close":
				if err := a.Close(); err != nil {
					t.Fatal(err)
				}
			}
			if a.settings != (settingsView{}) || !reflect.DeepEqual(usageViews[a].Record, usage.RecordView{Expanded: -1}) || aiViewOf(a).ServiceForm.Key != "" {
				t.Fatal("leaving work kept settings, records or an unsaved key")
			}
			old, err := workspace.Open(oldRoot)
			if err != nil {
				t.Fatal(err)
			}
			defer old.Close()
			if rs, err := old.Records(); err != nil || len(rs) != 50 {
				t.Fatal("leaving changed stored records", len(rs), err)
			}
			if how == "close" {
				return
			}
			a.openPrefs("AI")
			tt.Frame()
			if next != nil {
				if len(usageViews[a].Record.Records) != 1 || usageViews[a].Record.Records[0].Model != "local/new-model" {
					t.Fatal("new work did not load its own records")
				}
				click(t, tt, "查看全部 1 次")
				if first, _ := usageViews[a].Record.List.Visible(); first != 0 || usageViews[a].Record.Only != 0 || usageViews[a].Record.Expanded != -1 {
					t.Fatal("new work inherited the previous record selection")
				}
			} else if len(usageViews[a].Record.Records) != 0 || slices.Contains(tt.Texts(), "用量") || tt.HasText("AI 记录") {
				t.Fatal("shelf settings still show the previous work")
			}
		})
	}
}

// The AI's settings show what the work spent on each feature, read as
// they open: the requests, the tokens, the share read from the cache and
// the cost estimated, or that a model has no price. There is nothing to
// clear it with. 查看全部 lists each request, the newest first, by
// feature and where it was for; a line opens on what it sent and what the
// provider counted. With the AI off, none of it shows.
func TestTheWorkShowsWhatItSpent(t *testing.T) {
	a, tt, _ := fixture(t)
	a.openPrefs("AI")
	tt.Frame()
	if !slices.Contains(tt.Texts(), "用量") || tt.HasText("按内置价格估算") || !tt.HasText("还没用过 AI。") {
		t.Fatal("nothing spent yet", tt.Texts())
	}
	chapter, err := a.Store.NewChapter("", "第一章", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.Store.Append(talkOf(a).Session, workspace.Event{Role: workspace.RoleUser, Text: "灯为什么会灭"}); err != nil {
		t.Fatal(err)
	}
	a.SetError(a.refresh())
	a.SetError(a.loadTree())
	at := time.Now().Add(-time.Hour)
	for i, r := range []workspace.Record{
		{Feature: workspace.FeatureTalk, Talk: talkOf(a).Session, Model: "anthropic/claude-opus-5-5", Usage: workspace.Usage{Input: 1_682_400, CacheRead: 1_211_300, Output: 59_200}, Cost: 4.444, Currency: "$", Outcome: workspace.OutcomeDone,
			Parts: []workspace.Part{{Name: workspace.PartRules, Tokens: 1100}, {Name: workspace.PartTalk, Tokens: 9600}}},
		{Feature: workspace.FeatureTalk, Talk: talkOf(a).Session, Model: "deepseek/deepseek-flash", Usage: workspace.Usage{Input: 142_000, CacheRead: 85_200, Output: 4100}, Cost: 0.021, Currency: "$", Outcome: workspace.OutcomeDone},
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
	if !tt.HasText("还没用过 AI。") {
		t.Fatal("read again each frame")
	}
	a.settings.open = false
	a.openPrefs("AI")
	tt.Frame()
	for _, s := range []string{
		"讨论", "2 次请求 · 输入 1.8M，缓存命中 71% · 输出 63.3K", "约 $4.47",
		"续写", "1 次请求 · 输入 900，缓存命中 0% · 输出 30", "价格未知",
		"检查", "1 次请求 · 输入 7K，缓存命中 0% · 输出 0", "不到 $0.01",
		"查看全部 4 次",
	} {
		if !tt.HasText(s) {
			t.Fatal(s, tt.Texts())
		}
	}
	if tt.HasText("还没用过 AI。") || tt.HasText("清零") {
		t.Fatal(tt.Texts())
	}

	if err := tt.Click("查看全部 4 次"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	for _, s := range []string{"AI 记录", "只记发了什么、多少字，不存原文。",
		"‹ 用量", "检查 · 已删除的章节", "失败：无法连接模型服务", "续写 · 第一章", "已取消", "讨论 · 灯为什么会灭", "完成", "1.7M", "约 $4.44", "—"} {
		if !tt.HasText(s) {
			t.Fatal(s, tt.Texts())
		}
	}
	if tt.HasText("关闭 AI") || tt.HasText("选用模型") || tt.HasText("本作品每次向模型发的请求") {
		t.Fatal("the record shows with the rest", tt.Texts())
	}
	if texts := tt.Texts(); indexOf(texts, "检查 · 已删除的章节") > indexOf(texts, "讨论 · 灯为什么会灭") {
		t.Fatal("not the newest first", texts)
	}
	if err := tt.Click("记录-" + kit.When(at.Add(3*time.Minute), time.Now()) + " 检查 · 已删除的章节"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	for _, s := range []string{"deepseek-flash · High", "发了：规则 900 · 本章正文 4.4K", "实际用量：输入 7K；输出 0"} {
		if !tt.HasText(s) {
			t.Fatal(s, tt.Texts())
		}
	}
	if err := tt.Click("记录-" + kit.When(at.Add(2*time.Minute), time.Now()) + " 续写 · 第一章"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("qwen3 · 不思考") || tt.HasText("发了：规则 900 · 本章正文 4.4K") {
		t.Fatal("one line opens at a time", tt.Texts())
	}
	usageViews[a].Record.Only = 5 // 检查
	tt.Frame()
	if !tt.HasText("检查 · 已删除的章节") || tt.HasText("续写 · 第一章") || tt.HasText("讨论 · 灯为什么会灭") {
		t.Fatal("the filter", tt.Texts())
	}
	usageViews[a].Record.Only = 2 // 起草
	tt.Frame()
	if !tt.HasText("没有记录。") {
		t.Fatal(tt.Texts())
	}
	if err := tt.Click("‹ 用量"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !slices.Contains(tt.Texts(), "用量") || tt.HasText("AI 记录") {
		t.Fatal(tt.Texts())
	}
	usageViews[a].Record.Showing = true
	a.turnAI(false)
	tt.Frame()
	if tt.HasText("AI 记录") || slices.Contains(tt.Texts(), "用量") || tt.HasText("检查 · 已删除的章节") {
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

// With a long record, a frame builds only the lines in view and a few
// beyond; a line still opens, and the filter still filters. The indices
// of the lines are found when the record or the filter changes, not each
// frame.
func TestTheRecordBuildsOnlyWhatShows(t *testing.T) {
	a, tt, _ := fixture(t)
	a.openPrefs("AI")
	tt.Frame()
	usageViews[a].Record.Showing = true
	usageViews[a].Record.Records = make([]workspace.Record, 10_000)
	at := time.Now()
	for i := range usageViews[a].Record.Records {
		usageViews[a].Record.Records[i] = workspace.Record{Feature: workspace.FeatureTalk, Model: "deepseek/deepseek-flash",
			Usage: workspace.Usage{Input: 100, Output: 10}, Outcome: workspace.OutcomeDone, At: at.Add(-time.Duration(i) * time.Minute)}
	}
	usageViews[a].Record.Rows = nil
	usageViews[a].Record.Expanded = -1
	tt.Frame()
	tt.Frame()
	built := 0
	var first string
	for _, s := range tt.Texts() {
		if strings.HasPrefix(s, "记录-") {
			built++
			if first == "" {
				first = s
			}
		}
	}
	if built == 0 || built > 40 {
		t.Fatalf("%d lines built of 10000", built)
	}
	if len(usageViews[a].Record.Rows) != len(usageViews[a].Record.Records) {
		t.Fatalf("%d lines found", len(usageViews[a].Record.Rows))
	}
	rows := &usageViews[a].Record.Rows[0]
	tt.Frame()
	if &usageViews[a].Record.Rows[0] != rows {
		t.Fatal("the lines were found again with nothing changed")
	}
	if err := tt.Click(first); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if usageViews[a].Record.Expanded != len(usageViews[a].Record.Records)-1 {
		t.Fatalf("opened record %d", usageViews[a].Record.Expanded)
	}
	if !tt.HasText("实际用量：输入 100；输出 10") {
		t.Fatal("the line did not open", tt.Texts())
	}
	usageViews[a].Record.Only = 5 // 检查, which nothing was asked for
	tt.Frame()
	if !tt.HasText("没有记录。") || len(usageViews[a].Record.Rows) != 0 {
		t.Fatalf("the filter left %d lines", len(usageViews[a].Record.Rows))
	}
	for _, s := range tt.Texts() {
		if strings.HasPrefix(s, "记录-") {
			t.Fatal("a line was built for no record")
		}
	}
}

// SHOT=dir go test ./desk -run TestShotRecord draws the AI record, a line
// open, and a filter that leaves nothing.
func TestShotRecord(t *testing.T) {
	testshot.Dir(t)
	a, tt, _ := fixture(t)
	chapter, err := a.Store.NewChapter("", "第一章", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.Store.Append(talkOf(a).Session, workspace.Event{Role: workspace.RoleUser, Text: "灯为什么会灭"}); err != nil {
		t.Fatal(err)
	}
	a.SetError(a.refresh())
	a.SetError(a.loadTree())
	at := time.Now().Add(-time.Hour)
	for i, r := range []workspace.Record{
		{Feature: workspace.FeatureTalk, Talk: talkOf(a).Session, Model: "anthropic/claude-opus-5-5", Usage: workspace.Usage{Input: 1_682_400, CacheRead: 1_211_300, Output: 59_200}, Cost: 4.444, Currency: "$", Outcome: workspace.OutcomeDone,
			Parts: []workspace.Part{{Name: workspace.PartRules, Tokens: 1100}, {Name: workspace.PartTalk, Tokens: 9600}}},
		{Feature: workspace.FeatureContinue, Chapter: chapter, Model: "local/qwen3", Effort: "none", Usage: workspace.Usage{Input: 900, Output: 30}, Outcome: workspace.OutcomeCancelled},
		{Feature: workspace.FeatureRewrite, Chapter: chapter, Model: "deepseek/deepseek-flash", Usage: workspace.Usage{Input: 5000, Output: 700}, Cost: 0.001, Currency: "$", Outcome: workspace.OutcomeDone},
		{Feature: workspace.FeatureTalk, Talk: talkOf(a).Session, Model: "deepseek/deepseek-flash", Usage: workspace.Usage{Input: 142_000, CacheRead: 85_200, Output: 4100}, Cost: 0.021, Currency: "$", Outcome: workspace.OutcomeCancelled},
		{Feature: workspace.FeatureCheck, Chapter: "gone", Model: "deepseek/deepseek-flash", Effort: "high", Usage: workspace.Usage{Input: 7000}, Cost: 0.002, Currency: "$", Outcome: workspace.OutcomeFailed, Error: "无法连接模型服务",
			Parts: []workspace.Part{{Name: workspace.PartRules, Tokens: 900}, {Name: workspace.PartProse, Tokens: 4400}}},
	} {
		r.At = at.Add(time.Duration(i) * time.Minute)
		if err := a.Store.Log(r); err != nil {
			t.Fatal(err)
		}
	}
	a.openPrefs("AI")
	tt.Frame()
	usageViews[a].Record.Showing = true
	usageViews[a].Record.Only = 0
	usageViews[a].Record.Expanded = 4 // the check, whose detail is longest
	shot(t, tt, "record.png")
	usageViews[a].Record.Expanded = -1
	usageViews[a].Record.Only = 2 // 起草, which nothing was asked for
	shot(t, tt, "record-none.png")
}
