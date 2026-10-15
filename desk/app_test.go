package desk

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/ai"
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
	// Left on the discussion page, where most tests start.
	if err = os.WriteFile(filepath.Join(root, "work", ".writer", "desk.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	lib, was := filepath.Join(root, "library"), Library
	Library = func() string { return lib }
	t.Cleanup(func() { Library = was })
	a := newApp(filepath.Join(root, "providers.yaml"))
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

// newApp is the app on the services configured in config, or on none.
func newApp(config string) *App { return New(ai.New(config)) }

// aiOf is the AI of a, for a test to swap fake models in.
func aiOf(a *App) *ai.Service { return a.ai.(*ai.Service) }

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
	if !tt.HasText("主角怕黑") || !tt.HasText("人物") {
		t.Fatal("the setting is not shown", tt.Texts())
	}
	if err := tt.Click("设定-" + e.ID); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("修改"); err != nil {
		t.Fatal(err, tt.Texts())
	}
	tt.Frame()
	a.doc.text, a.doc.category = "主角怕安静", "关系"
	tt.Frame()
	if err := tt.Click("保存"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	got, _ := a.set.Entry(e.ID)
	if got.Text != "主角怕安静" || got.Category != "关系" || !got.Author || a.doc.editing != "" {
		t.Fatalf("edit %+v", got)
	}
	if err := tt.Click("删除设定"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got, _ = a.set.Entry(e.ID); got.Place != workspace.PlaceDropped {
		t.Fatal(got.Place)
	}
	if err := tt.Click("更多"); err != nil {
		t.Fatal(err)
	}
	if err := pick(tt, "已删除"); err != nil {
		t.Fatal(err, rows(tt))
	}
	tt.Frame()
	if err := tt.Click("设定-" + e.ID); err != nil {
		t.Fatal(err, tt.Texts())
	}
	tt.Frame()
	if err := tt.Click("恢复设定"); err != nil {
		t.Fatal(err, tt.Texts())
	}
	tt.Frame()
	if got, _ = a.set.Entry(e.ID); got.Place != workspace.PlaceBody {
		t.Fatal(got.Place)
	}
	if err := tt.Click("全部设定"); err != nil {
		t.Fatal(err, tt.Texts())
	}
	tt.Frame()
	if err := tt.Click("新增"); err != nil {
		t.Fatal(err, tt.Texts())
	}
	tt.Frame()
	a.doc.text = "雨从不停"
	if err := tt.Click("保存"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	// A new entry starts in the first category, as in the prototype.
	if len(a.set.Entries) != 2 || a.set.Entries[1].Category != "人物" {
		t.Fatalf("new entry %+v", a.set.Entries)
	}
	for _, r := range a.set.Revisions {
		if r.By != workspace.ByAuthor {
			t.Fatal("an author's change recorded as a talk's")
		}
	}
}

// A reply looks things up and notes the setting on its way: what it
// noted shows at once, tinted, and the reply folds above it what it looked
// up, open while it looks. The history says the talk noted it and goes
// back to the talk.
func TestTalkNotesTheSettingAsItReplies(t *testing.T) {
	a, tt, _ := fixture(t)
	var c *ui.Context
	tt = ui.NewTester(func(ctx *ui.Context) { c = ctx; a.View(ctx) }, 1266, 780)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true}) // the fold opens at once
	tt.Frame()
	session := a.session
	block := make(chan struct{})
	var rev workspace.Revision
	var reply workspace.Event
	sv := c.Services()
	a.pending = "这座城市雨从不停。"
	a.run(c, "AI 正在思考", func(context.Context) (func(), error) {
		// What the engine writes, in its order, the way a reply writes it.
		u, err := a.Store.Append(session, workspace.Event{Role: workspace.RoleUser, Text: "这座城市雨从不停。"})
		if err != nil {
			return nil, err
		}
		call, err := a.Store.Append(session, workspace.Event{Role: workspace.RoleCall, ReplyTo: u.ID, Calls: []workspace.Call{
			{ID: "c1", Name: "search_book", Input: []byte(`{"query":"雨"}`), Label: "搜了全书「雨」"},
			{ID: "c2", Name: "note_setting", Input: []byte(`{"op":"add"}`), Label: "记下：正文 · 规则：雨从不停"},
		}})
		if err != nil {
			return nil, err
		}
		a.stepped(sv, call)
		found, err := a.Store.Append(session, workspace.Event{Role: workspace.RoleResult, ReplyTo: u.ID, Call: "c1", Text: "全书的正文里没有「雨」。"})
		if err != nil {
			return nil, err
		}
		a.stepped(sv, found)
		if rev, err = a.Store.Revise(workspace.Revision{By: workspace.ByTalk, Session: session, Turn: u.ID, Model: "deepseek/deepseek-flash", Ops: []workspace.Op{
			{Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: "规则", Text: "雨从不停", Sources: []workspace.Source{{Session: session, Message: u.ID, Quote: "雨从不停"}}},
		}}); err != nil {
			return nil, err
		}
		noted, err := a.Store.Append(session, workspace.Event{Role: workspace.RoleResult, ReplyTo: u.ID, Call: "c2", Text: "已记下，编号 2：正文 · 规则：雨从不停", Revision: rev.ID})
		if err != nil {
			return nil, err
		}
		a.stepped(sv, noted)
		<-block
		reply, err = a.Store.Append(session, workspace.Event{Role: workspace.RoleAssistant, ReplyTo: u.ID, Text: "那就让雨一直下。"})
		return nil, err
	})
	a.talking, a.talkSession = true, a.session
	until(t, tt, "the note", func() bool { return len(a.set.Entries) == 2 })
	if _, ok := a.flash[a.set.Entries[1].ID]; !ok || !tt.HasText("雨从不停") {
		t.Fatal("what the talk noted is not shown and tinted", tt.Texts())
	}
	if !tt.HasText("正在查阅…") || !tt.HasText("全书的正文里没有「雨」。") {
		t.Fatal("what the reply looks up is not open as it looks", tt.Texts())
	}
	close(block)
	settle(t, a, tt)
	a.talking = false
	tt.Frame()
	if !tt.HasText("那就让雨一直下。") || !tt.HasText("搜了全书「雨」，记下：正文 · 规则：雨从不停") || tt.HasText("全书的正文里没有「雨」。") || !tt.HasText("2 条发言") {
		t.Fatal("the reply does not fold what it looked up", tt.Texts())
	}
	if err := tt.Click("搜了全书「雨」，记下：正文 · 规则：雨从不停"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("全书的正文里没有「雨」。") {
		t.Fatal("the fold does not open", tt.Texts())
	}
	if err := tt.Click(fmt.Sprintf("查看设定-look-%s-1", reply.ID)); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.doc.shown != a.set.Entries[1].ID {
		t.Fatal("the link does not open what was noted")
	}
	a.doc.shown, a.doc.history = "", true
	tt.Frame()
	if !tt.HasText("讨论") || !tt.HasText("新增到正文 · 规则：雨从不停") || !tt.HasText("记在「这座城市雨从不停。」") {
		t.Fatal("history", tt.Texts())
	}
	other, err := a.Store.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	a.switchSession(other)
	tt.Frame()
	if err := tt.Click("讨论-" + rev.ID); err != nil {
		t.Fatal(err)
	}
	if a.session != session || a.page != 0 {
		t.Fatal("the link does not go back to the talk")
	}
}

// The panel folds away for the talk alone, and stays folded when the work
// opens again.
func TestSettingPanelFoldsAndRemembers(t *testing.T) {
	a, tt, _ := fixture(t)
	if err := tt.Click("收起右栏"); err != nil {
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

// A work that cannot be opened is named over what is wrong with it, and the
// work open stays.
func TestAWorkThatCannotBeOpenedIsNamed(t *testing.T) {
	a, tt, _ := fixture(t)
	dir := filepath.Join(t.TempDir(), "青崖问剑")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "work.yaml"), []byte("schema: 一\n"), 0644); err != nil {
		t.Fatal(err)
	}
	a.setError(a.Open(dir))
	tt.Frame()
	if !tt.HasText("打开作品「青崖问剑」失败") || !tt.HasText("work.yaml") || a.Store.Work().Name != "测试作品" {
		t.Fatal(tt.Texts())
	}
}

// A blank talk in a work with a setting goes on from what the work has; in
// a new work it starts from an idea.
func TestABlankTalkFitsTheWork(t *testing.T) {
	a, tt, _ := fixture(t)
	if !tt.HasText("接着聊这部作品") || tt.HasText("从一个初步的想法开始") || tt.HasText("我想写一个总在下雨的城市") {
		t.Fatal(tt.Texts())
	}
	if err := tt.Click("接下来的情节怎么走"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.input != "接下来的情节怎么走" {
		t.Fatalf("%q", a.input)
	}
	a.openCreate()
	a.name, a.books.at = "新书", filepath.Join(t.TempDir(), "新书")
	if err := a.create(); err != nil || a.Error != "" {
		t.Fatal(err, a.Error)
	}
	a.page = 0
	tt.Frame()
	if !tt.HasText("从一个初步的想法开始") || !tt.HasText("我想写一个总在下雨的城市") || tt.HasText("接着聊这部作品") {
		t.Fatal(tt.Texts())
	}
}

// A new work starts with the models of the work open, or of the one last
// opened; a work whose service is no longer set up gives none.
func TestNewWorkKeepsTheModels(t *testing.T) {
	a, _, _ := fixture(t)
	a.talkProvider, a.talkModel, a.talkEffort = "deepseek", "deepseek-flash", "high"
	a.penProvider, a.penModel = "deepseek", "deepseek-v4-pro"
	if err := a.saveUsage(); err != nil {
		t.Fatal(err)
	}
	want := a.Store.Work()
	create := func(name string) workspace.Work {
		t.Helper()
		a.openCreate()
		a.name, a.books.at = name, filepath.Join(t.TempDir(), name)
		if err := a.create(); err != nil || a.Error != "" {
			t.Fatal(err, a.Error)
		}
		return a.Store.Work()
	}
	if w := create("雨城来信"); w.Discussion != want.Discussion || w.Writing != want.Writing {
		t.Fatalf("from the work open: %+v %+v", w.Discussion, w.Writing)
	}
	// The work just made goes to a service since removed; the shelf's
	// next work still has one set up.
	w := a.Store.Work()
	w.Discussion, w.Writing = workspace.Selection{Provider: "gone", Model: "m"}, workspace.Selection{}
	if err := a.Store.SaveWork(w); err != nil {
		t.Fatal(err)
	}
	a.closeWork()
	if w := create("旧城"); w.Discussion != want.Discussion || w.Writing != want.Writing {
		t.Fatalf("from the shelf: %+v %+v", w.Discussion, w.Writing)
	}
}

// Inputs the store would reject are caught first, in words the author reads;
// a service chosen without a model waits for one, unsaved.
func TestIncompleteFormsExplainInChinese(t *testing.T) {
	a, _, _ := fixture(t)
	was := a.Store.Work()
	a.talkProvider, a.talkModel = "deepseek", ""
	a.penProvider, a.penModel = "deepseek", ""
	if err := a.saveUsage(); err != nil || a.Store.Work().Discussion != was.Discussion {
		t.Fatalf("a service without a model: %v %+v", err, a.Store.Work().Discussion)
	}
	a.talkModel = "deepseek-flash"
	if err := a.saveUsage(); err != nil || a.Store.Work().Discussion != was.Discussion {
		t.Fatalf("the writing's service without a model: %v %+v", err, a.Store.Work().Discussion)
	}
	a.penModel = "deepseek-v4-pro"
	if err := a.saveUsage(); err != nil || a.Store.Work().Writing.Model != "deepseek-v4-pro" {
		t.Fatalf("complete: %v %+v", err, a.Store.Work().Writing)
	}
	// A category without its target is not saved, and is told as what is
	// left to fill in, not as a failure.
	a.categoryName, a.categoryTarget = "伏笔", ""
	if err := a.saveCategory(); err != nil || a.hint != "请填写落点，例如“设定库”或“大纲”" || len(a.Store.Work().Categories) != len(was.Categories) {
		t.Fatalf("category target: %v %q", err, a.hint)
	}
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
	a.talking, a.talkSession = true, a.session
	a.live.add("她怕黑，职业要和光有关", "")
	tt.Frame()
	if !tt.HasText("她怕黑，职业要和光有关") || !tt.HasText("正在思考…") {
		t.Fatal("reasoning not streamed", tt.Texts())
	}
	a.live.add("", "可以让她**修灯**。")
	tt.Frame()
	if !tt.HasText("正在回复…") || tt.HasText("**") {
		t.Fatal("reply not streamed as markdown", tt.Texts())
	}
	close(block)
	settle(t, a, tt)
}

// While a talk replies, the rest of the window goes on: another page,
// another talk, the words of the next turn, the model it takes. Only
// sending, trying again and switching the work wait; the reply lands in
// its own talk.
func TestTalkLeavesTheRestFree(t *testing.T) {
	a, _, _ := fixture(t)
	var c *ui.Context
	tt := ui.NewTester(func(ctx *ui.Context) { c = ctx; a.View(ctx) }, 1266, 780)
	session := a.session
	other, err := a.Store.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.Store.Append(other, workspace.Event{Role: workspace.RoleUser, Text: "井边的灯谁点的"}); err != nil {
		t.Fatal(err)
	}
	a.setError(a.refresh())
	tt.Frame()
	block := make(chan struct{})
	a.pending = "灯为什么会灭"
	a.run(c, "AI 正在思考…", func(context.Context) (func(), error) {
		u, err := a.Store.Append(session, workspace.Event{Role: workspace.RoleUser, Text: "灯为什么会灭"})
		if err != nil {
			return nil, err
		}
		<-block
		_, err = a.Store.Append(session, workspace.Event{Role: workspace.RoleAssistant, ReplyTo: u.ID, Text: "因为停电了。"})
		return func() { a.pending = "" }, err
	})
	a.talking, a.talkSession = true, session
	tt.Frame()

	if err := tt.Click("切换作品"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if tt.HasText("新建作品…") {
		t.Fatal("the work can be switched as the AI replies")
	}
	if err := tt.Click("作品分类"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if _, toast := tt.Find("AI 正在思考…"); a.page != 2 || !toast {
		t.Fatal("another page waits for the reply, or does not say it runs")
	}
	if err := tt.Click("讨论"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if _, toast := tt.Find("AI 正在思考…"); toast {
		t.Fatal("the talk replying is in view and the top says it again")
	}
	if err := tt.Click("当前模型"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("AI 配置…") {
		t.Fatal("the model waits for the reply", tt.Texts())
	}
	if err := tt.Click("当前模型"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("讨论输入"); err != nil {
		t.Fatal(err)
	}
	tt.Type("再说说")
	tt.Key(ui.Cmd, ui.KeyEnter)
	tt.Frame()
	if a.input != "再说说" || a.pending != "灯为什么会灭" || !tt.HasText("正在思考…") {
		t.Fatalf("typed %q, in flight %q", a.input, a.pending)
	}

	if err := tt.Click("对话-" + other); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	said := 0 // its words name its row in the list, and only that
	for _, s := range tt.Texts() {
		if strings.Contains(s, "灯为什么会灭") {
			said++
		}
	}
	if _, live := tt.Find("正在思考…"); a.session != other || live || said != 1 || !tt.HasText("该消息尚未收到回复") {
		t.Fatal("another talk shows the reply in flight", tt.Texts())
	}
	if _, toast := tt.Find("AI 正在思考…"); !toast {
		t.Fatal("another talk does not say one is replying", tt.Texts())
	}
	if err := tt.Click("重试回复"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.talkSession != session || a.busy != "AI 正在思考…" {
		t.Fatal("tried again as the AI replies")
	}
	close(block)
	settle(t, a, tt)
	tt.Frame()
	if a.session != other || tt.HasText("因为停电了。") || a.input != "再说说" {
		t.Fatal("the reply came to the talk shown", tt.Texts())
	}
	if err := tt.Click("对话-" + session); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("灯为什么会灭") || !tt.HasText("因为停电了。") || tt.HasText("该消息尚未收到回复") {
		t.Fatal("the reply did not land in its talk", tt.Texts())
	}
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

// Services are chosen by the names the author gave them, and kept by ID.
func TestChooseAServiceByName(t *testing.T) {
	a, tt, _ := fixture(t)
	if err := tt.Click("设置"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("设置-AI"); err != nil {
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
	if err := tt.Click("设置"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("设置-AI"); err != nil {
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
	if a.talkModel != "deepseek-v4-pro" {
		t.Fatal(a.talkModel, tt.Texts())
	}
	// The model is kept as the caret leaves it.
	tt.Key(0, ui.KeyTab)
	tt.Frame()
	tt.Frame()
	if a.Store.Work().Discussion.Model != "deepseek-v4-pro" || tt.HasText("deepseek-flash") {
		t.Fatal(a.Store.Work().Discussion, tt.Texts())
	}
	if err := tt.Click("讨论模型"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("flash")
	tt.Frame()
	// deepseek-v4-pro is still the model beside the chat input, not in the list.
	if !tt.HasText("deepseek-flash") || strings.Count(strings.Join(tt.Texts(), "\n"), "deepseek-v4-pro") != 1 {
		t.Fatal(tt.Texts())
	}
}

// A new talk is listed once something is said in it; until then, asking
// for another new talk gives the same one, so empty ones do not pile up.
// The list says the day, as the column beside the prose does.
func TestANewTalkIsListedOnceSomethingIsSaid(t *testing.T) {
	a, tt, _ := fixture(t)
	first := a.session
	tt.Frame()
	if _, ok := tt.Find("对话-" + first); ok {
		t.Fatal("an empty talk is listed", tt.Texts())
	}
	if err := tt.Click("新对话"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if ids, _ := a.Store.Sessions(); a.session != first || len(ids) != 1 {
		t.Fatal("another empty talk", ids)
	}
	if _, err := a.Store.Append(first, workspace.Event{Role: workspace.RoleUser, Text: "灯为什么会灭"}); err != nil {
		t.Fatal(err)
	}
	a.setError(a.refresh())
	tt.Frame()
	if _, ok := tt.Find("对话-" + first); !ok || !tt.HasText("今天") {
		t.Fatal("the talk is not listed by its day", tt.Texts())
	}
	if err := tt.Click("新对话"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if ids, _ := a.Store.Sessions(); a.session == first || len(ids) != 2 {
		t.Fatal("no new talk", ids)
	}
}

// Shelving a conversation takes it out of the list but keeps its file;
// the one open gives way to the latest left, and it can be put back.
func TestShelveAConversation(t *testing.T) {
	a, tt, _ := fixture(t)
	old := a.session
	if _, err := a.Store.Append(old, workspace.Event{Role: workspace.RoleUser, Text: "灯为什么会灭"}); err != nil {
		t.Fatal(err)
	}
	a.setError(a.refresh())
	a.newSession()
	open := a.session
	tt.Frame()
	if err := tt.RightClick("对话-" + old); err != nil {
		t.Fatal(err)
	}
	if err := pick(tt, "收起这段对话"); err != nil {
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
	if err := pick(tt, "移回列表"); err != nil {
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
