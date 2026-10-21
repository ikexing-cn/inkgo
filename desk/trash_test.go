package desk

import (
	"testing"
	"time"
)

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
