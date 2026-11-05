package desk

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/ai"
	"writer/desk/kit"
	"writer/ext/ai/aiapi"
	"writer/internal/testshot"
	"writer/workspace"
)

func TestBuildersReadTheShelfSwitch(t *testing.T) {
	a := newApp("")
	st, err := workspace.Create(t.TempDir(), "雨城", "网文", "zh-Hans")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	for _, off := range []bool{true, false, true} {
		a.turnAI(!off)
		for name, build := range map[string]func() error{
			"catalog": func() error { _, err := a.ai.Catalog("missing"); return err },
			"talk": func() error {
				_, err := a.ai.Model(st, aiapi.RoleTalk)
				return err
			},
			"pen": func() error { _, err := a.ai.Pen(st, aiapi.Material{Setting: true, Outline: true}); return err },
		} {
			if err := build(); errors.Is(err, ai.ErrOff) != off {
				t.Errorf("%s: shelf.NoAI=%v: %v", name, off, err)
			}
		}
	}
}

// aiWords tell of the AI: with it off, none shows in the window, what the
// author wrote aside.
var aiWords = []string{"AI", "讨论", "检查", "续写", "改写", "起草", "摘要", "模型", "整理"}

// aiSaid is what the window tells of the AI, in its texts and the names of
// what is in it; 连续写作 is no 续写, and the folders a test made are named
// after it, as skip says.
func aiSaid(tt *ui.Tester, skip ...string) []string {
	var out []string
	for _, s := range tt.Texts() {
		said := strings.ReplaceAll(s, "连续写", "")
		for _, k := range skip {
			said = strings.ReplaceAll(said, k, "")
		}
		for _, w := range aiWords {
			if strings.Contains(said, w) {
				out = append(out, s)
				break
			}
		}
	}
	return out
}

// spy is the AI of the machine, noting what it is asked to build that
// sends.
type spy struct {
	aiapi.Backend
	mu    sync.Mutex
	asked []string
}

func (s *spy) note(what string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.asked = append(s.asked, what)
}

func (s *spy) heard() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.asked)
}

func (s *spy) forget() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.asked = nil
}

func (s *spy) Catalog(provider string) (aiapi.Catalog, error) {
	s.note("Catalog")
	return s.Backend.Catalog(provider)
}

func (s *spy) Model(st *workspace.Store, role aiapi.Role) (aiapi.Handle, error) {
	s.note("Talk")
	return s.Backend.Model(st, role)
}

func (s *spy) Pen(st *workspace.Store, material aiapi.Material) (aiapi.Pen, error) {
	s.note("Pen")
	return s.Backend.Pen(st, material)
}

const aiProse = "整条街的灯是同时灭的。\n林婉站在巷口，手电在工具箱最底下。\n"

// oldSummary is a chapter's summary as versions before wrote them.
const oldSummary = `{"text":"灯灭了，林婉走进巷子。","by":"deepseek/deepseek-flash"}`

// aiWork is a work the AI has been at, in root/work: a chapter it drafted,
// its check and a summary an older version wrote; a talk, which drafted
// the chapter, with the snapshot from before, and noted the setting and an
// entry it suggests. Of the author: an
// entry, and a foreshadowing whose sentence is gone, which waits on them.
// Neither the prose nor the names of the work and the chapter tell of the
// AI.
func aiWork(t *testing.T) (root, chapter string) {
	t.Helper()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	root = t.TempDir()
	s, err := workspace.Create(filepath.Join(root, "work"), "雨城", "网文", "zh-Hans")
	must(err)
	_, err = s.Revise(workspace.Revision{By: workspace.ByAuthor, Ops: []workspace.Op{{Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: "人物", Text: "林晚怕黑", Names: []string{"林晚"}}}})
	must(err)
	chapter, err = s.NewChapter("", "停电", false)
	must(err)
	_, err = s.WriteChapter(chapter, aiProse, []workspace.ChapterMark{
		{Kind: workspace.MarkAI, Start: 0, End: 11, Quote: "整条街的灯是同时灭的。", Note: "deepseek/deepseek-flash"},
		{Kind: workspace.MarkForeshadow, Quote: "一只旧灯泡", Orphaned: true, ID: workspace.NewID()},
	})
	must(err)
	hash, words := workspace.ProseHash(aiProse), workspace.Chinese.Count(aiProse)
	must(os.WriteFile(filepath.Join(root, "work", "chapters", "停电.summary.json"), []byte(oldSummary), 0o644))
	must(s.WriteCheck(chapter, workspace.Check{Of: hash, Words: words, By: "deepseek/deepseek-flash", At: time.Now(), Findings: []workspace.Finding{
		{ID: workspace.NewID(), Kind: workspace.FindName, Title: "「林婉」是不是「林晚」写错了？", Quote: "林婉", Name: "林晚"},
	}}))
	session, err := s.NewSession()
	must(err)
	must(s.SnapshotForTalks(chapter, "", nil, []string{session}))
	u, err := s.Append(session, workspace.Event{Role: workspace.RoleUser, Text: "灯为什么会灭"})
	must(err)
	_, err = s.Append(session, workspace.Event{Role: workspace.RoleCall, ReplyTo: u.ID, Calls: []workspace.Call{
		{ID: "c1", Name: "note_setting", Input: []byte(`{"op":"add"}`), Label: "记下：正文 · 规则：雨从不停"}}})
	must(err)
	for _, op := range []workspace.Op{
		{Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: "规则", Text: "雨从不停"},
		{Op: workspace.OpAdd, Place: workspace.PlaceOpen, Category: "人物", Text: "老周修了三十年路灯", Names: []string{"老周"}},
	} {
		_, err = s.Revise(workspace.Revision{By: workspace.ByTalk, Session: session, Turn: u.ID, Model: "deepseek/deepseek-flash", Ops: []workspace.Op{op}})
		must(err)
	}
	_, err = s.Append(session, workspace.Event{Role: workspace.RoleAssistant, ReplyTo: u.ID, Text: "因为停电。"})
	must(err)
	s.Close()
	return root, chapter
}

