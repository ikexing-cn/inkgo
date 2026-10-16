package desk

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
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

// Leaving the box a chapter is named in settles the name, as Enter does;
// Esc leaves the name as it was.
func TestLeavingTheNameSettlesIt(t *testing.T) {
	a, tt, id := writingFixture(t, "整条街的灯是同时灭的。")
	n, _ := a.write.tree.Node(id)
	was := n.Title
	name := func() string {
		n, _ := a.write.tree.Node(id)
		return n.Title
	}
	rename := func(to string) {
		t.Helper()
		id = a.write.node // a chapter is known by its file, which is named after it
		a.write.renaming, a.write.title, a.write.titleWas, a.write.offered = id, to, name(), ""
		tt.Frame()
		tt.Frame()
		if !tt.Focused("章节标题") {
			t.Fatal("the box for the name does not have the caret")
		}
	}
	rename("灯灭")
	a.write.ed.Focus()
	for range 4 {
		tt.Frame()
	}
	if a.write.renaming != "" || name() != "灯灭" {
		t.Fatalf("leaving the box did not settle the name: renaming %q, named %q", a.write.renaming, name())
	}
	rename("不要这个")
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if a.write.renaming != "" || name() != "灯灭" || was == "灯灭" {
		t.Fatalf("Esc did not leave the name as it was: renaming %q, named %q", a.write.renaming, name())
	}
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
	if !tt.HasText("暂无章节") {
		t.Fatal(tt.Texts())
	}
	if err := tt.Click("新建第一章"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	// The name is typed in the row itself, which stays as high as the others
	// and has no button of its own: Enter settles it.
	if !tt.Focused("章节标题") {
		t.Fatal("the name of a new chapter is not ready to type")
	}
	if box, ok := tt.Find("章节标题"); !ok || box.H > hRow {
		t.Fatalf("the box for the name is higher than a row: %v", box)
	}
	tt.Type("第一章：雨夜")
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if !tt.HasText("第一章：雨夜") || a.write.renaming != "" {
		t.Fatal("the title did not take", tt.Texts())
	}
	tt.Frame()
	if !tt.Focused("正文") {
		t.Fatal("named, the chapter does not take the caret")
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
	b := newApp(a.ai.ConfigFile())
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
	if !tt.HasText("本章文件已被其他程序修改。请选择保留的版本，另一版本将存为快照。") {
		t.Fatal(tt.Texts())
	}
	if err := tt.Click("使用磁盘版本"); err != nil {
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
	if err := tt.Click("保留当前版本"); err != nil {
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
	if err := clickMenu(tt, "文件", "菜单-快照"); err != nil {
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

// Words picked in a paragraph are what Ctrl+F looks for, from where they
// are, selected in the box to be typed over; a pick over two paragraphs is
// not looked for.
func TestFindWhatIsPicked(t *testing.T) {
	a, tt, _ := writingFixture(t, "林晚说，林晚走了。\n雨还在下。")
	w := &a.write
	w.ed.Select(4, 6)
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyF)
	tt.Frame()
	if w.query != "林晚" || !tt.HasText("2 / 2") || !tt.Focused("查找") {
		t.Fatalf("query %q, %v, focused %v", w.query, tt.Texts(), tt.Focused("查找"))
	}
	tt.Type("雨")
	tt.Frame()
	if w.query != "雨" || !tt.HasText("1 / 1") {
		t.Fatalf("typed over: query %q", w.query)
	}
	w.ed.Select(6, 12)
	w.ed.Focus()
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyF)
	tt.Frame()
	if w.query != "雨" || !tt.Focused("查找") {
		t.Fatalf("a pick over two paragraphs: query %q, focused %v", w.query, tt.Focused("查找"))
	}
}

// The chapter's head and the AI's bar start where the prose starts, in a
// window that leaves the prose narrow and in one that centers it; the find
// bar is a band over the page.
func TestTheHeadLinesUpWithTheProse(t *testing.T) {
	a, tt, _ := writingFixture(t, strings.Repeat("雨从傍晚开始下。", 30))
	a.sideHidden = true // whose 正文 is not the prose's
	ed := a.write.ed
	ed.Select(0, ed.Doc.Len())
	a.openRewrite()
	for _, width := range []int{1000, 2200} {
		tt.SetSize(width, 780)
		tt.Frame()
		tt.Frame()
		prose, _ := tt.Find("正文")
		box, ok := ed.RangeBox(0, ed.Doc.Len())
		head, _ := tt.Find("章纲 · 空")
		ask, _ := tt.Find("改写要求")
		if !ok || head.X != prose.X+box.X {
			t.Fatalf("%d wide: the head at %v, the prose at %v", width, head.X, prose.X+box.X)
		}
		// The bar's card has its icon and the box in it.
		if ask.X < prose.X+box.X || ask.X > prose.X+box.X+48 {
			t.Fatalf("%d wide: the rewrite's box at %v, the prose at %v", width, ask.X, prose.X+box.X)
		}
	}
	a.write.rewriting = false
	if err := tt.Click("查找替换"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	find, _ := tt.Find("查找")
	head, _ := tt.Find("章纲 · 空")
	if find.Y > head.Y {
		t.Fatal("the find bar is under the chapter's head")
	}
}

// Stepping through what is found from the find bar picks it with no bar of
// what to do with a pick over the prose; Escape puts the find bar away and
// the caret goes back to the prose, and a second lets go of the pick.
func TestEscapeBacksOutOfFinding(t *testing.T) {
	a, tt, _ := writingFixture(t, "林晚说，林晚走了。")
	tt.Key(ui.Cmd, ui.KeyF)
	tt.Frame()
	tt.Frame()
	a.write.query = "林晚"
	tt.Frame()
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if s, e := a.write.ed.Selection(); !tt.HasText("2 / 2") || s != 4 || e != 6 {
		t.Fatalf("Enter in the find bar went to %d-%d: %q", s, e, tt.Texts())
	}
	if tt.HasText("加入设定") {
		t.Error("stepping through what is found shows the bar over a pick")
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	tt.Frame()
	if a.write.finding || tt.HasText("2 / 2") || !tt.Focused("正文") {
		t.Fatalf("Escape left the find bar: finding %v, the prose focused %v", a.write.finding, tt.Focused("正文"))
	}
	if !tt.HasText("加入设定") {
		t.Fatal("what was found is no longer picked in the prose")
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if s, e := a.write.ed.Selection(); s != e || e != 6 || tt.HasText("加入设定") {
		t.Fatalf("a second Escape left the pick %d-%d", s, e)
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
	if a.write.takeCategory != "人物" {
		t.Fatal("taken from the prose, an entry starts in", a.write.takeCategory)
	}
	a.write.takeText, a.write.takeCategory = "主角怕黑，从小如此", "关系"
	if err := tt.Click("加入"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	e := a.set.Entries[len(a.set.Entries)-1]
	if e.Text != "主角怕黑，从小如此" || e.Category != "关系" || len(e.Sources) != 1 || e.Sources[0].Chapter != id || e.Sources[0].Quote != "很怕黑。" {
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
	if err = pick(tt, "移到回收站"); err != nil {
		t.Fatal(err, rows(tt))
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
	if err = pick(tt, "上移"); err != nil {
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
	if err := clickMenu(tt, "文件", "菜单-导出"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.write.indent {
		t.Fatal("paragraphs are not indented unless asked")
	}
	// With a chapter open it is the chapter that is offered: alone, named
	// after it.
	if err := tt.Click("导出"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	b, err := os.ReadFile(filepath.Join(a.Store.Root(), "导出", "测试作品-第一章.txt"))
	if err != nil || !strings.HasPrefix(string(b), "第一章\n\n　　雨下着。") {
		t.Fatalf("chapter export %q %v", b, err)
	}
	// The whole book, when asked.
	if err := clickMenu(tt, "文件", "菜单-导出"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("全书"); err != nil {
		t.Fatal(err, tt.Texts())
	}
	tt.Frame()
	if err := tt.Click("导出"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if b, err = os.ReadFile(filepath.Join(a.Store.Root(), "导出", "测试作品.txt")); err != nil || !strings.Contains(string(b), "第一章\n\n　　雨下着。") {
		t.Fatalf("export %q %v", b, err)
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
	if err := openOutline(tt, "第一卷"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("本卷 1 字 · 全书 1 字") {
		t.Fatal("the folder's count", tt.Texts())
	}
}

// F2 names the chapter that is open, wherever the caret is: its row comes out
// of a folded folder and out of focus mode, with the box for its name and the
// name in it selected, as in the prototype.
func TestF2NamesTheOpenChapter(t *testing.T) {
	a, tt, _ := writingFixture(t, "雨从傍晚开始下。")
	folder, err := a.Store.NewChapter("", "第一卷", true)
	if err != nil {
		t.Fatal(err)
	}
	id, err := a.Store.NewChapter(folder, "第二章", false)
	if err != nil {
		t.Fatal(err)
	}
	if err = a.loadTree(); err != nil {
		t.Fatal(err)
	}
	a.openNode(id)
	a.write.folded[folder], a.write.focus = true, true
	tt.Frame()
	if _, shown := tt.Find("章节标题"); shown {
		t.Fatal("a name is being typed before F2")
	}
	tt.Key(0, ui.KeyF2)
	tt.Frame()
	tt.Frame()
	if a.write.renaming != id || a.write.title != "第二章" || a.write.focus || a.write.folded[folder] {
		t.Fatalf("F2 did not start naming the open chapter: renaming %q, title %q, focus %v, folded %v", a.write.renaming, a.write.title, a.write.focus, a.write.folded[folder])
	}
	if !tt.Focused("章节标题") {
		t.Fatal("the box for the name does not have the caret")
	}
	tt.Type("第二章：脚印") // over the name, not after it
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if n, _ := a.write.tree.Node(id); n.Title != "第二章：脚印" || a.write.renaming != "" {
		t.Fatalf("the name did not take: %q, renaming %q", n.Title, a.write.renaming)
	}
	// While a name is being typed, F2 does not start over.
	a.renameOpen()
	a.write.title = "写到一半"
	tt.Frame()
	tt.Key(0, ui.KeyF2)
	tt.Frame()
	if a.write.title != "写到一半" {
		t.Fatalf("F2 threw away what was being typed: %q", a.write.title)
	}
}

// A new chapter's name is typed in place of the one it is given, and left
// empty, it keeps that one. With no number to go on, 新章节 is in the box
// selected, typed over.
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
	if n, _ = a.write.tree.Node(a.write.node); n.Title != "第三章" || a.write.renaming != "" {
		t.Fatal("the number does not go on", n.Title, a.write.renaming)
	}
	if err := tt.Click("新建章节"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.write.title != "第四章 " {
		t.Fatalf("offered %q", a.write.title)
	}
	tt.Type("守夜人")
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if n, _ = a.write.tree.Node(a.write.node); n.Title != "第四章 守夜人" {
		t.Fatal(n.Title)
	}
	a.openNode(a.write.tree.Root.Children[0].ID)
	a.write.tree.Root.Children[0].Title = "楔子"
	a.write.renaming = ""
	if err := a.Store.RenameChapter(a.write.node, "楔子"); err != nil {
		t.Fatal(err)
	}
	a.warn(a.loadTree())
	if err := tt.Click("新建章节"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if n, _ = a.write.tree.Node(a.write.node); n.Title != "新章节" {
		t.Fatal("a title with no number is numbered", n.Title)
	}
	if err := tt.Click("新建章节"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Type("尾声")
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if n, _ = a.write.tree.Node(a.write.node); n.Title != "尾声" {
		t.Fatal("新章节 is not typed over:", n.Title)
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
		b := newApp(a.ai.ConfigFile())
		if err := b.Open(root); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { b.Close() })
		return b
	}
	// Settings saved on the way out do not forget the writing page.
	if err := tt.Click("设置"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("设置-AI"); err != nil {
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

// A work never opened on this machine opens on its prose, as a new one
// does, as the prototype opens every book; one fetched without .writer/
// keeps where it was left all the same.
func TestAWorkNewHereOpensOnItsProse(t *testing.T) {
	a, _, _ := fixture(t)
	root := a.Store.Root()
	a.Close()
	if err := os.RemoveAll(filepath.Join(root, ".writer")); err != nil {
		t.Fatal(err)
	}
	b := newApp(a.ai.ConfigFile())
	if err := b.Open(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { b.Close() })
	if b.page != 3 {
		t.Fatal("opened on", b.page)
	}
	// Where it was left is kept, though .writer/ was not there.
	tb := ui.NewTester(b.View, 1266, 780)
	tb.Frame()
	if err := tb.Click("讨论"); err != nil {
		t.Fatal(err)
	}
	tb.Frame()
	if _, err := os.Stat(filepath.Join(root, ".writer", "desk.json")); err != nil || b.Error != "" {
		t.Fatal(err, b.Error)
	}
	b.openCreate()
	b.name, b.books.at = "新书", filepath.Join(t.TempDir(), "新书")
	if err := b.create(); err != nil || b.Error != "" {
		t.Fatal(err, b.Error)
	}
	if b.page != 3 {
		t.Fatal("a new work opened on", b.page)
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

// A window too narrow for the sidebar and a readable talk folds the
// sidebar to icons; the trash is one of them, over the gear.
func TestASlimSidebarKeepsTheTrash(t *testing.T) {
	a, _, _ := writingFixture(t, "雨从傍晚开始下。")
	a.page = 0
	tt := ui.NewTester(a.View, 1100, 700)
	tt.Frame()
	tt.Frame()
	if tt.HasText("测试作品") || !tt.HasText("回收站") || !tt.HasText("设置") {
		t.Fatal("the slim sidebar", tt.Texts())
	}
	if err := tt.Click("回收站"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("已删除的章节及其内容暂存于此，恢复后回到原文件夹末尾。") {
		t.Fatal("the trash did not open", tt.Texts())
	}
}

// A snapshot the same as the chapter says so and has nothing to restore.
func TestASnapshotLikeTheChapter(t *testing.T) {
	a, tt, _ := writingFixture(t, "第一版。")
	if err := clickMenu(tt, "文件", "菜单-快照"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("该快照与当前正文相同") {
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
	// F11 from the prose goes in and out.
	a.write.ed.Focus()
	tt.Key(0, ui.KeyF11)
	tt.Frame()
	if !a.write.focus {
		t.Fatal("F11 did not leave the page alone")
	}
	tt.Key(0, ui.KeyF11)
	tt.Frame()
	if a.write.focus {
		t.Fatal("F11 did not bring the tree and the column back")
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
	// The day gone is kept, also with the work, and shows among the last
	// days; a day the work was not open is nothing.
	a.write.counts[a.write.node] = 15
	week := a.lastDays(day.Add(72*time.Hour), 4)
	if want := []int{7, 5, 0, 0}; !slices.Equal(week, want) {
		t.Fatalf("the last days are %v, want %v", week, want)
	}
	if p := savedPrefs(t, a); p.Days["2026-10-09"] != 7 || p.Days["2026-10-10"] != 5 {
		t.Fatalf("days kept %v", p.Days)
	}
}

// The writing switches in the settings are kept as they are ticked, not
// only with the models' save.
func TestWritingSwitchesKeepAsTicked(t *testing.T) {
	a, tt, _ := writingFixture(t, "雨从傍晚开始下。")
	if err := tt.Click("设置"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("设置-AI"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	for _, label := range []string{"停笔后自动续写", "章节修改后更新摘要"} {
		if err := tt.Click(label); err != nil {
			t.Fatal(err, tt.Texts())
		}
		tt.Frame()
	}
	if p := savedPrefs(t, a); !p.AutoPredict || !p.NoAutoSummary {
		t.Fatalf("prefs %+v", p)
	}
}

// The day's goal is set from the counts in the bar and kept with the work;
// what is not a count leaves it be, and is said in red under the field.
func TestSetTheDaysGoal(t *testing.T) {
	a, tt, _ := writingFixture(t, "一二三")
	if err := tt.Click("字数"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.write.goalOpen || !tt.HasText("每日目标") {
		t.Fatal("no goal to set", tt.Texts())
	}
	a.setGoal("2000")
	if a.write.goal != 2000 || savedPrefs(t, a).Goal != 2000 {
		t.Fatal(a.write.goal)
	}
	for _, typed := range []string{"两千", "0"} {
		a.write.goalText = typed
		a.setGoal(typed)
		tt.Frame()
		if a.write.goal != 2000 || !tt.HasText("每日目标须为正整数，留空则不设。") {
			t.Fatal(typed, "changed the goal, or was not said to be wrong:", a.write.goal)
		}
	}
	a.write.goalText = "2000"
	tt.Frame()
	if tt.HasText("每日目标须为正整数，留空则不设。") || !tt.HasText("设置后，字数旁显示今日字数和目标。留空则不设。") {
		t.Fatal("a goal set right is said to be wrong", tt.Texts())
	}
	a.write.focus = true // room for today's count in the bar
	tt.Frame()
	tt.Frame()
	if !tt.HasText("/ 2,000") {
		t.Fatal("the goal is not by the counts", tt.Texts())
	}
	a.setGoal("")
	if a.write.goal != 0 || savedPrefs(t, a).Goal != 0 {
		t.Fatal("the goal stayed", a.write.goal)
	}
}

// Ctrl+PageDown and Ctrl+PageUp go through the chapters in the tree's
// order, over folders, and stop at the ends.
func TestStepThroughChapters(t *testing.T) {
	a, tt, one := writingFixture(t, "一")
	vol, err := a.Store.NewChapter("", "第一卷", true)
	if err != nil {
		t.Fatal(err)
	}
	two, err := a.Store.NewChapter(vol, "第二章", false)
	if err != nil {
		t.Fatal(err)
	}
	a.warn(a.loadTree())
	step := func(key ui.Key, want string) {
		t.Helper()
		a.write.ed.Focus()
		tt.Key(ui.Cmd, key)
		tt.Frame()
		if a.write.node != want {
			t.Fatalf("at %s, want %s", a.write.node, want)
		}
	}
	step(ui.KeyPageDown, two)
	step(ui.KeyPageDown, two)
	step(ui.KeyPageUp, one)
	step(ui.KeyPageUp, one)
	a.openNode(vol)
	tt.Frame()
	a.stepChapter(1)
	if a.write.node != two {
		t.Fatal("from a folder down is not its first chapter", a.write.node)
	}
	a.openNode(vol)
	a.stepChapter(-1)
	if a.write.node != one {
		t.Fatal("from a folder up is not the chapter before it", a.write.node)
	}
}

// A chapter opens with the caret where it was left, also after the work
// is opened again.
func TestChapterOpensWhereItWasLeft(t *testing.T) {
	a, tt, one := writingFixture(t, "一二三四五六")
	two, err := a.Store.NewChapter("", "第二章", false)
	if err != nil {
		t.Fatal(err)
	}
	a.warn(a.loadTree())
	a.write.ed.MoveTo(4, false)
	a.openNode(two)
	tt.Frame()
	a.openNode(one)
	tt.Frame()
	if got := a.write.ed.Caret(); got != 4 {
		t.Fatal("caret at", got)
	}
	if savedPrefs(t, a).Carets[one] != 4 {
		t.Fatal("the caret is not kept with the work", savedPrefs(t, a).Carets)
	}
	a.write.carets = nil
	a.loadPrefs()
	a.openNode(two)
	a.openNode(one)
	if got := a.write.ed.Caret(); got != 4 {
		t.Fatal("caret after opening again at", got)
	}
}

// Ctrl+S saves the chapter at once and says so.
func TestCtrlSSavesNow(t *testing.T) {
	a, tt, one := writingFixture(t, "一")
	a.write.ed.Focus()
	tt.Frame()
	a.write.ed.Insert("二")
	tt.Key(ui.Cmd, ui.KeyS)
	tt.Frame()
	ch, err := a.Store.ReadChapter(one)
	if err != nil {
		t.Fatal(err)
	}
	if ch.Text != a.write.ed.Doc.String() || !strings.Contains(ch.Text, "二") {
		t.Fatalf("disk has %q", ch.Text)
	}
	if !strings.HasPrefix(a.notice, "已保存") {
		t.Fatal(a.notice)
	}
}

// Ctrl+N makes a chapter after the one open, ready to be named.
func TestCtrlNMakesAChapter(t *testing.T) {
	a, tt, one := writingFixture(t, "一")
	a.write.ed.Focus()
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyN)
	tt.Frame()
	if a.write.node == one || a.write.renaming != a.write.node {
		t.Fatal("no new chapter to name", a.write.node, a.write.renaming)
	}
	if n, _ := a.write.tree.Node(a.write.node); n.Folder || n.Parent.Children[0].ID != one {
		t.Fatal("the new chapter is not after the one open")
	}
}

// Ctrl+/ shows the shortcuts; Esc puts them away.
func TestShortcutsHelp(t *testing.T) {
	a, tt, _ := writingFixture(t, "一")
	tt.Key(ui.Cmd, ui.KeySlash)
	tt.Frame()
	if !a.keysOpen || !tt.HasText("快捷键") || !tt.HasText("下一章 / 上一章") {
		t.Fatal("no shortcuts", a.keysOpen, tt.Texts())
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if a.keysOpen {
		t.Fatal("the shortcuts stay")
	}
}

// Ctrl+= and Ctrl+- size the text within bounds, Ctrl+0 puts it back, and
// the size is kept with the work.
func TestSizeTheText(t *testing.T) {
	a, tt, one := writingFixture(t, "一")
	a.write.ed.Focus()
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyEqual)
	tt.Key(ui.Cmd, ui.KeyEqual)
	tt.Frame()
	if a.write.ed.FontSize() != 19 || savedPrefs(t, a).TextSize != 19 {
		t.Fatal("size", a.write.ed.FontSize(), savedPrefs(t, a).TextSize)
	}
	a.loadPrefs()
	a.openNode(one)
	if a.write.ed.FontSize() != 19 {
		t.Fatal("a chapter opens at", a.write.ed.FontSize())
	}
	for range 20 {
		a.sizeText(-1)
	}
	if a.write.ed.FontSize() != textSizeMin {
		t.Fatal("below the least", a.write.ed.FontSize())
	}
	a.write.ed.Focus()
	tt.Frame()
	tt.Key(ui.Cmd, ui.Key0)
	tt.Frame()
	if a.write.ed.FontSize() != textSizeDefault || savedPrefs(t, a).TextSize != 0 {
		t.Fatal("not back", a.write.ed.FontSize())
	}
}

// The window is named after the work, and on the writing page after the
// chapter open too.
func TestWindowTitle(t *testing.T) {
	a, tt, _ := writingFixture(t, "一")
	var titles []string
	a.SetTitle = func(s string) bool { titles = append(titles, s); return true }
	work := a.Store.Work().Name
	tt.Frame()
	tt.Frame()
	if want := "第一章 — " + work; len(titles) != 1 || titles[0] != want {
		t.Fatalf("titles %q, want once %q", titles, want)
	}
	if err := tt.Click("讨论"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if titles[len(titles)-1] != work {
		t.Fatalf("discussing: %q", titles)
	}
}

// With text selected, the header counts the selection in place of the
// chapter.
func TestCountTheSelection(t *testing.T) {
	a, tt, _ := writingFixture(t, "林晚推门。 雨停了。")
	a.write.focus = true
	tt.Frame()
	if !tt.HasText("本章 9 字 · 全书 9 字") {
		t.Fatal("before", tt.Texts())
	}
	a.write.ed.MoveTo(0, false)
	a.write.ed.MoveTo(5, true)
	tt.Frame()
	if !tt.HasText("选中 5 字 · 全书 9 字") {
		t.Fatal("selected", tt.Texts())
	}
}

// A folder lists each chapter with the start of its outline, or of its
// summary when it has no outline.
func TestFolderShowsTheGists(t *testing.T) {
	a, tt, _ := writingFixture(t, "一")
	vol, err := a.Store.NewChapter("", "第一卷", true)
	if err != nil {
		t.Fatal(err)
	}
	planned, _ := a.Store.NewChapter(vol, "第二章", false)
	summed, _ := a.Store.NewChapter(vol, "第三章", false)
	if err = a.Store.WriteOutline(planned, "她找到\n井底的钥匙"); err != nil {
		t.Fatal(err)
	}
	if err = a.Store.WriteSummary(summed, workspace.Summary{Text: "老周说出旧事。", Of: workspace.ProseHash(""), At: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if err = a.loadTree(); err != nil {
		t.Fatal(err)
	}
	a.openNode(vol)
	tt.Frame()
	if !tt.HasText("她找到 井底的钥匙") || !tt.HasText("老周说出旧事。") {
		t.Fatal(tt.Texts())
	}
}

// The counts move as the author types, before the text is saved.
func TestCountsMoveAsTyped(t *testing.T) {
	a, tt, _ := writingFixture(t, "一")
	saveAfter = time.Hour
	a.write.focus = true
	a.write.ed.Focus()
	a.write.ed.MoveTo(1, false)
	tt.Frame()
	tt.Type("二三")
	tt.Frame()
	if !tt.HasText("本章 3 字 · 全书 3 字 · 今天 +2") {
		t.Fatal(tt.Texts())
	}
}

// The title bar's arrows go back and forth between the chapters opened, as
// a browser's; opening another drops what was ahead, and a chapter deleted
// is passed over.
func TestChapterHistory(t *testing.T) {
	a, tt, one := writingFixture(t, "一。")
	two, err := a.Store.NewChapter("", "第二章", false)
	if err != nil {
		t.Fatal(err)
	}
	three, err := a.Store.NewChapter("", "第三章", false)
	if err != nil {
		t.Fatal(err)
	}
	if err = a.loadTree(); err != nil {
		t.Fatal(err)
	}
	a.openNode(two)
	a.openNode(three)
	tt.Frame()
	step := func(label, want string) {
		t.Helper()
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		if a.write.node != want {
			t.Fatalf("after %s: shown %q, want %q", label, a.write.node, want)
		}
	}
	step("上一个打开的章节", two)
	step("上一个打开的章节", one)
	step("下一个打开的章节", two)
	a.openNode(three)
	if len(a.write.forward) != 0 {
		t.Fatalf("forward = %v after opening another", a.write.forward)
	}
	if err = a.Store.TrashChapter(two); err != nil {
		t.Fatal(err)
	}
	if err = a.loadTree(); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	step("上一个打开的章节", one) // 第二章 is gone; 第一章 was before it
}

// A chapter changed outside is not written over by the save that comes
// before the next look at the disk: the author is asked, as when the look
// finds it.
func TestSavingDoesNotWriteOverAnOutsideChange(t *testing.T) {
	a, tt, id := writingFixture(t, "原来的。")
	p := filepath.Join(a.Store.Root(), "chapters", "第一章.txt")
	if err := os.WriteFile(p, []byte("别处改的，很重要。"), 0644); err != nil {
		t.Fatal(err)
	}
	a.write.ed.Focus()
	tt.Frame()
	tt.Type("新字")
	tt.Frame()
	tt.Frame()
	if got := disk(t, a, "第一章.txt"); got != "别处改的，很重要。" {
		t.Fatalf("the save wrote over the outside change: %q", got)
	}
	if !tt.HasText("本章文件已被其他程序修改。请选择保留的版本，另一版本将存为快照。") {
		t.Fatal(tt.Texts())
	}
	// Keeping the editor's writes it, the disk's kept as a snapshot.
	if err := tt.Click("保留当前版本"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := disk(t, a, "第一章.txt"); got != "新字原来的。" {
		t.Fatalf("on disk %q", got)
	}
	snaps, _ := a.Store.Snapshots(id)
	if text, _ := a.Store.SnapshotText(id, snaps[0].Name); text != "别处改的，很重要。" || snaps[0].Reason != workspace.SnapExternal {
		t.Fatalf("the disk's text was not kept: %q", text)
	}
	// And the next save goes through.
	a.write.ed.Focus()
	tt.Frame()
	tt.Type("再写")
	tt.Frame()
	tt.Frame()
	if got := disk(t, a, "第一章.txt"); got != "新字再写原来的。" || a.write.external != nil {
		t.Fatalf("on disk %q", got)
	}
}

// The same words under a new date, as a syncing program leaves them, are
// no change: the save goes through.
func TestSavingOverAFileOnlyTouched(t *testing.T) {
	a, tt, _ := writingFixture(t, "原来的。")
	p := filepath.Join(a.Store.Root(), "chapters", "第一章.txt")
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(p, later, later); err != nil {
		t.Fatal(err)
	}
	a.write.ed.Focus()
	tt.Frame()
	tt.Type("新字")
	tt.Frame()
	tt.Frame()
	if got := disk(t, a, "第一章.txt"); got != "新字原来的。" || a.write.external != nil || a.Error != "" {
		t.Fatalf("on disk %q, error %q", got, a.Error)
	}
}

// Words the work is left with, the disk holding others the author had not
// chosen between, are kept as a snapshot.
func TestLeavingWithAnOutsideChangeKeepsTheEditorsWords(t *testing.T) {
	a, tt, id := writingFixture(t, "原来的。")
	p := filepath.Join(a.Store.Root(), "chapters", "第一章.txt")
	a.write.ed.Focus()
	tt.Frame()
	if err := os.WriteFile(p, []byte("别处改的，很重要。"), 0644); err != nil {
		t.Fatal(err)
	}
	saveAfter = time.Hour // the words are not saved before the work is left
	tt.Type("新字")
	tt.Frame()
	store := a.Store
	a.flush()
	a.keepUnsettled()
	if got := disk(t, a, "第一章.txt"); got != "别处改的，很重要。" {
		t.Fatalf("on disk %q", got)
	}
	snaps, _ := store.Snapshots(id)
	if text, _ := store.SnapshotText(id, snaps[0].Name); text != "新字原来的。" || snaps[0].Reason != workspace.SnapExternal {
		t.Fatalf("the editor's words were not kept: %q (%s)", text, snaps[0].Reason)
	}
}

// The settings' reminders count the foreshadowings though the writing page
// was never shown: they read the tree and the marks themselves.
func TestRemindersBeforeTheWritingPage(t *testing.T) {
	a, tt, _ := fixture(t)
	var first string
	for i := 1; i <= 12; i++ {
		id, err := a.Store.NewChapter("", fmt.Sprintf("第%d章", i), false)
		if err != nil {
			t.Fatal(err)
		}
		if i == 1 {
			first = id
		}
	}
	marks := []workspace.ChapterMark{{ID: "f1", Kind: workspace.MarkForeshadow, Start: 0, End: 2}}
	if _, err := a.Store.WriteChapter(first, "钥匙在井里。", marks); err != nil {
		t.Fatal(err)
	}
	if a.write.entered {
		t.Fatal("the writing page was shown")
	}
	a.write.lateAfter = lateMin
	a.openPrefs("提醒与检查")
	tt.Frame()
	if !tt.HasText("当前有 1 个伏笔久未回收。") {
		t.Fatal(tt.Texts())
	}
}

// A chapter tree that cannot be read is told, and nothing breaks: the
// work opens, and the writing page has no chapters to show.
func TestATreeThatCannotBeRead(t *testing.T) {
	a, tt, _ := fixture(t)
	if _, err := a.Store.NewChapter("", "第一章", false); err != nil {
		t.Fatal(err)
	}
	order := filepath.Join(a.Store.Root(), "chapters", ".order")
	if err := os.WriteFile(order, []byte(`[{"id":`), 0644); err != nil {
		t.Fatal(err)
	}
	a.loadChecks() // as the work opens
	tt.Frame()
	if !strings.Contains(a.Error, ".order") {
		t.Fatalf("error %q", a.Error)
	}
	a.Error = ""
	if err := tt.Click("正文"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if !strings.Contains(a.Error, ".order") {
		t.Fatalf("error %q", a.Error)
	}
	a.openPrefs("提醒与检查")
	tt.Frame()
}

// What the desk remembers of a work is written whole: the file is put in
// place of the last, with nothing left beside it.
func TestPrefsAreWrittenWhole(t *testing.T) {
	a, _, _ := writingFixture(t, "原来的。")
	a.savePrefs()
	b, err := os.ReadFile(a.prefsPath())
	if err != nil || !strings.Contains(string(b), a.write.node) {
		t.Fatalf("%s, %v", b, err)
	}
	if _, err = os.Stat(a.prefsPath() + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("a file left beside it: %v", err)
	}
}

// paragraphs cuts the pieces of a text at the ends of its lines, keeping
// the empty ones.
func TestParagraphsOfADiff(t *testing.T) {
	got := paragraphs([]piece{{same, "一\n\n二"}, {gone, "旧"}, {came, "新\n三"}, {same, "\n"}})
	want := [][]piece{{{same, "一"}}, nil, {{same, "二"}, {gone, "旧"}, {came, "新"}}, {{came, "三"}}, nil}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%v", got)
	}
}

// The snapshot shows a paragraph to an element, and they select and copy
// as the one text they are.
func TestASnapshotSelectsAcrossItsParagraphs(t *testing.T) {
	old := "第一段旧的。\n\n第三段。\n第四段。"
	a, tt, id := writingFixture(t, "第一段新的。\n\n第三段。\n第四段。")
	if _, err := a.Store.TakeSnapshot(id, workspace.SnapInterval, old, nil); err != nil {
		t.Fatal(err)
	}
	a.openSnapshots()
	tt.Frame()
	shown := &a.write.snapShown
	if shown.same || shown.long || len(shown.paras) != 4 || len(shown.paras[0]) != 4 || len(shown.paras[1]) != 0 {
		t.Fatalf("compared: %+v", shown.paras)
	}
	// Made as the snapshot or the comparing changes, and not in each frame.
	first := &shown.paras[0][0]
	tt.Move(500, 300)
	tt.Frame()
	if first != &a.write.snapShown.paras[0][0] {
		t.Fatal("the comparison was made again in a frame nothing changed in")
	}
	a.write.compare = false
	tt.Frame()
	if len(shown.paras) != 4 || len(shown.paras[0]) != 1 || shown.paras[0][0].Text != "第一段旧的。" {
		t.Fatalf("plain: %+v", shown.paras)
	}
	from, ok := tt.Find("第一段旧的。")
	to, ok2 := tt.Find("第四段。")
	third, ok3 := tt.Find("第三段。")
	if !ok || !ok2 || !ok3 || to.Y <= third.Y {
		t.Fatal(from, to, tt.Texts())
	}
	// The empty line between takes the room of a line.
	if line := to.Y - third.Y; third.Y-from.Y < 2*line-1 || third.Y-from.Y > 2*line+1 {
		t.Fatalf("the first at %v, the third at %v, a line is %v", from.Y, third.Y, line)
	}
	tt.Press(from.X+1, from.Y+from.H/2)
	tt.Move(to.X+to.W-1, to.Y+to.H/2)
	tt.Frame()
	tt.Release(to.X+to.W-1, to.Y+to.H/2)
	tt.Frame()
	tt.Command("copy")
	tt.Frame()
	if got := tt.Clipboard(); got != old {
		t.Fatalf("copied %q", got)
	}
}

// A snapshot too long to build whole builds the paragraphs in view, and
// restores as any other.
func TestALongSnapshot(t *testing.T) {
	var old strings.Builder
	n := 0
	for ; old.Len() <= snapWhole; n++ {
		fmt.Fprintf(&old, "第%d段，街上的灯一盏一盏地灭下去。\n", n)
	}
	last := fmt.Sprintf("第%d段，街上的灯一盏一盏地灭下去。", n-1)
	a, tt, id := writingFixture(t, "只有一句。")
	if _, err := a.Store.TakeSnapshot(id, workspace.SnapInterval, old.String(), nil); err != nil {
		t.Fatal(err)
	}
	a.openSnapshots()
	a.write.compare = false
	tt.Frame()
	tt.Frame()
	shown := &a.write.snapShown
	if !shown.long || len(shown.paras) != n+1 {
		t.Fatalf("long %v, %d paragraphs of %d", shown.long, len(shown.paras), n+1)
	}
	if !tt.HasText("第0段，街上的灯一盏一盏地灭下去。") || tt.HasText(last) {
		t.Fatal("the paragraphs in view, and only those, are built")
	}
	// It scrolls to the others.
	at, _ := tt.Find("第0段，街上的灯一盏一盏地灭下去。")
	for range 4 {
		tt.Scroll(at.X+10, at.Y+10, 0, 600)
		tt.Frame()
	}
	if first, _ := shown.list.Visible(); first == 0 || tt.HasText("第0段，街上的灯一盏一盏地灭下去。") {
		t.Fatal("the long snapshot did not scroll: its first paragraph in view is", first)
	}
	if err := tt.Click("恢复这个版本"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := disk(t, a, "第一章.txt"); got != old.String() {
		t.Fatalf("restored %d bytes of %d", len(got), old.Len())
	}
}

// The prose keeps the keyboard as what is around it comes and goes: the
// title bar and the sidebar in focus mode, the sidebar hidden, the find bar
// over it.
func TestTheProseKeepsTheKeyboard(t *testing.T) {
	a, tt, _ := writingFixture(t, "林晚说，林晚走了。")
	a.write.ed.Focus()
	tt.Frame()
	tt.Frame()
	steps := []struct {
		name string
		do   func()
	}{
		{"focus mode", func() { tt.Key(0, ui.KeyF11) }},
		{"out of focus mode", func() { tt.Key(0, ui.KeyF11) }},
		{"the sidebar hidden", func() { a.sideHidden = true }},
		{"the sidebar shown", func() { a.sideHidden = false }},
		{"the find bar over it", func() { a.write.finding = true; a.write.query = "林晚" }},
	}
	for _, s := range steps {
		if s.name == "the find bar over it" {
			a.write.ed.Focus() // opened from the menu, the find bar is not focused
		}
		s.do()
		tt.Frame()
		tt.Frame()
		if !tt.Focused("正文") {
			t.Errorf("%s: the prose lost the keyboard", s.name)
		}
	}
}

// A chapter added between two numbered ones is not given the number of the
// one after it: there is no number free there.
func TestANewChapterBetweenTwoTakesNoNumber(t *testing.T) {
	a, tt, id := writingFixture(t, "雨从傍晚开始下。")
	if _, err := a.Store.NewChapter("", "第二章 井", false); err != nil {
		t.Fatal(err)
	}
	a.warn(a.loadTree())
	a.openNode(id)
	tt.Frame()
	if err := tt.Click("新建章节"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.write.title != "新章节" {
		t.Fatalf("offered %q", a.write.title)
	}
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if n, _ := a.write.tree.Node(a.write.node); n.Title != "新章节" {
		t.Fatal("numbered as the chapter after it:", n.Title)
	}
}
