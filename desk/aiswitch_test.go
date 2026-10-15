package desk

import (
	"context"
	"errors"
	"image/png"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/ai"
	"writer/desk/aiport"
	"writer/shelf"
	"writer/workspace"
)

// desk draws all of the AI and sends nothing itself: what sends is in
// package ai, which desk reaches through desk/aiport alone.
func TestDeskImportsNothingThatSends(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", "writer/desk").CombinedOutput()
	if err != nil {
		t.Fatal(err, string(out))
	}
	deps := strings.Fields(string(out))
	if !slices.Contains(deps, "writer/desk/aiport") {
		t.Fatalf("not the dependencies of desk: %q", deps)
	}
	for _, p := range []string{"writer/ai", "writer/model", "writer/discussion", "writer/writing"} {
		if slices.Contains(deps, p) {
			t.Errorf("desk depends on %s", p)
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
	aiport.Service
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

func (s *spy) Catalog(provider string) (aiport.Catalog, error) {
	s.note("Catalog")
	return s.Service.Catalog(provider)
}

func (s *spy) Talk(st *workspace.Store, beside *aiport.Beside, progress func(reasoning, text string), step func(workspace.Event)) (aiport.Talk, error) {
	s.note("Talk")
	return s.Service.Talk(st, beside, progress, step)
}

func (s *spy) Pen(st *workspace.Store) (aiport.Pen, error) {
	s.note("Pen")
	return s.Service.Pen(st)
}

const aiProse = "整条街的灯是同时灭的。\n林婉站在巷口，手电在工具箱最底下。\n"

// aiWork is a work the AI has been at, in root/work: a chapter it drafted,
// with the snapshot from before, its summary and its check; a talk, which
// noted the setting and an entry it suggests. Of the author: an
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
	s, err := workspace.Create(filepath.Join(root, "work"), "雨城", "网文")
	must(err)
	_, err = s.Revise(workspace.Revision{By: workspace.ByAuthor, Ops: []workspace.Op{{Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: "人物", Text: "林晚怕黑", Names: []string{"林晚"}}}})
	must(err)
	chapter, err = s.NewChapter("", "停电", false)
	must(err)
	_, err = s.WriteChapter(chapter, aiProse, []workspace.ChapterMark{
		{Kind: workspace.MarkAIDraft, Start: 0, End: 11, Quote: "整条街的灯是同时灭的。", Note: "deepseek/deepseek-flash"},
		{Kind: workspace.MarkForeshadow, Quote: "一只旧灯泡", Orphaned: true, ID: workspace.NewID()},
	})
	must(err)
	must(s.SnapshotForAI(chapter, "", nil, workspace.AIRecord{Op: workspace.AIDraft, Instruction: "约 800 字", Model: "deepseek/deepseek-flash"}))
	hash, words := workspace.ProseHash(aiProse), workspace.Count(aiProse)
	must(s.WriteSummary(chapter, workspace.Summary{Text: "灯灭了，林婉走进巷子。", Of: hash, Words: words, By: "deepseek/deepseek-flash", At: time.Now()}))
	must(s.WriteCheck(chapter, workspace.Check{Of: hash, Words: words, By: "deepseek/deepseek-flash", At: time.Now(), Findings: []workspace.Finding{
		{ID: workspace.NewID(), Kind: workspace.FindName, Title: "「林婉」是不是「林晚」写错了？", Quote: "林婉", Name: "林晚"},
	}}))
	session, err := s.NewSession()
	must(err)
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
	quick(t)
	lib, was := filepath.Join(root, "library"), Library
	Library = func() string { return lib }
	t.Cleanup(func() { Library = was })
	sp := &spy{Service: ai.New(filepath.Join(root, "providers.yaml"))}
	a := New(sp)
	if err := a.Open(filepath.Join(root, "work")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.Close() })
	tt := ui.NewTester(a.View, 1266, 780)
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
	if err := tt.Click("关闭 AI"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	verb := "关闭 AI"
	if on {
		verb = "开启 AI"
	}
	if !tt.HasText("是否" + verb + "？") {
		t.Fatalf("not asked before the AI is turned %v: %q", on, tt.Texts())
	}
	if err := tt.Click("确定" + verb); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.aiOn() != on || a.bk.askAI || !a.bk.prefs {
		t.Fatalf("the AI is on: %v, asking: %v, the settings open: %v", a.aiOn(), a.bk.askAI, a.bk.prefs)
	}
	shut(tt, &a.bk.prefs)
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
	a.doc.talk = true // the talk beside the prose, kept for the work
	a.savePrefs()
	tt.Frame()
	if w.node != chapter || !a.doc.talk || len(aiSaid(tt)) == 0 {
		t.Fatalf("with the AI on, the page of prose tells nothing of it: %q", tt.Texts())
	}
	w.ed.MoveTo(2, false)
	if !w.ed.Marked() {
		t.Fatal("what the AI drafted is no mark with the AI on")
	}

	switchAI(t, a, tt, false)
	sp.forget()
	look := func(where string) {
		t.Helper()
		tt.Frame()
		if said := aiSaid(tt, root); len(said) > 0 {
			t.Errorf("with the AI off, %s tells of it: %q", where, said)
		}
	}
	look("the page of prose")
	if a.page != 3 || !a.doc.talk {
		t.Fatalf("page %d; the talk beside the prose not kept for the AI to come back", a.page)
	}
	if w.ed.Marked() {
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
	if !a.doc.history || !tt.HasText("林晚怕黑") || tt.HasText("雨从不停") {
		t.Errorf("with the AI off, the change log lists what a talk noted or not the author's: %q", tt.Texts())
	}
	if err := tt.Click("全部设定"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"文件", "编辑", "视图", "帮助"} {
		openMenu(t, a, tt, name)
		look("the menu " + name)
		a.menu = menuState{}
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
	w.ed.Select(0, 2)
	tt.Frame()
	if !tt.HasText("加入设定") {
		t.Fatalf("no bar over the words picked: %q", tt.Texts())
	}
	look("the bar over the words picked")
	tt.Key(ui.Cmd, ui.KeyL)
	w.ed.MoveTo(len([]rune(aiProse)), false)
	w.ed.Focus()
	tt.Frame()
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyJ)
	tt.Key(ui.Cmd, ui.KeyEnter)
	time.Sleep(50 * time.Millisecond)
	tt.Frame()
	if w.ed.Predicting() || w.ed.Ghost() != "" || len(a.quotes) > 0 || a.busy != "" {
		t.Errorf("a key of the AI did something with it off: predicting %v, %q, quotes %v, busy %q", w.ed.Predicting(), w.ed.Ghost(), a.quotes, a.busy)
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
	if err := clickMenu(tt, "文件", "菜单-快照"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !w.snapsOpen || !tt.HasText("改动前") {
		t.Fatalf("the snapshots do not show the one from before the draft: %q", tt.Texts())
	}
	look("the snapshots")
	shut(tt, &w.snapsOpen)

	// The chapter changed and left: no summary, no check.
	w.ed.Focus()
	tt.Frame()
	tt.Frame()
	tt.Type("雨还在下。")
	tt.Frame()
	if !strings.HasSuffix(w.ed.Doc.String(), "雨还在下。") {
		t.Fatalf("typed into %q", w.ed.Doc.String())
	}
	if err := tt.Click("作品分类"); err != nil {
		t.Fatal(err)
	}
	look("the categories")
	if len(a.sums.queue) > 0 || a.sums.running != "" || len(a.ck.queue) > 0 || a.ck.running != "" {
		t.Error("a chapter left with the AI off is to be summed up or checked")
	}
	a.closeWork()
	tt.Frame()
	if err := a.Open(filepath.Join(root, "work")); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if a.page != 3 {
		t.Errorf("with the AI off, the work opens on page %d", a.page)
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
	sum, _ := s.ReadSummary(chapter)
	check, _ := s.ReadCheck(chapter)
	if sum.Text == "" || len(check.Findings) != 1 {
		t.Errorf("what the AI left went with it: %+v %+v", sum, check)
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
// Off while the AI is at work, a draft keeps what it wrote and a reply is
// let go, the author's words kept; on again, the talk and the checks are
// back, and the reply waits to be tried again.
func TestTurnTheAIOffAndOn(t *testing.T) {
	a, _, _ := writingFixture(t, "")
	var c *ui.Context
	tt := ui.NewTester(func(ctx *ui.Context) { c = ctx; a.View(ctx) }, 1266, 780)
	tt.Frame()
	tt.Frame()
	w := &a.write
	config := a.ai.ConfigFile()
	noAI := func() bool {
		t.Helper()
		s, _, err := shelf.Load(shelf.File(config))
		if err != nil {
			t.Fatal(err)
		}
		return s.NoAI
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
	if !tt.HasText("提醒与检查") {
		t.Fatalf("with the AI on, the reminders are not 提醒与检查: %q", tt.Texts())
	}
	if err := tt.Click("设置-AI"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("关闭 AI"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.bk.askAI || !tt.HasText("是否关闭 AI？") {
		t.Fatalf("not asked: %q", tt.Texts())
	}
	if err := tt.Click("取消"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.bk.askAI || !a.bk.prefs || !a.aiOn() || noAI() {
		t.Fatal("取消 changed something")
	}
	if err := tt.Click("关闭 AI"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if a.bk.askAI || !a.bk.prefs || !a.aiOn() {
		t.Fatalf("Esc: asking %v, the settings open %v, the AI on %v", a.bk.askAI, a.bk.prefs, a.aiOn())
	}
	shut(tt, &a.bk.prefs)

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
	if _, ok := tt.Find("设置-提醒"); !ok || tt.HasText("提醒与检查") || tt.HasText("模型服务") {
		t.Fatalf("with the AI off, the settings of a work: %q", tt.Texts())
	}
	if err := tt.Click("设置-提醒"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if tt.HasText("检查") {
		t.Fatalf("with the AI off, the reminders tell of the checks: %q", tt.Texts())
	}
	shut(tt, &a.bk.prefs)
	b := newApp(config)
	if b.aiOn() {
		t.Fatal("the app starts with the AI on, the shelf saying off")
	}
	if _, err := b.ai.Catalog("deepseek"); !errors.Is(err, ai.ErrOff) {
		t.Fatalf("the app started with the AI off builds a client: %v", err)
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

	// Off while the AI drafts and replies.
	f := &stream{pieces: []string{"雨下了一夜。", "\n她醒了。"}, hold: make(chan struct{})}
	penWith(a, f)
	w.outline = "她在雨夜醒来。"
	tt.Frame()
	if err := clickMenu(tt, "编辑", "菜单-起草本章"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	w.words = "800"
	if err := tt.Click("开始起草"); err != nil {
		t.Fatal(err)
	}
	until(t, tt, "the first piece", func() bool { return proposed(a) == "雨下了一夜。" })
	words, err := a.Store.Append(a.session, workspace.Event{Role: workspace.RoleUser, Text: "灯为什么会灭"})
	if err != nil {
		t.Fatal(err)
	}
	a.pending = words.Text
	a.run(c, "AI 正在思考…", func(ctx context.Context) (func(), error) {
		<-ctx.Done()
		return func() { a.pending = "" }, errors.New("讨论失败，原文已保存")
	})
	a.talking, a.talkSession = true, a.session
	tt.Frame()
	switchAI(t, a, tt, false)
	until(t, tt, "the reply let go", func() bool { return a.busy == "" })
	tt.Frame()
	if _, _, _, ok := w.ed.Proposal(); ok || w.proposed != nil || w.run != nil || w.ed.ReadOnly() || w.ed.Doc.String() != "" {
		t.Fatalf("the draft was not let go: %q, running %v", w.ed.Doc.String(), w.run != nil)
	}
	if a.Error != "" || a.notice != "" || len(aiSaid(tt)) > 0 {
		t.Fatalf("turned off, the AI still tells of itself: %q %q %q", a.Error, a.notice, aiSaid(tt))
	}
	close(f.hold)
	time.Sleep(50 * time.Millisecond)
	tt.Frame()
	if _, _, _, ok := w.ed.Proposal(); ok || disk(t, a, "第一章.txt") != "" {
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
	if !tt.HasText("灯为什么会灭") || !tt.HasText("重试回复") {
		t.Fatalf("on again, the reply let go does not wait to be tried again: %q", tt.Texts())
	}
}

// TestShotAIOff writes screenshots of the switch and of the window with the
// AI off when SHOT is a directory: SHOT=dir go test ./desk -run ShotAIOff
func TestShotAIOff(t *testing.T) {
	dir := os.Getenv("SHOT")
	if dir == "" {
		t.Skip("set SHOT=dir")
	}
	root, _ := aiWork(t)
	a, tt, _ := openSpied(t, root)
	shot := func(name string) {
		t.Helper()
		tt.Frame()
		tt.Frame()
		f, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
	if err := tt.Click("正文"); err != nil {
		t.Fatal(err)
	}
	a.openPrefs("AI")
	a.bk.askAI = true
	shot("ai-off-ask.png")
	a.bk.askAI = false
	a.turnAI(false)
	shot("ai-off-settings.png")
	a.bk.askAI = true
	shot("ai-on-ask.png")
	a.bk.askAI, a.bk.prefs = false, false
	shot("ai-off-prose.png")
	tt.SetDark(true)
	shot("ai-off-prose-dark.png")
	tt.SetDark(false)
	a.closeWork()
	a.shelf.Books = nil
	shot("ai-off-shelf-empty.png")
}
