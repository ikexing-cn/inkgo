package desk

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/discussion"
	"writer/model"
	"writer/workspace"
)

// fixture opens a work whose setting holds one entry the author wrote.
func fixture(t *testing.T) (*App, *ui.Tester, workspace.Entry) {
	t.Helper()
	root := t.TempDir()
	s, err := workspace.Create(filepath.Join(root, "work"), "测试作品", "网文")
	if err != nil {
		t.Fatal(err)
	}
	r, err := s.Revise(workspace.Revision{By: workspace.ByAuthor, Ops: []workspace.Op{{Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: "人物", Text: "主角怕黑"}}})
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	lib, was := filepath.Join(root, "library"), Library
	Library = func() string { return lib }
	t.Cleanup(func() { Library = was })
	a := NewWithConfig(filepath.Join(root, "providers.yaml"))
	if err = a.Open(filepath.Join(root, "work")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.Close() })
	tt := ui.NewTester(a.View, 1266, 780)
	tt.Frame()
	tt.Frame()
	e, _ := a.set.Entry(r.Ops[0].Entry)
	return a, tt, e
}

func settle(t *testing.T, a *App, tt *ui.Tester) {
	t.Helper()
	if a.busy == "" {
		return
	}
	select {
	case r := <-a.results:
		a.results <- r
		tt.Frame()
	case <-time.After(3 * time.Second):
		t.Fatal("worker did not finish")
	}
}

// answer is a model that gives its replies in order.
type answer struct {
	mu      sync.Mutex
	replies []string
}

func (f *answer) Complete(ctx context.Context, r model.Request, on func(model.Delta)) (model.Reply, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.replies) == 0 {
		return model.Reply{}, errors.New("unexpected model call")
	}
	s := f.replies[0]
	f.replies = f.replies[1:]
	return model.Reply{Text: s, Reasoning: "想一想"}, nil
}

