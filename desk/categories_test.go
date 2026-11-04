package desk

import (
	"testing"

	"writer/workspace"
)

// A category counts the entries the author still has, not the deleted
// ones; one with none goes at the click, with nothing to move to 其他.
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
	if err := clickMenu(tt, "文件", "菜单-作品分类"); err != nil {
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
	if tt.HasText("删除-地点") || !tt.HasText("已删除分类“地点”") {
		t.Fatal("an empty category is asked about", tt.Texts())
	}
}

// Opened on a category to change, the form can be put away, leaving the
// category as it was; a new name has nothing to put away.
func TestACategoryOpenedCanBePutAway(t *testing.T) {
	a, tt, _ := fixture(t)
	if err := clickMenu(tt, "文件", "菜单-作品分类"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	catsOf(a).Name = "线索"
	tt.Frame()
	if _, ok := tt.Find("取消修改分类"); ok {
		t.Fatal("a new name can be put away")
	}
	if err := tt.Click("编辑-人物"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	was := a.Store.Work().Categories
	catsOf(a).Target = "大纲"
	tt.Frame()
	if err := tt.Click("取消修改分类"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if catsOf(a).Name != "" || catsOf(a).Target != "" {
		t.Fatalf("the form keeps %q, %q", catsOf(a).Name, catsOf(a).Target)
	}
	for i, cat := range a.Store.Work().Categories {
		if cat != was[i] {
			t.Fatalf("putting the form away changed %+v", cat)
		}
	}
	if _, ok := tt.Find("取消修改分类"); ok {
		t.Fatal("取消 stays with nothing to put away")
	}
}
