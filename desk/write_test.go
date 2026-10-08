package desk

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/editor"
	"writer/workspace"
)

// quick makes the writing page save at once and look at the disk only
// when a test asks.
func quick(t *testing.T) {
	saved, checked := saveAfter, checkEvery
	saveAfter, checkEvery = 0, time.Hour
	t.Cleanup(func() { saveAfter, checkEvery = saved, checked })
}

// writingFixture opens the writing page of a work holding a chapter of
// text.
func writingFixture(t *testing.T, text string) (*App, *ui.Tester, string) {
	t.Helper()
	quick(t)
	a, tt, _ := fixture(t)
	id, err := a.Store.NewChapter("", "第一章", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.Store.WriteChapter(id, text, nil); err != nil {
		t.Fatal(err)
	}
	if err = tt.Click("正文"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.write.node != id || a.write.ed == nil {
		t.Fatal("the chapter did not open", tt.Texts())
	}
	return a, tt, id
}

func disk(t *testing.T, a *App, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(a.Store.Root(), "chapters", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// The author makes a chapter, names it and writes: the text is on disk
// once they pause, with no saving to do.
func TestWritingSavesAsTheAuthorPauses(t *testing.T) {
	quick(t)
	a, tt, _ := fixture(t)
	if err := tt.Click("正文"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("还没有章节") {
		t.Fatal(tt.Texts())
	}
	if err := tt.Click("新建第一章"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	a.write.title = "第一章：雨夜"
	if err := tt.Click("确定标题"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("第一章：雨夜") {
		t.Fatal("the title did not take", tt.Texts())
	}
	a.write.ed.Focus()
	tt.Frame()
	tt.Type("雨下了一夜。")
	tt.Key(0, ui.KeyEnter)
	tt.Type("她没有回答。")
	tt.Frame()
	if got := disk(t, a, "第一章：雨夜.txt"); got != "雨下了一夜。\n她没有回答。" {
		t.Fatalf("on disk %q", got)
	}
	if !tt.HasText("本章 12 字 · 全书 12 字") {
		t.Fatal(tt.Texts())
	}
	a.write.outlineOpen = true
	tt.Frame()
	a.write.outline = "她在雨夜里等人"
	tt.Frame()
	if got := disk(t, a, "第一章：雨夜.outline.txt"); got != "她在雨夜里等人" {
		t.Fatalf("outline %q", got)
	}
	// Coming back opens the chapter last written.
	tt.Click("讨论")
	tt.Frame()
	b := NewWithConfig(a.ConfigFile)
	if err := b.Open(a.Store.Root()); err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	bt := ui.NewTester(b.View, 1266, 780)
	bt.Frame()
	bt.Click("正文")
	bt.Frame()
	if b.write.ed == nil || b.write.ed.Doc.String() != "雨下了一夜。\n她没有回答。" {
		t.Fatal("the chapter last open did not open again")
	}
}

// A chapter changed outside while open asks which side stays; the other
// becomes a snapshot.
func TestOutsideChangeAsksTheAuthor(t *testing.T) {
	a, tt, id := writingFixture(t, "原来的。")
	p := filepath.Join(a.Store.Root(), "chapters", "第一章.txt")
	if err := os.WriteFile(p, []byte("　　别处改的。"), 0644); err != nil {
		t.Fatal(err)
	}
	a.write.checked = time.Time{}
	tt.Frame()
	if !tt.HasText("磁盘上的这一章被别的程序改过了。保留哪一边？另一边会存成快照，不会丢。") {
		t.Fatal(tt.Texts())
	}
	if err := tt.Click("用磁盘上的"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := a.write.ed.Doc.String(); got != "别处改的。" {
		t.Fatalf("editor %q", got)
	}
	// The snapshot of opening it already holds the editor's text.
	snaps, _ := a.Store.Snapshots(id)
	if text, _ := a.Store.SnapshotText(id, snaps[0].Name); text != "原来的。" {
		t.Fatalf("the editor's text was not kept: %q", text)
	}
	if err := os.WriteFile(p, []byte("又改了。"), 0644); err != nil {
		t.Fatal(err)
	}
	a.write.checked = time.Time{}
	tt.Frame()
	if err := tt.Click("保留我的"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := disk(t, a, "第一章.txt"); got != "别处改的。" {
		t.Fatalf("on disk %q", got)
	}
	snaps, _ = a.Store.Snapshots(id)
	if text, _ := a.Store.SnapshotText(id, snaps[0].Name); text != "又改了。" || snaps[0].Reason != workspace.SnapExternal {
		t.Fatalf("the disk's text was not kept: %q", text)
	}
	// Only the indent differing, as the editor saves it, is no change.
	if err := os.WriteFile(p, []byte("　　别处改的。"), 0644); err != nil {
		t.Fatal(err)
	}
	a.write.checked = time.Time{}
	tt.Frame()
	if a.write.external != nil {
		t.Fatal("an indent alone asked the author")
	}
	// Renamed outside, the chapter is gone as far as the work knows; the
	// editor's text can still be saved.
	if err := os.Rename(p, filepath.Join(a.Store.Root(), "chapters", "改名.txt")); err != nil {
		t.Fatal(err)
	}
	a.write.checked = time.Time{}
	tt.Frame()
	if err := tt.Click("存为新章节"); err != nil {
		t.Fatal(err, tt.Texts())
	}
	tt.Frame()
	if got := disk(t, a, "第一章.txt"); got != "别处改的。" || a.write.lost {
		t.Fatalf("saved %q", got)
	}
}

// Snapshots list what was kept; restoring one keeps the text it replaces.
func TestRestoreASnapshot(t *testing.T) {
	a, tt, id := writingFixture(t, "第一版。")
	a.write.ed.MoveTo(a.write.ed.Doc.Len(), false)
	a.write.ed.Insert("加了一句。")
	tt.Frame()
	if err := tt.Click("快照"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("打开时 · 4 字") || !tt.HasText("加了一句。") {
		t.Fatal(tt.Texts())
	}
	if err := tt.Click("恢复这个版本"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := disk(t, a, "第一章.txt"); got != "第一版。" || a.write.ed.Doc.String() != "第一版。" {
		t.Fatalf("restored %q", got)
	}
	snaps, _ := a.Store.Snapshots(id)
	if snaps[0].Reason != workspace.SnapRestore {
		t.Fatalf("snapshots %+v", snaps)
	}
	if text, _ := a.Store.SnapshotText(id, snaps[0].Name); text != "第一版。加了一句。" {
		t.Fatalf("the text replaced was not kept: %q", text)
	}
}

func TestFindAndReplaceInAChapter(t *testing.T) {
	a, tt, _ := writingFixture(t, "林晚说，林晚走了。")
	if err := tt.Click("查找替换"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	a.write.query, a.write.with = "林晚", "沈砚"
	tt.Frame()
	if !tt.HasText("1 / 2") {
		t.Fatal(tt.Texts())
	}
	if err := tt.Click("全部替换"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := a.write.ed.Doc.String(); got != "沈砚说，沈砚走了。" {
		t.Fatalf("replaced %q", got)
	}
	a.write.ed.Undo()
	if got := a.write.ed.Doc.String(); got != "林晚说，林晚走了。" {
		t.Fatalf("undo %q", got)
	}
}

// Words of the prose go into the setting as the author's entry, quoting
// the chapter.
func TestTakeProseIntoTheSetting(t *testing.T) {
	a, tt, id := writingFixture(t, "她其实很怕黑。")
	a.write.ed.Select(3, 7)
	tt.Frame()
	if err := tt.Click("加入设定"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	a.write.takeText, a.write.takeCategory = "主角怕黑，从小如此", "人物"
	if err := tt.Click("加入"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	e := a.set.Entries[len(a.set.Entries)-1]
	if e.Text != "主角怕黑，从小如此" || e.Category != "人物" || len(e.Sources) != 1 || e.Sources[0].Chapter != id || e.Sources[0].Quote != "很怕黑。" {
		t.Fatalf("entry %+v", e)
	}
}

// The tree's menu trashes a chapter, and the trash brings it back.
func TestTrashAndRestoreFromTheTree(t *testing.T) {
	a, tt, _ := writingFixture(t, "一")
	two, err := a.Store.NewChapter("", "第二章", false)
	if err != nil {
		t.Fatal(err)
	}
	a.write.checked = time.Time{}
	tt.Frame()
	tt.Frame() // the sidebar shows the tree read in the frame before
	if err = tt.RightClick("章节-第二章"); err != nil {
		t.Fatal(err, tt.Texts())
	}
	if err = tt.ChooseMenuItem("移到回收站"); err != nil {
		t.Fatal(err, tt.Menu())
	}
	tt.Frame()
	if _, ok := a.write.tree.Node(two); ok || tt.HasText("第二章") && !tt.HasText("「第二章」已移到回收站") {
		t.Fatal("not trashed", tt.Texts())
	}
	if err = tt.Click("回收站"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err = tt.Click("恢复-第二章"); err != nil {
		t.Fatal(err, tt.Texts())
	}
	tt.Frame()
	if _, ok := a.write.tree.Node(two); !ok || !tt.HasText("「第二章」已恢复") {
		t.Fatal("not restored", tt.Texts())
	}
	if err = tt.Click("关闭"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.write.trashOpen {
		t.Fatal("the trash stays open")
	}
	if err = tt.RightClick("章节-第二章"); err != nil {
		t.Fatal(err)
	}
	if err = tt.ChooseMenuItem("上移"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.write.tree.Root.Children[0].ID != two {
		t.Fatal("not moved up")
	}
}

// Marks are kept beside the text and come back when the chapter opens.
func TestMarksSurviveReopening(t *testing.T) {
	a, tt, id := writingFixture(t, "井边有一把钥匙。")
	a.write.ed.Select(5, 7)
	a.write.ed.MarkSelection(editor.MarkForeshadow, "")
	tt.Frame()
	a.write.node = ""
	a.openNode(id)
	marks := a.write.ed.Doc.Marks()
	if len(marks) != 1 || a.write.ed.Doc.Slice(marks[0].Start, marks[0].End) != "钥匙" {
		t.Fatalf("marks %+v", marks)
	}
}

func TestExportTheWork(t *testing.T) {
	a, tt, _ := writingFixture(t, "雨下着。")
	if err := tt.Click("导出全书"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	a.write.indent = true
	if err := tt.Click("导出"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	b, err := os.ReadFile(filepath.Join(a.Store.Root(), "导出", "测试作品.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "第一章\n\n　　雨下着。") {
		t.Fatalf("export %q", b)
	}
}

// Dragging a chapter onto a folder puts it in there; onto a chapter, just
// before it.
func TestDragMovesChapters(t *testing.T) {
	a, tt, one := writingFixture(t, "一")
	vol, err := a.Store.NewChapter("", "第一卷", true)
	if err != nil {
		t.Fatal(err)
	}
	two, err := a.Store.NewChapter("", "第二章", false)
	if err != nil {
		t.Fatal(err)
	}
	a.write.checked = time.Time{}
	tt.Frame()
	tt.Frame()
	drag := func(from, to string) {
		t.Helper()
		f, ok1 := tt.Find("章节-" + from)
		g, ok2 := tt.Find("章节-" + to)
		if !ok1 || !ok2 {
			t.Fatal(tt.Texts())
		}
		tt.Press(f.X+20, f.Y+f.H/2)
		for k := float32(1); k <= 6; k++ {
			tt.Move(f.X+20, f.Y+f.H/2+(g.Y-f.Y)*k/6)
		}
		tt.Release(g.X+20, g.Y+g.H/2)
		tt.Frame()
	}
	drag("第二章", "第一卷")
	if n, _ := a.write.tree.Node(two); n.Parent.ID != vol {
		t.Fatal("not moved into the folder")
	}
	tt.Frame()
	drag("第一章", "第二章")
	if n, _ := a.write.tree.Node(one); n.Parent.ID != vol || n.Parent.Children[0].ID != one {
		t.Fatal("not moved before the chapter")
	}
	if _, err := os.Stat(filepath.Join(a.Store.Root(), "chapters", "第一卷", "第一章.txt")); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("章节-第一卷"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("本卷 1 字 · 全书 1 字") {
		t.Fatal("the folder's count", tt.Texts())
	}
}

// A new chapter's name is typed in place of the one it is given, and left
// empty, it keeps that one.
func TestNameANewChapter(t *testing.T) {
	a, tt, _ := writingFixture(t, "雨从傍晚开始下。")
	if err := tt.Click("新建章节"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Type("第二章 脚印")
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	n, ok := a.write.tree.Node(a.write.node)
	if !ok || n.Title != "第二章 脚印" || a.write.renaming != "" {
		t.Fatal(n, a.write.renaming)
	}
	if err := tt.Click("新建章节"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if n, _ = a.write.tree.Node(a.write.node); n.Title != "新章节" || a.write.renaming != "" {
		t.Fatal(n.Title, a.write.renaming)
	}
}

// A work left on the writing page reopens there, on the same chapter; left
// in the talk, it reopens in the talk.
func TestReopenWhereLeft(t *testing.T) {
	a, tt, id := writingFixture(t, "雨从傍晚开始下。")
	tt.Frame()
	root := a.Store.Root()
	reopen := func() *App {
		a.Close()
		b := NewWithConfig(a.ConfigFile)
		if err := b.Open(root); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { b.Close() })
		return b
	}
	// Settings saved on the way out do not forget the writing page.
	if err := tt.Click("模型设置"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	a.savePrefs()
	b := reopen()
	if b.page != 3 || b.write.node != id {
		t.Fatal(b.page, b.write.node)
	}
	a = b
	tb := ui.NewTester(a.View, 1280, 800)
	tb.Frame()
	if err := tb.Click("讨论"); err != nil {
		t.Fatal(err)
	}
	tb.Frame()
	if b = reopen(); b.page != 0 {
		t.Fatal(b.page)
	}
}

// In the narrowest window the column gives way to the page, and the counts
// to the chapter's title.
func TestNarrowWindowKeepsThePage(t *testing.T) {
	a, _, _ := writingFixture(t, "雨从傍晚开始下。")
	if w := a.panelWidth(1000); w != panelMin {
		t.Fatal(w)
	}
	if w := a.panelWidth(1280); w != panelWidth {
		t.Fatal(w)
	}
	tt := ui.NewTester(a.View, 1000, 700)
	tt.Frame()
	tt.Frame()
	if !tt.HasText("8 字") || tt.HasText("全书 8 字") {
		t.Fatal(tt.Texts())
	}
}

// A snapshot the same as the chapter says so and has nothing to restore.
func TestASnapshotLikeTheChapter(t *testing.T) {
	a, tt, _ := writingFixture(t, "第一版。")
	if err := tt.Click("快照"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("这一份和现在的正文一样") {
		t.Fatal(tt.Texts())
	}
	if err := tt.Click("恢复这个版本"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.write.snapsOpen || a.notice != "" {
		t.Fatal("restored the same text", a.notice)
	}
}

// Writing in focus leaves the page alone; quoting into the talk brings
// the column back, since the talk is in it.
func TestFocusLeavesThePageAlone(t *testing.T) {
	a, tt, _ := writingFixture(t, "雨下了一夜。")
	if err := tt.Click("专注写作"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if tt.HasText("章节-第一章") || tt.HasText("右栏-设定") || !tt.HasText("退出专注") {
		t.Fatal("the tree or the column is still there", tt.Texts())
	}
	if err := tt.Click("退出专注"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("章节-第一章") || !tt.HasText("右栏-设定") {
		t.Fatal(tt.Texts())
	}
	a.write.focus = true
	a.write.ed.Select(0, 2)
	a.quoteToTalk()
	tt.Frame()
	if a.write.focus || !tt.HasText("右栏-讨论") {
		t.Fatal("quoting kept the column away", tt.Texts())
	}
}

func TestAFolderCountsItsChapters(t *testing.T) {
	w := &writeView{counts: map[string]int{"a": 3, "b": 4, "c": 5}}
	vol := &workspace.Node{ID: "v", Folder: true, Children: []*workspace.Node{
		{ID: "a"},
		{ID: "w", Folder: true, Children: []*workspace.Node{{ID: "b"}}},
	}}
	if k := w.wordsIn(vol); k != 7 {
		t.Fatalf("folder counts %d, want 7", k)
	}
	if k := w.wordsIn(&workspace.Node{ID: "c"}); k != 5 {
		t.Fatalf("chapter counts %d, want 5", k)
	}
}

// What was written today starts from the book as the day began, survives
// reopening the work, and starts over the next day.
func TestTodayCountsFromTheDaysStart(t *testing.T) {
	a, _, _ := writingFixture(t, "一二三")
	day := time.Date(2026, 10, 9, 9, 0, 0, 0, time.Local)
	a.write.day = ""
	if k := a.today(day); k != 0 {
		t.Fatalf("today starts at %d", k)
	}
	a.write.counts[a.write.node] = 10
	if k := a.today(day.Add(time.Hour)); k != 7 {
		t.Fatalf("today is %d, want 7", k)
	}
	a.write.day, a.write.dayStart = "", 0
	a.loadPrefs()
	if k := a.today(day.Add(2 * time.Hour)); k != 7 {
		t.Fatalf("after reopening today is %d, want 7", k)
	}
	if k := a.today(day.Add(24 * time.Hour)); k != 0 {
		t.Fatalf("the next day starts at %d", k)
	}
}

// The writing switches in the settings are kept as they are ticked, not
// only with the models' save.
func TestWritingSwitchesKeepAsTicked(t *testing.T) {
	a, tt, _ := writingFixture(t, "雨从傍晚开始下。")
	if err := tt.Click("模型设置"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	for _, label := range []string{"停止输入后自动续写", "离开改过的章节时更新摘要"} {
		if err := tt.Click(label); err != nil {
			t.Fatal(err, tt.Texts())
		}
		tt.Frame()
	}
	if p := savedPrefs(t, a); !p.AutoPredict || !p.NoAutoSummary {
		t.Fatalf("prefs %+v", p)
	}
}
