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
	"writer/ext/talk/talkapi"

	"github.com/egoist/mygo/ui"
	"writer/ai"
	"writer/desk/kit"
	"writer/editor"
	"writer/ext"
	aimod "writer/ext/ai"
	"writer/ext/ai/aiapi"
	"writer/ext/backup"
	"writer/ext/cats"
	contextmod "writer/ext/context"
	"writer/ext/context/contextapi"
	"writer/ext/export"
	findmod "writer/ext/find"
	"writer/ext/focus"
	"writer/ext/goal"
	"writer/ext/hooks"
	"writer/ext/lore"
	"writer/ext/marks"
	"writer/ext/names"
	"writer/ext/notes"
	"writer/ext/outline"
	"writer/ext/phone"
	"writer/ext/snaps"
	talkmod "writer/ext/talk"
	"writer/ext/trash"
	"writer/model"
	"writer/workspace"
)

// fixture opens a work whose setting holds one entry the author wrote.
func fixture(t *testing.T) (*App, *ui.Tester, workspace.Entry) {
	t.Helper()
	root := t.TempDir()
	s, err := workspace.Create(filepath.Join(root, "work"), "测试作品", "网文", "zh-Hans")
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
	t.Cleanup(func() { a.Close(); delete(controllers, a) })
	tt := stillTester(a.View, 1266, 780)
	tt.Frame()
	tt.Frame()
	e, _ := setting(a).Entry(r.Ops[0].Entry)
	return a, tt, e
}

// newApp is the app on the services configured in config, or on none.
func newApp(config string) *App { return testApp(ai.New(config)) }

func testModules() []ext.Module {
	mod, _ := backup.New(false)
	return []ext.Module{focus.Module, goal.Module, names.Module, findmod.Module, notes.Module, lore.Module, outline.Module, hooks.Module, talkmod.Module, snaps.Module, trash.Module, cats.Module, mod, export.Module, phone.Module, contextmod.Module, marks.Module}
}

func testApp(service aiapi.Backend) *App {
	mod, view := backup.New(false)
	outlineModule, page := outline.New()
	gm, gp := goal.New()
	cm, cp := cats.New()
	sm, sp := snaps.New()
	tm, tp := trash.New()
	fm, fp := findmod.New()
	a := newTestApp(service, focus.Module, gm, names.Module, fm, notes.Module, lore.Module, outlineModule, hooks.Module, talkmod.Module, sm, tm, cm, mod, export.Module, phone.Module, contextmod.Module, marks.Module)
	snapControllers[a] = sp
	trashControllers[a] = tp
	findControllers[a] = fp
	goalControllers[a] = gp
	catsControllers[a] = cp
	outlineControllers[a] = page
	controllers[a] = view
	return a
}

// aiOf is the AI of a, for a test to swap fake models in.
func aiOf(a *App) *ai.Service { return a.ai.(*ai.Service) }

func TestLeavingAWorkSavesItAndStopsItsChecks(t *testing.T) {
	for _, how := range []string{"attach", "shelf", "close", "busy close"} {
		t.Run(how, func(t *testing.T) {
			a, _, id := writingFixture(t, "旧稿。")
			store := a.Store
			shownEditor(a).Insert("新稿。")
			want := shownEditor(a).Doc.String()
			check, stopCheck := context.WithCancel(context.Background())
			defer stopCheck()
			a.ck.runner.cancel = stopCheck
			a.ck.queue.Enqueue(id)
			worker, stopWorker := context.WithCancel(context.Background())
			defer stopWorker()
			if how == "busy close" {
				a.cancel, a.busy = stopWorker, "正在回复"
			}
			switch how {
			case "attach":
				next, err := workspace.Create(filepath.Join(t.TempDir(), "next"), "另一本", "网文", "zh-Hans")
				if err != nil {
					t.Fatal(err)
				}
				if err = a.attach(next, next.Root(), false); err != nil {
					t.Fatal(err)
				}
				if a.Store != next {
					t.Fatal("new store was not attached")
				}
			case "shelf":
				a.closeWork()
				if a.Store != nil || a.path != "" || a.name != "" {
					t.Fatal("work did not return to shelf")
				}
			default:
				if err := a.Close(); err != nil {
					t.Fatal(err)
				}
			}
			if check.Err() != context.Canceled || len(a.ck.queue.Pending) != 0 {
				t.Fatal("checks were not stopped")
			}
			_, err := store.Chapters()
			if how == "busy close" {
				if worker.Err() != context.Canceled || err != nil {
					t.Fatal("busy worker lost its open store", worker.Err(), err)
				}
				if err = store.Close(); err != nil {
					t.Fatal(err)
				}
				a.busy, a.Store = "", nil
			} else if err == nil {
				t.Fatal("store was not closed")
			}
			reopened, err := workspace.Open(store.Root())
			if err != nil {
				t.Fatal(err)
			}
			defer reopened.Close()
			chapter, err := reopened.ReadChapter(id)
			if err != nil || chapter.Text != workspace.CleanProse(want) {
				t.Fatal("chapter was not saved", chapter.Text, err)
			}
		})
	}
}

