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
	if tt.HasText("删除-地点") || !tt.HasText("已删除分类“地点”") {
		t.Fatal("an empty category is asked about", tt.Texts())
	}
}

// A part of the settings with nothing to say under its title says nothing
// there, not an empty line; one that does still says it.
func TestSettingsSayNothingNeedless(t *testing.T) {
	a, tt, _ := fixture(t)
	for tab, note := range map[string]string{"AI": "", "备份": "", "外观": "仅在本机生效，适用于所有作品。", "提醒与检查": "仅适用于本作品。"} {
		a.openPrefs(tab)
		tt.Frame()
		texts := tt.Texts()
		at := -1 // the list beside names the parts too; the title comes after it
		for i, s := range texts {
			if s == a.prefTitle(tab) {
				at = i
			}
		}
		if at < 0 || at+1 >= len(texts) {
			t.Fatal(tab, texts)
		}
		if a.prefNote(tab) != note || note == "" && texts[at+1] == "" || note != "" && texts[at+1] != note {
			t.Fatalf("%s: note %q, then %q", tab, a.prefNote(tab), texts[at+1])
		}
	}
}
