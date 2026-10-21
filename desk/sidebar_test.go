package desk

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

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

// A window too narrow for the sidebar and a readable talk folds the
// sidebar to icons; the trash is one of them, over the gear.
func TestASlimSidebarKeepsTheTrash(t *testing.T) {
	a, _, _ := writingFixture(t, "雨从傍晚开始下。")
	a.page = pageTalk
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
	if !tt.HasText("恢复后回到原文件夹末尾。") || tt.HasText("已删除的章节及其内容暂存于此") {
		t.Fatal("the trash did not open", tt.Texts())
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
	if !strings.Contains(a.errorText(), ".order") {
		t.Fatalf("error %q", a.errorText())
	}
	a.SetError(nil)
	if err := tt.Click("正文"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if !strings.Contains(a.errorText(), ".order") {
		t.Fatalf("error %q", a.errorText())
	}
	a.openPrefs("提醒与检查")
	tt.Frame()
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

// The chapters opened are gone back and forth between on the page of
// prose, coming back to it from another; the sidebar's foot opens the
// trash from any page.
func TestSideRowsOnEveryPage(t *testing.T) {
	a, tt, one := writingFixture(t, "整条街的灯是同时灭的。")
	two, err := a.Store.NewChapter("", "第二章", false)
	if err != nil {
		t.Fatal(err)
	}
	if err = a.loadTree(); err != nil {
		t.Fatal(err)
	}
	a.openNode(two)
	tt.Frame()
	if err = tt.Click("讨论"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if _, ok := tt.Find("上一个打开的章节"); ok {
		t.Fatal("the page of discussion goes between chapters")
	}
	if err = tt.Click("正文"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err = tt.Click("上一个打开的章节"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if a.page != pageWrite || a.write.node != one {
		t.Fatalf("on page %d at %q", a.page, a.write.node)
	}
	if err = tt.Click("作品分类"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err = tt.Click("回收站"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("回收站为空") {
		t.Fatal("no trash", tt.Texts())
	}
}
