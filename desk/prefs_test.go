package desk

import "testing"

// The sidebar's top row goes back and forth between chapters, and its
// foot opens the trash, from any page.
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