// The author changes the setting directly: each change is a revision of
// theirs, and what they drop can come back.
func TestTheAuthorEditsTheSetting(t *testing.T) {
	a, tt, e := fixture(t)
	if !tt.HasText("主角怕黑") || !tt.HasText("人物 · 1") {
		t.Fatal("the setting is not shown", tt.Texts())
	}
	if err := tt.Click("编辑-" + e.ID); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	a.doc.text, a.doc.category = "主角怕安静", "关系"
	tt.Frame() // the form has no names for a relation
	if err := tt.Click("保存"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	got, _ := a.set.Entry(e.ID)
	if got.Text != "主角怕安静" || got.Category != "关系" || !got.Author || a.doc.editing != "" {
		t.Fatalf("edit %+v", got)
	}
	if err := tt.Click("放弃-" + e.ID); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got, _ = a.set.Entry(e.ID); got.Place != workspace.PlaceDropped {
		t.Fatal(got.Place)
	}
	*a.doc.open[workspace.PlaceDropped+"/"] = true
	tt.Frame()
	if err := tt.Click("移回-" + e.ID); err != nil {
		t.Fatal(err, tt.Texts())
	}
	tt.Frame()
	if got, _ = a.set.Entry(e.ID); got.Place != workspace.PlaceBody {
		t.Fatal(got.Place)
	}
	if err := tt.Click("新增设定"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	a.doc.text = "雨从不停"
	if err := tt.Click("保存"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(a.set.Entries) != 2 || a.set.Entries[1].Category != workspace.Other {
		t.Fatalf("new entry %+v", a.set.Entries)
	}
	for _, r := range a.set.Revisions {
		if r.By != workspace.ByAuthor {
			t.Fatal("an author's change recorded as the bookkeeper's")
		}
	}
}

// The bookkeeper works in the background: a turn saved brings the setting
// up to date without the author doing anything, and what it changed is
// tinted for a moment.
func TestBookkeeperFillsTheSettingInTheBackground(t *testing.T) {
	a, tt, _ := fixture(t)
	f := &answer{replies: []string{`{"ops":[{"op":"add","place":"body","category":"规则","text":"雨从不停","sources":[{"msg":"m1","quote":"雨从不停"}]}]}`}}
	a.Book = func(s *workspace.Store) (*discussion.Bookkeeper, error) {
		return &discussion.Bookkeeper{Store: s, Model: discussion.Model{Completer: f, Name: "deepseek/deepseek-flash"}}, nil
	}
	a.halt() // the next frame starts one with the bookkeeper above
	tt.Frame()
	var c *ui.Context
	tt = ui.NewTester(func(ctx *ui.Context) { c = ctx; a.View(ctx) }, 1266, 780)
	tt.Frame()
	// A turn saved the way a send saves it; nothing else wakes the bookkeeper.
	a.run(c, "AI 正在思考…", func(context.Context) (func(), error) {
		u, err := a.Store.Append(a.session, workspace.Event{Role: workspace.RoleUser, Text: "这座城市雨从不停。"})
		if err == nil {
			_, err = a.Store.Append(a.session, workspace.Event{Role: workspace.RoleAssistant, ReplyTo: u.ID, Text: "好"})
		}
		return nil, err
	})
	settle(t, a, tt)
	deadline := time.Now().Add(3 * time.Second)
	for len(a.set.Entries) < 2 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
		tt.Frame()
	}
	if len(a.set.Entries) != 2 || !tt.HasText("雨从不停") {
		t.Fatalf("setting %+v %q", a.set.Entries, a.Error)
	}
	if _, ok := a.flash[a.set.Entries[1].ID]; !ok {
		t.Fatal("the bookkeeper's change is not tinted")
	}
	if err := tt.Click("设定历史"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("后台整理") || !tt.HasText("新增到正文 · 规则：雨从不停") || !tt.HasText("整理时的思考") {
		t.Fatal("history", tt.Texts())
	}
}

// A batch the bookkeeper could not take in shows in the history only, with
// a retry.
func TestFailedBookkeepingShowsInHistory(t *testing.T) {
	a, tt, _ := fixture(t)
	u, _ := a.Store.Append(a.session, workspace.Event{Role: workspace.RoleUser, Text: "雨从不停。"})
	r, _ := a.Store.Append(a.session, workspace.Event{Role: workspace.RoleAssistant, ReplyTo: u.ID, Text: "好"})
	failed, err := a.Store.Revise(workspace.Revision{By: workspace.ByBookkeeper, Session: a.session, From: u.ID, Through: r.ID, Failed: "服务暂时不可用"})
	if err != nil {
		t.Fatal(err)
	}
	a.setError(a.refresh())
	tt.Frame()
	if tt.HasText("这几轮没整理成功：服务暂时不可用") {
		t.Fatal("a failure shows outside the history")
	}
	a.doc.hidden = true
	tt.Frame()
	if err = tt.Click("上次整理没成功，点开看看"); err != nil {
		t.Fatal("no sign of the failure", err)
	}
	tt.Frame()
	if !a.doc.history || a.doc.hidden || !tt.HasText("这几轮没整理成功：服务暂时不可用") {
		t.Fatal("history", tt.Texts())
	}
	f := &answer{replies: []string{`{"ops":[]}`}}
	a.Book = func(s *workspace.Store) (*discussion.Bookkeeper, error) {
		return &discussion.Bookkeeper{Store: s, Model: discussion.Model{Completer: f}}, nil
	}
	if err = tt.Click("重试整理-" + failed.ID); err != nil {
		t.Fatal(err)
	}
	settle(t, a, tt)
	if !a.set.Redone(failed.ID) || a.Error != "" {
		t.Fatalf("redo %q", a.Error)
	}
	tt.Frame()
	if _, ok := tt.Find("上次整理没成功，点开看看"); ok {
		t.Fatal("the sign outlived the retry")
	}
}

// The panel folds away for the talk alone, and stays folded when the work
// opens again.
func TestSettingPanelFoldsAndRemembers(t *testing.T) {
	a, tt, _ := fixture(t)
	if err := tt.Click("收起设定"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.doc.hidden || tt.HasText("主角怕黑") {
		t.Fatal("panel still shown")
	}
	path := a.path
	a.closeWork()
	if err := a.Open(path); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.doc.hidden {
		t.Fatal("folding was not remembered")
	}
	if err := tt.Click("展开设定"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.doc.hidden || !tt.HasText("主角怕黑") {
		t.Fatal("panel not shown again")
	}
}

func TestSessionTitles(t *testing.T) {
	a, tt, _ := fixture(t)
	if _, err := a.Store.Append(a.session, workspace.Event{Role: "user", Text: "雨城的故事\n第二行"}); err != nil {
		t.Fatal(err)
	}
	if err := a.refresh(); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if title := a.titles[a.session]; !strings.HasSuffix(title, "雨城的故事") {
		t.Fatalf("title %q", title)
	}
}

func TestExtractionAddsTheAuthorsEntry(t *testing.T) {
	a, tt, _ := fixture(t)
	m, err := a.Store.Append(a.session, workspace.Event{Role: "user", Text: "林晚修路灯"})
	if err != nil {
		t.Fatal(err)
	}
	a.setError(a.refresh())
	tt.Frame()
	a.openSource(m, 0, 2)
	tt.Frame()
	if err = tt.Click("加入设定"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	last := a.set.Entries[len(a.set.Entries)-1]
	if last.Text != "林晚" || last.Place != workspace.PlaceBody || last.Sources[0].Message != m.ID || a.sourceOpen {
		t.Fatalf("%+v", last)
	}
}

func TestCancellationKeepsUIResponsive(t *testing.T) {
	a, tt, _ := fixture(t)
	var viewContext *ui.Context
	tt = ui.NewTester(func(c *ui.Context) { viewContext = c; a.View(c) }, 1266, 780)
	tt.Frame()
	a.run(viewContext, "测试中的请求", func(ctx context.Context) (func(), error) { <-ctx.Done(); return nil, ctx.Err() })
	tt.Frame()
	if err := tt.Click("取消请求"); err != nil {
		t.Fatal(err)
	}
	settle(t, a, tt)
	if a.busy != "" || a.cancel != nil || a.Error != context.Canceled.Error() {
		t.Fatalf("cancel result %q %q", a.busy, a.Error)
	}
}

func TestFailureRefreshesPersistedWork(t *testing.T) {
	a, tt, _ := fixture(t)
	var c *ui.Context
	tt = ui.NewTester(func(ctx *ui.Context) { c = ctx; a.View(ctx) }, 1266, 780)
	tt.Frame()
	a.run(c, "讨论", func(context.Context) (func(), error) {
		if _, err := a.Store.Append(a.session, workspace.Event{Role: "user", Text: "失败前已经落盘的发言"}); err != nil {
			return nil, err
		}
		return nil, errors.New("网络失败")
	})
	settle(t, a, tt)
	if len(a.events) != 1 || a.Error != "网络失败" {
		t.Fatalf("persisted partial result lost: %d %q", len(a.events), a.Error)
	}
}

func TestNewWorkDoesNotInheritLastWork(t *testing.T) {
	a, _, _ := fixture(t)
	a.closeWork()
	if a.path != "" || a.name != "" {
		t.Fatalf("welcome form kept the last work: %q %q", a.path, a.name)
	}
	a.path = filepath.Join(t.TempDir(), "雨城来信")
	if err := a.create(); err != nil {
		t.Fatal(err)
	}
	if name := a.Store.Work().Name; name != "雨城来信" {
		t.Fatalf("an unnamed work is named after its folder, got %q", name)
	}
}

// Inputs the store would reject are caught first, in words the author reads.
func TestIncompleteFormsExplainInChinese(t *testing.T) {
	a, _, _ := fixture(t)
	check := func(name string, err error) {
		t.Helper()
		if err == nil {
			t.Fatalf("%s: accepted", name)
		}
		for _, field := range []string{"discussion:", "bookkeeping:", "categories:"} {
			if strings.Contains(err.Error(), field) {
				t.Fatalf("%s: store error leaked: %v", name, err)
			}
		}
	}
	a.talkProvider, a.talkModel = "deepseek", ""
	check("discussion model", a.saveUsage())
	a.talkModel, a.bookProvider, a.bookModel = "deepseek-flash", "deepseek", ""
	check("bookkeeping model", a.saveUsage())
	a.categoryName, a.categoryTarget = "伏笔", ""
	check("category target", a.saveCategory())
}

func TestMarkdownBlocks(t *testing.T) {
	src := "# 标题\n\n有**粗体**、*斜体*、`代码`和[链接](https://example.com)。\n\n- 一\n- 二\n\n1. 甲\n\n> 引用\n\n```\ncode\n```\n\n| 列 | 值 |\n|---|---|\n| 甲 | 1 |\n\n<script>x</script>\n\n---\n"
	blocks := mdCache{}.get(src)
	var kinds []mdKind
	for _, b := range blocks {
		kinds = append(kinds, b.kind)
	}
	want := []mdKind{mdHeading, mdParagraph, mdList, mdList, mdQuote, mdCode, mdTable, mdParagraph, mdRule}
	if fmt.Sprint(kinds) != fmt.Sprint(want) {
		t.Fatalf("kinds %v, want %v", kinds, want)
	}
	styled := map[string]mdSpan{}
	for _, s := range blocks[1].inline {
		styled[s.text] = s
	}
	if !styled["粗体"].bold || !styled["斜体"].italic || !styled["代码"].code || !styled["链接"].link {
		t.Fatalf("inline styles %+v", blocks[1].inline)
	}
	if !blocks[3].ordered || blocks[3].start != 1 || blocks[5].code != "code" || len(blocks[6].rows) != 2 || !blocks[6].rows[0][0][0].bold {
		t.Fatalf("blocks %+v", blocks)
	}
	if blocks[7].inline[0].text != "<script>x</script>" {
		t.Fatal("raw HTML must show as text")
	}
}

// A turn in flight shows its reasoning, then its reply, as they stream.
func TestLiveTurnStreams(t *testing.T) {
	a, tt, _ := fixture(t)
	var c *ui.Context
	tt = ui.NewTester(func(ctx *ui.Context) { c = ctx; a.View(ctx) }, 1266, 780)
	tt.Frame()
	block := make(chan struct{})
	a.pending = "她做什么工作？"
	a.run(c, "AI 正在思考", func(ctx context.Context) (func(), error) { <-block; return nil, nil })
	a.talking = true
	a.live.add(model.Delta{Reasoning: "她怕黑，职业要和光有关"})
	tt.Frame()
	if !tt.HasText("她怕黑，职业要和光有关") || !tt.HasText("正在思考…") {
		t.Fatal("reasoning not streamed", tt.Texts())
	}
	a.live.add(model.Delta{Text: "可以让她**修灯**。"})
	tt.Frame()
	if !tt.HasText("正在回复…") || tt.HasText("**") {
		t.Fatal("reply not streamed as markdown", tt.Texts())
	}
	close(block)
	settle(t, a, tt)
}

func TestEffortMenuSavesToWork(t *testing.T) {
	a, tt, _ := fixture(t)
	a.setTalk(func(s *workspace.Selection) { s.Provider, s.Model = "deepseek", "deepseek-flash" })
	a.setTalk(func(s *workspace.Selection) { s.Effort = string(model.EffortMax) })
	tt.Frame()
	if w := a.Store.Work().Discussion; w.Effort != "max" || w.Model != "deepseek-flash" || a.talkEffort != "max" {
		t.Fatalf("selection %+v", w)
	}
	if !tt.HasText("deepseek-flash") || !tt.HasText("Max") {
		t.Fatal("chip does not show model and effort", tt.Texts())
	}
}

// A reply's paragraphs, headings and lists are one text, which the author
// selects across; Copy takes its Markdown.
func TestReplyTextSelectsAcrossBlocksAndCopies(t *testing.T) {
	a, tt, _ := fixture(t)
	reply := "第一段。\n\n## 小标题\n\n- 甲\n- 乙\n\n第二段。\n\n```\ncode\n```"
	u, err := a.Store.Append(a.session, workspace.Event{Role: workspace.RoleUser, Text: "说说"})
	if err != nil {
		t.Fatal(err)
	}
	r, err := a.Store.Append(a.session, workspace.Event{Role: workspace.RoleAssistant, ReplyTo: u.ID, Text: reply})
	if err != nil {
		t.Fatal(err)
	}
	a.setError(a.refresh())
	tt.Frame()
	if !tt.HasText("第一段。\n\n小标题\n\n• 甲\n• 乙\n\n第二段。") || !tt.HasText("code") {
		t.Fatal("text blocks do not run together", tt.Texts())
	}
	flow, _ := tt.Find("第一段。\n\n小标题\n\n• 甲\n• 乙\n\n第二段。")
	tt.Press(flow.X+2, flow.Y+4)
	tt.Move(flow.X+flow.W/2, flow.Y+flow.H-4)
	tt.Release(flow.X+flow.W/2, flow.Y+flow.H-4)
	tt.Key(ui.Ctrl, ui.KeyC)
	if got := tt.Clipboard(); !strings.Contains(got, "第一段。") || !strings.Contains(got, "第二段") {
		t.Fatalf("a drag selected %q", got)
	}
	if err = tt.Click("复制-" + r.ID); err != nil {
		t.Fatal(err)
	}
	if tt.Clipboard() != reply {
		t.Fatalf("copied %q", tt.Clipboard())
	}
}

// gate is a model that answers once let through.
type gate struct{ open chan struct{} }

func (g gate) Complete(ctx context.Context, r model.Request, on func(model.Delta)) (model.Reply, error) {
	select {
	case <-g.open:
		return model.Reply{Text: `{"ops":[]}`}, nil
	case <-ctx.Done():
		return model.Reply{}, ctx.Err()
	}
}

// While the bookkeeper works, a dot says so; it goes when the work is done.
func TestBookkeeperShowsWhileItWorks(t *testing.T) {
	a, tt, _ := fixture(t)
	g := gate{open: make(chan struct{})}
	a.Book = func(s *workspace.Store) (*discussion.Bookkeeper, error) {
		return &discussion.Bookkeeper{Store: s, Model: discussion.Model{Completer: g}}, nil
	}
	a.halt()
	tt.Frame()
	u, _ := a.Store.Append(a.session, workspace.Event{Role: workspace.RoleUser, Text: "雨从不停。"})
	a.Store.Append(a.session, workspace.Event{Role: workspace.RoleAssistant, ReplyTo: u.ID, Text: "好"})
	a.kick()
	shown := func(want bool) {
		t.Helper()
		for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
			tt.Frame()
			if _, ok := tt.Find("正在整理设定"); ok == want {
				return
			}
		}
		t.Fatalf("dot shown should be %v", want)
	}
	shown(true)
	a.setDocHidden(true)
	shown(true) // beside the button that brings the panel back
	close(g.open)
	shown(false)
}

// Services are chosen by the names the author gave them, and kept by ID.
func TestChooseAServiceByName(t *testing.T) {
	a, tt, _ := fixture(t)
	if err := tt.Click("模型设置"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("写作提供商"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	texts := tt.Texts()
	if options := strings.Join(texts[len(texts)-5:], " "); options != "沿用讨论模型 DeepSeek Kimi OpenAI Anthropic" {
		t.Fatal(options)
	}
	tt.Key(0, ui.KeyDown)
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if a.penProvider != "deepseek" {
		t.Fatal(a.penProvider)
	}
}

// A confirmation goes away on its own; an error stays until dismissed.
func TestNoticesFade(t *testing.T) {
	a, tt, _ := fixture(t)
	was := noticeFor
	noticeFor = 50 * time.Millisecond
	t.Cleanup(func() { noticeFor = was })
	a.notice, a.Error = "已保存", "出错了"
	tt.Frame()
	if !tt.HasText("已保存") {
		t.Fatal(tt.Texts())
	}
	until(t, tt, "the notice to fade", func() bool { return !tt.HasText("已保存") })
	if !tt.HasText("出错了") || a.notice != "" {
		t.Fatal(tt.Texts())
	}
}

// Clicking a model field lists the service's models; typing narrows them.
func TestPickAModelFromTheList(t *testing.T) {
	a, tt, _ := fixture(t)
	a.talkProvider, a.talkModel = "deepseek", "deepseek-flash"
	if err := tt.Click("模型设置"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("讨论模型"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("deepseek-v4-pro") {
		t.Fatal(tt.Texts())
	}
	if err := tt.Click("deepseek-v4-pro"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if a.talkModel != "deepseek-v4-pro" || tt.HasText("deepseek-flash") {
		t.Fatal(a.talkModel, tt.Texts())
	}
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("flash")
	tt.Frame()
	if !tt.HasText("deepseek-flash") || tt.HasText("deepseek-v4-pro") {
		t.Fatal(tt.Texts())
	}
}

// Shelving a conversation takes it out of the list but keeps its file;
// the one open gives way to the latest left, and it can be put back.
func TestShelveAConversation(t *testing.T) {
	a, tt, _ := fixture(t)
	old := a.session
	a.newSession()
	open := a.session
	tt.Frame()
	if err := tt.RightClick("对话-" + old); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("收起这段对话"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if _, ok := tt.Find("对话-" + old); ok || !tt.HasText("已收起 1 段对话") {
		t.Fatal("still listed", tt.Texts())
	}
	if p := savedPrefs(t, a); !slices.Equal(p.Shelved, []string{old}) {
		t.Fatal("saved", p.Shelved)
	}
	if ids, _ := a.Store.Sessions(); !slices.Contains(ids, old) {
		t.Fatal("the file went", ids)
	}
	if err := tt.Click("收起的对话"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.RightClick("对话-" + old); err != nil {
		t.Fatal("not shown", err)
	}
	if err := tt.ChooseMenuItem("放回列表"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(savedPrefs(t, a).Shelved) != 0 || tt.HasText("已收起 1 段对话") {
		t.Fatal("not back")
	}
	a.shelve(open)
	if a.session != old {
		t.Fatal("the open one stays", a.session)
	}
	a.shelve(old)
	if a.session == old || a.session == open {
		t.Fatal("no new conversation", a.session)
	}
}