func settle(t *testing.T, a *App, tt *ui.Tester) {
	t.Helper()
	for end := time.Now().Add(3 * time.Second); a.busy != ""; {
		if a.jobs.pending() {
			tt.Frame()
		} else {
			time.Sleep(time.Millisecond)
		}
		if time.Now().After(end) {
			t.Fatal("worker did not finish")
		}
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
	lorePanel(a).Doc.Text, lorePanel(a).Doc.Category = "主角怕安静", "关系"
	tt.Frame()
	if err := tt.Click("保存"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	got, _ := setting(a).Entry(e.ID)
	if got.Text != "主角怕安静" || got.Category != "关系" || !got.Author || editingEntry(a) != "" {
		t.Fatalf("edit %+v", got)
	}
	if err := tt.Click("删除设定"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got, _ = setting(a).Entry(e.ID); got.Place != workspace.PlaceDropped {
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
	if got, _ = setting(a).Entry(e.ID); got.Place != workspace.PlaceBody {
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
	lorePanel(a).Doc.Text = "雨从不停"
	if err := tt.Click("保存"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	// A new entry starts in the first category, as in the prototype.
	if len(setting(a).Entries) != 2 || setting(a).Entries[1].Category != "人物" {
		t.Fatalf("new entry %+v", setting(a).Entries)
	}
	for _, r := range setting(a).Revisions {
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
	tt = stillTester(func(ctx *ui.Context) { c = ctx; a.View(ctx) }, 1266, 780)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true}) // the fold opens at once
	tt.Frame()
	session := talkOf(a).Session
	block := make(chan struct{})
	var rev workspace.Revision
	var reply workspace.Event
	sv := c.Services()
	talkOf(a).Turn.Pending = "这座城市雨从不停。"
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
		talkOf(a).Stepped(talkOf(a).Turn, sv, call)
		found, err := a.Store.Append(session, workspace.Event{Role: workspace.RoleResult, ReplyTo: u.ID, Call: "c1", Text: "全书的正文里没有「雨」。"})
		if err != nil {
			return nil, err
		}
		talkOf(a).Stepped(talkOf(a).Turn, sv, found)
		if rev, err = a.Store.Revise(workspace.Revision{By: workspace.ByTalk, Session: session, Turn: u.ID, Model: "deepseek/deepseek-flash", Ops: []workspace.Op{
			{Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: "规则", Text: "雨从不停", Sources: []workspace.Source{{Session: session, Message: u.ID, Quote: "雨从不停"}}},
		}}); err != nil {
			return nil, err
		}
		noted, err := a.Store.Append(session, workspace.Event{Role: workspace.RoleResult, ReplyTo: u.ID, Call: "c2", Text: "已记下，编号 2：正文 · 规则：雨从不停", Revision: rev.ID})
		if err != nil {
			return nil, err
		}
		talkOf(a).Stepped(talkOf(a).Turn, sv, noted)
		<-block
		reply, err = a.Store.Append(session, workspace.Event{Role: workspace.RoleAssistant, ReplyTo: u.ID, Text: "那就让雨一直下。"})
		return nil, err
	})
	talkOf(a).Turn.Active, talkOf(a).Turn.Session = true, talkOf(a).Session
	until(t, tt, "the note", func() bool { return len(setting(a).Entries) == 2 })
	if _, ok := a.flash[setting(a).Entries[1].ID]; !ok || !tt.HasText("雨从不停") {
		t.Fatal("what the talk noted is not shown and tinted", tt.Texts())
	}
	if !tt.HasText("正在查阅…") || !tt.HasText("全书的正文里没有「雨」。") {
		t.Fatal("what the reply looks up is not open as it looks", tt.Texts())
	}
	close(block)
	settle(t, a, tt)
	talkOf(a).Turn.Active = false
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
	if shownEntry(a) != setting(a).Entries[1].ID {
		t.Fatal("the link does not open what was noted")
	}
	showEntry(a, "")
	showHistory(a, true)
	tt.Frame()
	if !tt.HasText("讨论") || !tt.HasText("新增到正文 · 规则：雨从不停") || !tt.HasText("记在「这座城市雨从不停。」") {
		t.Fatal("history", tt.Texts())
	}
	other, err := a.Store.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	talkOf(a).SwitchSession(other)
	tt.Frame()
	if err := tt.Click("讨论-" + rev.ID); err != nil {
		t.Fatal(err)
	}
	if talkOf(a).Session != session || shownPage(a) != pageTalk {
		t.Fatal("the link does not go back to the talk")
	}
}

func TestOldReadSummaryShowsItsSavedLabel(t *testing.T) {
	a, tt, _ := fixture(t)
	u, err := a.Store.Append(talkOf(a).Session, workspace.Event{Role: workspace.RoleUser, Text: "读一下旧摘要。"})
	if err != nil {
		t.Fatal(err)
	}
	const saved = "读了摘要「第三章：旧章名」"
	for _, event := range []workspace.Event{
		{Role: workspace.RoleCall, ReplyTo: u.ID, Calls: []workspace.Call{
			{ID: "old", Name: "read_summary", Input: []byte(`{"chapters":["第三章"]}`), Label: saved},
		}},
		{Role: workspace.RoleResult, ReplyTo: u.ID, Call: "old", Text: "旧摘要的内容。"},
		{Role: workspace.RoleAssistant, ReplyTo: u.ID, Text: "读完了。"},
	} {
		if _, err = a.Store.Append(talkOf(a).Session, event); err != nil {
			t.Fatal(err)
		}
	}
	if err = a.refresh(); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if !tt.HasText(saved) {
		t.Fatal("the old call lost its saved label", tt.Texts())
	}
	if err = tt.Click(saved); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	tt.Frame()
	if !tt.HasText(saved) || !tt.HasText("旧摘要的内容。") {
		t.Fatal("the old call does not unfold with its saved label", tt.Texts())
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
	if err := tt.Click("展开右栏"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.doc.hidden || !tt.HasText("主角怕黑") {
		t.Fatal("panel not shown again")
	}
}

func TestSessionTitles(t *testing.T) {
	a, tt, _ := fixture(t)
	if _, err := a.Store.Append(talkOf(a).Session, workspace.Event{Role: "user", Text: "雨城的故事\n第二行"}); err != nil {
		t.Fatal(err)
	}
	if err := a.refresh(); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if title, _ := talkOf(a).TalkName(talkOf(a).Session); !strings.HasSuffix(title, "雨城的故事") {
		t.Fatalf("title %q", title)
	}
}

func TestExtractionAddsTheAuthorsEntry(t *testing.T) {
	a, tt, _ := fixture(t)
	m, err := a.Store.Append(talkOf(a).Session, workspace.Event{Role: "user", Text: "林晚修路灯"})
	if err != nil {
		t.Fatal(err)
	}
	a.SetError(a.refresh())
	tt.Frame()
	talkOf(a).OpenSource(m, 0, 2)
	tt.Frame()
	if err = tt.Click("加入设定"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	last := setting(a).Entries[len(setting(a).Entries)-1]
	if last.Text != "林晚" || last.Place != workspace.PlaceBody || last.Sources[0].Message != m.ID || talkOf(a).SourceOpen {
		t.Fatalf("%+v", last)
	}
}

func TestCancellationKeepsUIResponsive(t *testing.T) {
	a, tt, _ := fixture(t)
	var viewContext *ui.Context
	tt = stillTester(func(c *ui.Context) { viewContext = c; a.View(c) }, 1266, 780)
	tt.Frame()
	a.run(viewContext, "测试中的请求", func(ctx context.Context) (func(), error) { <-ctx.Done(); return nil, ctx.Err() })
	tt.Frame()
	if err := tt.Click("取消请求"); err != nil {
		t.Fatal(err)
	}
	settle(t, a, tt)
	if a.busy != "" || a.cancel != nil || a.errorText() != context.Canceled.Error() {
		t.Fatalf("cancel result %q %q", a.busy, a.errorText())
	}
}

func TestFailureRefreshesPersistedWork(t *testing.T) {
	a, tt, _ := fixture(t)
	var c *ui.Context
	tt = stillTester(func(ctx *ui.Context) { c = ctx; a.View(ctx) }, 1266, 780)
	tt.Frame()
	a.run(c, "讨论", func(context.Context) (func(), error) {
		if _, err := a.Store.Append(talkOf(a).Session, workspace.Event{Role: "user", Text: "失败前已经落盘的发言"}); err != nil {
			return nil, err
		}
		return nil, errors.New("网络失败")
	})
	settle(t, a, tt)
	if len(talkOf(a).Events) != 1 || a.errorText() != "网络失败" {
		t.Fatalf("persisted partial result lost: %d %q", len(talkOf(a).Events), a.errorText())
	}
}

func TestNewWorkDoesNotInheritLastWork(t *testing.T) {
	a, _, _ := fixture(t)
	a.closeWork()
	if a.path != "" || a.name != "" {
		t.Fatalf("welcome form kept the last work: %q %q", a.path, a.name)
	}
	a.openCreate()
	a.books.at = filepath.Join(t.TempDir(), "雨城来信")
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
	a.SetError(a.Open(dir))
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
	if talkOf(a).Input != "接下来的情节怎么走" {
		t.Fatalf("%q", talkOf(a).Input)
	}
	a.openCreate()
	a.name, a.books.at = "新书", filepath.Join(t.TempDir(), "新书")
	if err := a.create(); err != nil || a.errorText() != "" {
		t.Fatal(err, a.errorText())
	}
	showPage(a, pageTalk)
	tt.Frame()
	if !tt.HasText("从一个初步的想法开始") || !tt.HasText("我想写一个总在下雨的城市") || tt.HasText("接着聊这部作品") {
		t.Fatal(tt.Texts())
	}
}

// A new work starts with the models of the work open, or of the one last
// opened; a work whose service is no longer set up gives none.
func TestNewWorkKeepsTheModels(t *testing.T) {
	a, _, _ := fixture(t)
	talkChoicesOf(a).TalkProvider, talkChoicesOf(a).TalkModel, talkChoicesOf(a).TalkEffort = "deepseek", "deepseek-flash", "high"
	a.PenProvider, a.PenModel = "deepseek", "deepseek-v4-pro"
	if err := saveModelChoices(a); err != nil {
		t.Fatal(err)
	}
	want := a.Store.Work()
	create := func(name string) workspace.Work {
		t.Helper()
		a.openCreate()
		a.name, a.books.at = name, filepath.Join(t.TempDir(), name)
		if err := a.create(); err != nil || a.errorText() != "" {
			t.Fatal(err, a.errorText())
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
	talkChoicesOf(a).TalkProvider, talkChoicesOf(a).TalkModel = "deepseek", ""
	a.PenProvider, a.PenModel = "deepseek", ""
	if err := saveModelChoices(a); err != nil || a.Store.Work().Discussion != was.Discussion {
		t.Fatalf("a service without a model: %v %+v", err, a.Store.Work().Discussion)
	}
	talkChoicesOf(a).TalkModel = "deepseek-flash"
	if err := saveModelChoices(a); err != nil || a.Store.Work().Discussion != was.Discussion {
		t.Fatalf("the writing's service without a model: %v %+v", err, a.Store.Work().Discussion)
	}
	a.PenModel = "deepseek-v4-pro"
	if err := saveModelChoices(a); err != nil || a.Store.Work().Writing.Model != "deepseek-v4-pro" {
		t.Fatalf("complete: %v %+v", err, a.Store.Work().Writing)
	}
	// A category without its target is not saved, and is told as what is
	// left to fill in, not as a failure.
	catsOf(a).Name, catsOf(a).Target = "伏笔", ""
	if err := catsOf(a).Save(); err != nil || hintOf(a) != "请填写落点，例如“设定库”或“大纲”" || len(a.Store.Work().Categories) != len(was.Categories) {
		t.Fatalf("category target: %v %q", err, hintOf(a))
	}
}

// A category can be saved once it has a name and a target; till then its
// button cannot be pressed, as the other forms' cannot.
func TestACategorySavesOnceFilledIn(t *testing.T) {
	a, tt, _ := fixture(t)
	if err := clickMenu(tt, "文件", "菜单-作品分类"); err != nil {
		t.Fatal(err)
	}
	was := len(a.Store.Work().Categories)
	catsOf(a).Name = "线索"
	tt.Frame()
	if err := tt.Click("保存分类"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if hintOf(a) != "" || len(a.Store.Work().Categories) != was {
		t.Fatalf("pressed with no target: %q", hintOf(a))
	}
	catsOf(a).Target = "大纲"
	tt.Frame()
	if err := tt.Click("保存分类"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(a.Store.Work().Categories) != was+1 || !tt.HasText("分类“线索”已保存") {
		t.Fatal(tt.Texts())
	}
	// Whether the name is a new one or one already there, the form says
	// before it is saved, and what the one there lands in now.
	catsOf(a).Name = "伏线"
	tt.Frame()
	if !tt.HasText("将添加新分类。") {
		t.Fatal("a new name is not told", tt.Texts())
	}
	if err := tt.Click("编辑-人物"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("将修改已有的分类“人物”，现在的落点是“设定库”。") {
		t.Fatal("an existing name is not told", tt.Texts())
	}
}

func TestMarkdownBlocks(t *testing.T) {
	src := "# 标题\n\n有**粗体**、*斜体*、`代码`和[链接](https://example.com)。\n\n- 一\n- 二\n\n1. 甲\n\n> 引用\n\n```\ncode\n```\n\n| 列 | 值 |\n|---|---|\n| 甲 | 1 |\n\n<script>x</script>\n\n---\n"
	a, _, _ := fixture(t)
	tt := stillTester(func(c *ui.Context) { talkOf(a).Markdown(c, src, kit.ReplyText) }, 720, 1200)
	for _, text := range []string{"标题", "粗体", "斜体", "代码", "链接", "一", "二", "甲", "引用", "code", "列", "值", "1", "<script>x</script>"} {
		if !tt.HasText(text) {
			t.Errorf("missing %q in %q", text, tt.Texts())
		}
	}
	if err := tt.Click("链接"); err != nil {
		t.Fatal(err)
	}
	if urls := tt.OpenedURLs(); len(urls) != 0 {
		t.Fatalf("reply opened %q", urls)
	}
}

// A turn in flight shows its reasoning, then its reply, as they stream.
func TestLiveTurnStreams(t *testing.T) {
	a, tt, _ := fixture(t)
	var c *ui.Context
	tt = stillTester(func(ctx *ui.Context) { c = ctx; a.View(ctx) }, 1266, 780)
	tt.Frame()
	block := make(chan struct{})
	talkOf(a).Turn.Pending = "她做什么工作？"
	a.run(c, "AI 正在思考", func(ctx context.Context) (func(), error) { <-block; return nil, nil })
	talkOf(a).Turn.Active, talkOf(a).Turn.Session = true, talkOf(a).Session
	talkOf(a).Turn.Live.Add(talkapi.Progress{Reasoning: "她怕黑，职业要和光有关"})
	tt.Frame()
	if !tt.HasText("她怕黑，职业要和光有关") || !tt.HasText("正在思考…") {
		t.Fatal("reasoning not streamed", tt.Texts())
	}
	talkOf(a).Turn.Live.Add(talkapi.Progress{Text: "可以让她**修灯**。"})
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
	tt := stillTester(func(ctx *ui.Context) { c = ctx; a.View(ctx) }, 1266, 780)
	session := talkOf(a).Session
	other, err := a.Store.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.Store.Append(other, workspace.Event{Role: workspace.RoleUser, Text: "井边的灯谁点的"}); err != nil {
		t.Fatal(err)
	}
	a.SetError(a.refresh())
	tt.Frame()
	block := make(chan struct{})
	talkOf(a).Turn.Pending = "灯为什么会灭"
	a.run(c, "AI 正在思考…", func(context.Context) (func(), error) {
		u, err := a.Store.Append(session, workspace.Event{Role: workspace.RoleUser, Text: "灯为什么会灭"})
		if err != nil {
			return nil, err
		}
		<-block
		_, err = a.Store.Append(session, workspace.Event{Role: workspace.RoleAssistant, ReplyTo: u.ID, Text: "因为停电了。"})
		return func() { talkOf(a).Turn.Pending = "" }, err
	})
	talkOf(a).Turn.Active, talkOf(a).Turn.Session = true, session
	tt.Frame()

	if err := tt.Click("切换作品"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if tt.HasText("新建作品…") {
		t.Fatal("the work can be switched as the AI replies")
	}
	if err := clickMenu(tt, "文件", "菜单-作品分类"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if _, toast := tt.Find("AI 正在思考…"); shownPage(a) != pageCategories || !toast {
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
	if talkOf(a).Input != "再说说" || talkOf(a).Turn.Pending != "灯为什么会灭" || !tt.HasText("正在思考…") {
		t.Fatalf("typed %q, in flight %q", talkOf(a).Input, talkOf(a).Turn.Pending)
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
	if _, live := tt.Find("正在思考…"); talkOf(a).Session != other || live || said != 1 || !tt.HasText("该消息尚未收到回复") {
		t.Fatal("another talk shows the reply in flight", tt.Texts())
	}
	if _, toast := tt.Find("AI 正在思考…"); !toast {
		t.Fatal("another talk does not say one is replying", tt.Texts())
	}
	if err := tt.Click("重试回复"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if talkOf(a).Turn.Session != session || a.busy != "AI 正在思考…" {
		t.Fatal("tried again as the AI replies")
	}
	close(block)
	settle(t, a, tt)
	tt.Frame()
	if talkOf(a).Session != other || tt.HasText("因为停电了。") || talkOf(a).Input != "再说说" {
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
	talkOf(a).SetTalk(func(s *workspace.Selection) { s.Provider, s.Model = "deepseek", "deepseek-flash" })
	talkOf(a).SetTalk(func(s *workspace.Selection) { s.Effort = string(model.EffortMax) })
	tt.Frame()
	if w := a.Store.Work().Discussion; w.Effort != "max" || w.Model != "deepseek-flash" || talkChoicesOf(a).TalkEffort != "max" {
		t.Fatalf("selection %+v", w)
	}
	if !tt.HasText("deepseek-flash") || !tt.HasText("Max") {
		t.Fatal("chip does not show model and effort", tt.Texts())
	}
}

// A reply's paragraphs, headings and lists are drawn as Markdown that the
// author selects across; Copy takes its Markdown, a code block's button
// its code.
func TestReplyTextSelectsAcrossBlocksAndCopies(t *testing.T) {
	a, tt, _ := fixture(t)
	reply := "第一段。\n\n## 小标题\n\n- 甲\n- 乙\n\n第二段。\n\n```\ncode\n```"
	u, err := a.Store.Append(talkOf(a).Session, workspace.Event{Role: workspace.RoleUser, Text: "说说"})
	if err != nil {
		t.Fatal(err)
	}
	r, err := a.Store.Append(talkOf(a).Session, workspace.Event{Role: workspace.RoleAssistant, ReplyTo: u.ID, Text: reply})
	if err != nil {
		t.Fatal(err)
	}
	a.SetError(a.refresh())
	tt.Frame()
	for _, s := range []string{"第一段。", "小标题", "甲", "第二段。", "code"} {
		if !tt.HasText(s) {
			t.Fatal(s, tt.Texts())
		}
	}
	if tt.HasText("## 小标题") || tt.HasText("```") {
		t.Fatal("the marks show", tt.Texts())
	}
	first, _ := tt.Find("第一段。")
	last, _ := tt.Find("第二段。")
	tt.Press(first.X+2, first.Y+first.H/2)
	tt.Move(last.X+last.W-2, last.Y+last.H/2)
	tt.Release(last.X+last.W-2, last.Y+last.H/2)
	tt.Key(ui.Ctrl, ui.KeyC)
	if got := tt.Clipboard(); !strings.Contains(got, "第一段。") || !strings.Contains(got, "甲") || !strings.Contains(got, "第二段") {
		t.Fatalf("a drag selected %q", got)
	}
	if err = tt.Click("复制代码"); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(tt.Clipboard()); got != "code" {
		t.Fatalf("the code block copied %q", got)
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
	if err := clickMenu(tt, "文件", "菜单-设置"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("设置-AI"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("续写和检查提供商"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	texts := tt.Texts()
	if options := strings.Join(texts[len(texts)-5:], " "); options != "和讨论相同 DeepSeek Kimi OpenAI Anthropic" {
		t.Fatal(options)
	}
	tt.Key(0, ui.KeyDown)
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if a.PenProvider != "deepseek" {
		t.Fatal(a.PenProvider)
	}
}

// A confirmation goes away on its own; an error stays until dismissed.
func TestNoticesFade(t *testing.T) {
	a, tt, _ := fixture(t)
	setNotice(a, "已保存")
	a.SetError(errors.New("出错了"))
	tt.Frame()
	if !tt.HasText("已保存") {
		t.Fatal(tt.Texts())
	}
	tt.Advance(noticeFor + time.Second) // the window's clock, not the wall's
	tt.Frame()
	if tt.HasText("已保存") {
		t.Fatal("the notice stayed")
	}
	if !tt.HasText("出错了") || noticeOf(a) != "" {
		t.Fatal(tt.Texts())
	}
}

// Clicking a model field lists the service's models; typing narrows them.
func TestPickAModelFromTheList(t *testing.T) {
	a, tt, _ := fixture(t)
	talkChoicesOf(a).TalkProvider, talkChoicesOf(a).TalkModel = "deepseek", "deepseek-flash"
	if err := clickMenu(tt, "文件", "菜单-设置"); err != nil {
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
	if talkChoicesOf(a).TalkModel != "deepseek-v4-pro" {
		t.Fatal(talkChoicesOf(a).TalkModel, tt.Texts())
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
	first := talkOf(a).Session
	tt.Frame()
	if _, ok := tt.Find("对话-" + first); ok {
		t.Fatal("an empty talk is listed", tt.Texts())
	}
	if err := tt.Click("新对话"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if ids, _ := a.Store.Sessions(); talkOf(a).Session != first || len(ids) != 1 {
		t.Fatal("another empty talk", ids)
	}
	if _, err := a.Store.Append(first, workspace.Event{Role: workspace.RoleUser, Text: "灯为什么会灭"}); err != nil {
		t.Fatal(err)
	}
	a.SetError(a.refresh())
	tt.Frame()
	if _, ok := tt.Find("对话-" + first); !ok || !tt.HasText("今天") {
		t.Fatal("the talk is not listed by its day", tt.Texts())
	}
	if err := tt.Click("新对话"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if ids, _ := a.Store.Sessions(); talkOf(a).Session == first || len(ids) != 2 {
		t.Fatal("no new talk", ids)
	}
}

// Shelving a conversation takes it out of the list but keeps its file;
// the one open gives way to the latest left, and it can be put back.
func TestShelveAConversation(t *testing.T) {
	a, tt, _ := fixture(t)
	old := talkOf(a).Session
	if _, err := a.Store.Append(old, workspace.Event{Role: workspace.RoleUser, Text: "灯为什么会灭"}); err != nil {
		t.Fatal(err)
	}
	a.SetError(a.refresh())
	talkOf(a).NewSession()
	open := talkOf(a).Session
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
	talkOf(a).Shelve(open)
	if talkOf(a).Session != old {
		t.Fatal("the open one stays", talkOf(a).Session)
	}
	talkOf(a).Shelve(old)
	if talkOf(a).Session == old || talkOf(a).Session == open {
		t.Fatal("no new conversation", talkOf(a).Session)
	}
}

var outlineControllers = map[*App]*outline.Page{}

func outlinePage(a *App) *outline.Page { return outlineControllers[a] }

func newTestApp(service aiapi.Backend, mods ...ext.Module) *App {
	var hookPanel *hooks.Panel
	var notePanel *notes.Panel
	mods = slices.Clone(mods)
	for i, m := range mods {
		switch m.ID {
		case "hooks":
			mods[i], hookPanel = hooks.New()
		case "notes":
			mods[i], notePanel = notes.New()
		}
	}
	view := &aimod.View{}
	mod := aimod.New(aimod.Options{View: view, ConfigFile: service.ConfigFile(), Factory: func() aiapi.Backend { return service }})
	a := New(service.ConfigFile(), append(append([]ext.Module(nil), mods...), mod)...)
	aiViews[a] = view
	if svc, ok := ext.Lookup(a.extensions, talkapi.Service); ok {
		talkControllers[a] = svc.(interface{ View() *talkmod.Panel }).View()
	}
	hookControllers[a] = hookPanel
	noteControllers[a] = notePanel
	return a
}

var aiViews = map[*App]*aimod.View{}

func aiViewOf(a *App) *aimod.View { return aiViews[a] }

func saveModelChoices(a *App) error {
	if talkChoicesOf(a).TalkProvider != "" && talkChoicesOf(a).TalkModel == "" || a.PenProvider != "" && a.PenModel == "" {
		return nil
	}
	if a.talkAPI != nil {
		if err := a.talkAPI.(interface{ SaveChoices() error }).SaveChoices(); err != nil {
			return err
		}
	}
	return aiViewOf(a).SaveUsage()
}

var talkControllers = map[*App]*talkmod.Panel{}

func talkOf(a *App) *talkmod.Panel {
	if p := talkControllers[a]; p != nil {
		return p
	}
	if svc, ok := ext.Lookup(a.extensions, talkapi.Service); ok {
		p := svc.(interface{ View() *talkmod.Panel }).View()
		talkControllers[a] = p
		return p
	}
	return nil
}
func talkChoicesOf(a *App) *talkapi.ModelChoices {
	if svc, ok := ext.Lookup(a.extensions, talkapi.Service); ok {
		return svc.(interface{ Choices() *talkapi.ModelChoices }).Choices()
	}
	return &talkapi.ModelChoices{}
}

var hookControllers = map[*App]*hooks.Panel{}
var noteControllers = map[*App]*notes.Panel{}

func hooksOf(a *App) *hooks.Panel                             { return hookControllers[a] }
func notesOf(a *App) *notes.Panel                             { return noteControllers[a] }
func (a *App) openBury()                                      { hooksOf(a).OpenBury() }
func (a *App) openPay(id string)                              { hooksOf(a).OpenPay(id) }
func (a *App) pay(id string)                                  { hooksOf(a).Pay(id) }
func (a *App) renoteFore(f workspace.Foreshadow, note string) { hooksOf(a).Renote(f, note) }
func (a *App) dropFore(f workspace.Foreshadow, dropped bool)  { hooksOf(a).Drop(f, dropped) }
func (a *App) rehang(f workspace.Foreshadow)                  { hooksOf(a).Rehang(f) }
func (a *App) showFore(id string)                             { hooksOf(a).Show(id) }

func (a *App) bury() { hooksOf(a).Bury() }

func (a *App) foreshadows() []workspace.Foreshadow {
	if a.extensions == nil {
		return nil
	}
	service, ok := ext.Lookup(a.extensions, contextapi.Service)
	if !ok {
		return nil
	}
	var out []workspace.Foreshadow
	_ = ext.Guard(a.extensions, "context", "context.hooks", func() { out, _ = service.Hooks() })
	return out
}

// paneKey is everything the right column's tab counts read.
type paneKey struct {
	fores checkKey
	ai    bool
	late  int
}

type paneNums struct {
	waiting, open int
}

func (a *App) paneCounts() paneNums {
	a.syncExtensions()
	n := paneNums{open: len(a.ideas())}
	for _, item := range a.extViews.panes {
		if item.Owner == "hooks" && item.Value.Count != nil {
			_ = ext.Guard(a.extensions, item.Owner, item.ID, func() { n.waiting = item.Value.Count() })
		}
	}
	return n
}

func (a *App) entities() []editor.Entity { return a.nameValues().Entities }
