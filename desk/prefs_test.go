package desk

import (
	"testing"

	"writer/workspace"
)

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
	if a.page != 3 || a.write.node != one {
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

// A category counts the entries the author still has, not the deleted
// ones; removing one with none says so rather than "0 条".
func TestCategoriesCountWhatIsKept(t *testing.T) {
	a, tt, _ := writingFixture(t, "")
	if _, err := a.Store.Revise(workspace.Revision{By: workspace.ByAuthor, Ops: []workspace.Op{
		{Op: workspace.OpAdd, Place: workspace.PlaceDropped, Category: "人物", Text: "不要了"},
	}}); err != nil {
		t.Fatal(err)
	}
	if err := a.refresh(); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("作品分类"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("删除-人物"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("1 条设定将移到“其他”") {
		t.Fatal("counted the deleted entry", tt.Texts())
	}
	if err := tt.Click("取消"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("删除-地点"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("这个分类下没有设定") {
		t.Fatal("an empty category", tt.Texts())
	}
}