// openSpied opens root/work with the services configured beside it, on
// the AI of this machine with a spy on it.
func openSpied(t *testing.T, root string) (*App, *ui.Tester, *spy) {
	t.Helper()
	lib, was := filepath.Join(root, "library"), Library
	Library = func() string { return lib }
	t.Cleanup(func() { Library = was })
	sp := &spy{Backend: ai.New(filepath.Join(root, "providers.yaml"))}
	a := testApp(sp)
	if err := a.Open(filepath.Join(root, "work")); err != nil {
		t.Fatal(err)
	}
	quick(a)
	t.Cleanup(func() { a.Close() })
	tt := stillTester(a.View, 1266, 780)
	tt.Frame()
	tt.Frame()
	return a, tt, sp
}

// switchAI turns the AI on or off as the author does: in the settings,
// answering the question.
func switchAI(t *testing.T, a *App, tt *ui.Tester, on bool) {
	t.Helper()
	if err := clickMenu(tt, "文件", "菜单-设置"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("设置-AI"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("扩展开关-ai"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	verb, title := "关闭 AI", "关闭 AI？"
	if on {
		verb, title = "开启 AI", "打开 AI？"
	}
	if !tt.HasText(title) {
		t.Fatalf("not asked before the AI is turned %v: %q", on, tt.Texts())
	}
	if err := tt.Click("确定" + verb); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.aiOn() != on || a.settings.askAI || !a.settings.open {
		t.Fatalf("the AI is on: %v, asking: %v, the settings open: %v", a.aiOn(), a.settings.askAI, a.settings.open)
	}
	shut(tt, &a.settings.open)
}

// With the AI off, nothing in the window tells of it or offers it, the
// prose aside, and nothing that sends is built: not on the page of prose
// and what opens from it, nor in the column beside it, the menus, the
// shortcuts, what waits, the snapshots, the categories, the work opened
// again, or the shelf, with works and with none. Ctrl+J, Ctrl+L and
// Ctrl+Enter do nothing. What the AI left stays where it was.
func TestAIOffShowsNothingOfIt(t *testing.T) {
	root, chapter := aiWork(t)
	a, tt, sp := openSpied(t, root)
	w := &a.write
	if err := tt.Click("正文"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	showTab(a, paneTalk) // the talk beside the prose, kept for the work
	a.savePrefs()
	tt.Frame()
	if shownChapter(a) != chapter || shownTab(a) != paneTalk || len(aiSaid(tt)) == 0 {
		t.Fatalf("with the AI on, the page of prose tells nothing of it: %q", tt.Texts())
	}
	shownEditor(a).MoveTo(2, false)
	if !shownEditor(a).Marked() {
		t.Fatal("what the AI drafted is no mark with the AI on")
	}

	switchAI(t, a, tt, false)
	sp.forget()
	look := func(where string) {
		t.Helper()
		tt.Frame()
		if said := aiSaid(tt, root, filepath.Dir(root)); len(said) > 0 {
			t.Errorf("with the AI off, %s tells of it: %q", where, said)
		}
	}
	look("the page of prose")
	if shownPage(a) != pageWrite || shownTab(a) != paneTalk {
		t.Fatalf("page %d; the talk beside the prose not kept for the AI to come back", shownPage(a))
	}
	if shownEditor(a).Marked() {
		t.Error("what the AI drafted is a mark at the caret with the AI off")
	}
	if err := tt.Click("伏笔"); err != nil {
		t.Fatal(err)
	}
	look("the foreshadowings")
	if err := tt.Click("设定"); err != nil {
		t.Fatal(err)
	}
	look("the setting")
	if err := tt.Click("更多"); err != nil {
		t.Fatal(err)
	}
	if err := pick(tt, "修改记录"); err != nil {
		t.Fatal(err, rows(tt))
	}
	look("the change log")
	if !historyShown(a) || !tt.HasText("林晚怕黑") || tt.HasText("雨从不停") {
		t.Errorf("with the AI off, the change log lists what a talk noted or not the author's: %q", tt.Texts())
	}
	if err := tt.Click("全部设定"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"文件", "编辑", "视图", "帮助"} {
		openMenu(t, a, tt, name)
		look("the menu " + name)
		a.menu = kit.MenuState{}
		tt.Frame()
	}
	tt.RightClickAt(550, 300)
	tt.Frame()
	tt.Frame()
	if len(rows(tt)) == 0 {
		t.Fatal("the menu of the prose did not open")
	}
	look("the menu of the prose")
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	shownEditor(a).Select(0, 2)
	tt.Frame()
	if !tt.HasText("加入设定") {
		t.Fatalf("no bar over the words picked: %q", tt.Texts())
	}
	look("the bar over the words picked")
	tt.Key(ui.Cmd, ui.KeyL)
	shownEditor(a).MoveTo(len([]rune(aiProse)), false)
	shownEditor(a).Focus()
	tt.Frame()
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyJ)
	tt.Key(ui.Cmd, ui.KeyEnter)
	time.Sleep(50 * time.Millisecond)
	tt.Frame()
	if shownEditor(a).Predicting() || shownEditor(a).Ghost() != "" || len(talkOf(a).Quotes) > 0 || a.busy != "" {
		t.Errorf("a key of the AI did something with it off: predicting %v, %q, quotes %v, busy %q", shownEditor(a).Predicting(), shownEditor(a).Ghost(), talkOf(a).Quotes, a.busy)
	}
	tt.Key(ui.Cmd, ui.KeySlash)
	tt.Frame()
	if !a.keysOpen || !tt.HasText("打开快捷键列表") {
		t.Fatal("the shortcuts did not open")
	}
	look("the shortcuts")
	shut(tt, &a.keysOpen)
	if err := tt.Click("待处理"); err != nil {
		t.Fatal(err, tt.Texts())
	}
	tt.Frame()
	if !tt.HasText("原句已删") {
		t.Fatalf("what waits is not shown: %q", tt.Texts())
	}
	look("what waits")
	w.pendOpen = false
	tt.Frame()
	if err := clickMenu(tt, "文件", "菜单-历史版本"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !snapsOf(a).SnapsOpen || !tt.HasText("改动前") {
		t.Fatalf("the snapshots do not show the one from before the draft: %q", tt.Texts())
	}
	look("the snapshots")
	shut(tt, &snapsOf(a).SnapsOpen)

	// The chapter changed and left: no check.
	shownEditor(a).Focus()
	tt.Frame()
	tt.Frame()
	tt.Type("雨还在下。")
	tt.Frame()
	if !strings.HasSuffix(shownEditor(a).Doc.String(), "雨还在下。") {
		t.Fatalf("typed into %q", shownEditor(a).Doc.String())
	}
	if err := clickMenu(tt, "文件", "菜单-作品分类"); err != nil {
		t.Fatal(err)
	}
	look("the categories")
	if len(checkOf(a).State.Queue.Pending) > 0 || checkOf(a).State.Queue.Running != "" {
		t.Error("a chapter left with the AI off is to be checked")
	}
	a.closeWork()
	tt.Frame()
	if err := a.Open(filepath.Join(root, "work")); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if shownPage(a) != pageWrite {
		t.Errorf("with the AI off, the work opens on page %d", shownPage(a))
	}
	look("the work opened again")
	a.closeWork()
	look("the shelf")
	a.shelf.Books = nil
	look("the empty shelf")
	if !tt.HasText("新建一部作品，或添加已有的作品文件夹。") {
		t.Errorf("the empty shelf: %q", tt.Texts())
	}
	if asked := sp.heard(); len(asked) > 0 {
		t.Errorf("with the AI off, it was asked to build %q", asked)
	}
	s, err := workspace.Open(filepath.Join(root, "work"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	sum, _ := os.ReadFile(filepath.Join(root, "work", "chapters", "停电.summary.json"))
	check, _ := s.ReadCheck(chapter)
	if string(sum) != oldSummary || len(check.Findings) != 1 {
		t.Errorf("what the AI left went with it: %q %+v", sum, check)
	}
}

// files is what the folder at root holds, by path.
func files(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(p)
		out[p] = string(b)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// The switch asks first, both ways; 取消 and Esc change nothing, Esc
// putting the question away alone. It holds for the machine: kept in the
// shelf, and read again as the app starts, a shelf without it being on.
// Off and on again, nothing in the work's folder or the services changed.
// Off while the AI drafts in a talk, what it proposed is let go and the
// reply with it, the author's words kept; on again, the talk and the
// checks are back, and the reply waits to be tried again.
func TestTurnTheAIOffAndOn(t *testing.T) {
	a, tt, _ := writingFixture(t, "")
	config := a.ai.ConfigFile()
	noAI := func() bool {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(filepath.Dir(config), "ext.json"))
		if os.IsNotExist(err) {
			return false
		}
		if err != nil {
			t.Fatal(err)
		}
		return strings.Contains(string(data), `"ai"`)
	}

	if err := clickMenu(tt, "文件", "菜单-设置"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	for _, tab := range []string{"设置-AI", "设置-外观", "设置-提醒", "设置-备份"} {
		if _, ok := tt.Find(tab); !ok {
			t.Fatalf("the settings of a work have no %s: %q", tab, tt.Texts())
		}
	}
	if !tt.HasText("提醒") {
		t.Fatalf("with the AI on, the reminders are not 提醒: %q", tt.Texts())
	}
	if err := tt.Click("设置-AI"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("扩展开关-ai"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.settings.askAI || !tt.HasText("关闭 AI？") {
		t.Fatalf("not asked: %q", tt.Texts())
	}
	if err := tt.Click("取消"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.settings.askAI || !a.settings.open || !a.aiOn() || noAI() {
		t.Fatal("取消 changed something")
	}
	if err := tt.Click("扩展开关-ai"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if a.settings.askAI || !a.settings.open || !a.aiOn() {
		t.Fatalf("Esc: asking %v, the settings open %v, the AI on %v", a.settings.askAI, a.settings.open, a.aiOn())
	}
	shut(tt, &a.settings.open)

	// Off and on again, with nothing at work: the work's folder and the
	// services stay as they were.
	before := files(t, a.Store.Root())
	services, _ := os.ReadFile(config)
	switchAI(t, a, tt, false)
	if !noAI() {
		t.Fatal("the switch is not kept in the shelf")
	}
	if err := clickMenu(tt, "文件", "菜单-设置"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if _, ok := tt.Find("设置-提醒"); !ok || !tt.HasText("提醒") || tt.HasText("AI 服务") {
		t.Fatalf("with the AI off, the settings of a work: %q", tt.Texts())
	}
	if err := tt.Click("设置-提醒"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	// Check's header stays, saying it waits for the AI, as the prototype's
	// extHead does; its settings are gone.
	if !tt.HasText("AI已关闭，暂时用不了") || tt.HasText("何时检查") {
		t.Fatalf("with the AI off, the reminders show check's settings: %q", tt.Texts())
	}
	shut(tt, &a.settings.open)
	b := newApp(config)
	if b.aiOn() {
		t.Fatal("the app starts with the AI on, the shelf saying off")
	}
	if _, ok := b.aiRef.Get(); ok {
		t.Fatal("AI service exists while off")
	}

	switchAI(t, a, tt, true)
	if noAI() {
		t.Fatal("the shelf still has the AI off")
	}
	if now, _ := os.ReadFile(config); string(now) != string(services) {
		t.Error("the services changed off and on")
	}
	after := files(t, a.Store.Root())
	for p, b := range before {
		if after[p] != b {
			t.Errorf("%s changed off and on", p)
		}
	}
	for p := range after {
		if _, ok := before[p]; !ok {
			t.Errorf("%s was written off and on", p)
		}
	}

	// Off while the AI drafts.
	f := &script{}
	penWith(a, f)
	hold := f.holding()
	f.then(proposal("第一章", "end", "", "雨下了一夜。\n她醒了。"))
	setChapterOutline(a, "她在雨夜醒来。")
	tt.Frame()
	if err := clickMenu(tt, "编辑", "菜单-起草本章"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	draftOf(a).Words = "800"
	if err := tt.Click("开始起草"); err != nil {
		t.Fatal(err)
	}
	until(t, tt, "the first piece", func() bool { return slices.Equal(shownProposals(a), []string{"雨下了一夜"}) })
	switchAI(t, a, tt, false)
	until(t, tt, "the reply let go", func() bool { return a.busy == "" })
	tt.Frame()
	if len(shownEditor(a).Proposals()) != 0 || len(proposeOf(a).Props) != 0 || shownEditor(a).ReadOnly() || shownEditor(a).Doc.String() != "" {
		t.Fatalf("the draft was not let go: %q", shownEditor(a).Doc.String())
	}
	if a.errorText() != "" || noticeOf(a) != "" || len(aiSaid(tt)) > 0 {
		t.Fatalf("turned off, the AI still tells of itself: %q %q %q", a.errorText(), noticeOf(a), aiSaid(tt))
	}
	close(hold)
	time.Sleep(50 * time.Millisecond)
	tt.Frame()
	if len(shownEditor(a).Proposals()) != 0 || len(proposeOf(a).Props) != 0 || disk(t, a, "第一章.txt") != "" {
		t.Fatalf("the draft went on with the AI off: %q", disk(t, a, "第一章.txt"))
	}
	switchAI(t, a, tt, true)
	for _, page := range []string{"讨论", "检查"} {
		if _, ok := tt.Find(page); !ok {
			t.Fatalf("on again, no page %s: %q", page, tt.Texts())
		}
	}
	if err := tt.Click("讨论"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("按章纲起草本章，约 800 字") || !tt.HasText("重试回复") {
		t.Fatalf("on again, the reply let go does not wait to be tried again: %q", tt.Texts())
	}
}

func TestAISwitchCallsPartsInOrder(t *testing.T) {
	a, _, _ := writingFixture(t, "雨停了。")
	checkOf(a).State.Queue.Enqueue("waiting")
	draftOf(a).Open = true
	proposeOf(a).Rewriting = true
	showTodo(a, true)
	a.SetError(errors.New("AI failed"))
	a.turnAI(false)
	if len(checkOf(a).State.Queue.Pending) > 0 || draftOf(a).Open || proposeOf(a).Rewriting || todoShown(a) || appError(a) != nil {
		t.Fatal("AI dependent state survived")
	}
}

// TestShotAIOff writes screenshots of the switch and of the window with the
// AI off when SHOT is a directory: SHOT=dir go test ./desk -run ShotAIOff
// Turned off on the talk or check page, the AI gives way to the prose;
// on again while the prose shows, that page is back, and not after the
// author went elsewhere.
func TestTheAIComesBackToItsPage(t *testing.T) {
	a, tt, _ := fixture(t)
	for _, from := range []page{pageTalk, pageCheck} {
		showPage(a, from)
		tt.Frame()
		switchAI(t, a, tt, false)
		if shownPage(a) != pageWrite {
			t.Fatalf("off on page %d, page %d shows", from, shownPage(a))
		}
		switchAI(t, a, tt, true)
		if shownPage(a) != from {
			t.Fatalf("on again, page %d shows, not %d", shownPage(a), from)
		}
	}
	showPage(a, pageTalk)
	tt.Frame()
	switchAI(t, a, tt, false)
	showPage(a, pageCategories)
	switchAI(t, a, tt, true)
	if shownPage(a) != pageCategories {
		t.Fatalf("on again, page %d took the place of the categories", shownPage(a))
	}
	showPage(a, pageWrite)
	switchAI(t, a, tt, false)
	switchAI(t, a, tt, true)
	if shownPage(a) != pageWrite {
		t.Fatalf("off and on on the prose, page %d shows", shownPage(a))
	}
}

func TestShotAIOff(t *testing.T) {
	testshot.Dir(t)
	root, _ := aiWork(t)
	a, tt, _ := openSpied(t, root)

	if err := tt.Click("正文"); err != nil {
		t.Fatal(err)
	}
	a.openPrefs("AI")
	a.settings.askAI = true
	shot(t, tt, "ai-off-ask.png")
	a.settings.askAI = false
	a.turnAI(false)
	shot(t, tt, "ai-off-settings.png")
	a.settings.askAI = true
	shot(t, tt, "ai-on-ask.png")
	a.settings.askAI, a.settings.open = false, false
	shot(t, tt, "ai-off-prose.png")
	tt.SetDark(true)
	shot(t, tt, "ai-off-prose-dark.png")
	tt.SetDark(false)
	a.closeWork()
	a.shelf.Books = nil
	shot(t, tt, "ai-off-shelf-empty.png")
}
