package desk

import (
	"testing"
	"time"
)

// Finding in the whole book lists each chapter with the query and opens
// the one chosen with its first match selected.
func TestFindInTheWholeBook(t *testing.T) {
	a, tt, first := writingFixture(t, "林晚走进雨里。")
	second, err := a.Store.NewChapter("", "第二章", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.Store.WriteChapter(second, "门开了，林晚又看见了林晚的影子。", nil); err != nil {
		t.Fatal(err)
	}
	a.write.checked = time.Time{}
	tt.Frame()
	if err = tt.Click("查找替换"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Type("林晚")
	tt.Frame()
	if err = tt.Click("全书"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("全书 2 章、共 3 处") || !tt.HasText("门开了，林晚又看见了林晚的影子。") {
		t.Fatal(tt.Texts())
	}
	if err = tt.Click("全书结果-第二章"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.write.node != second || !a.write.finding {
		t.Fatal("the chapter chosen did not open with the search", a.write.node, first)
	}
	if s, e := a.write.ed.Selection(); s != 4 || e != 6 {
		t.Fatal("the first match is not selected", s, e)
	}
	// What is typed into the chapter open counts before it is saved.
	a.write.ed.Replace(0, 0, "林晚")
	tt.Frame()
	if !tt.HasText("全书 2 章、共 4 处") {
		t.Fatal(tt.Texts())
	}
}

func TestAroundKeepsALittleOnEachSide(t *testing.T) {
	if got := around("一二三四五林晚六七八九十", "林晚", 2); got != "…四五林晚六七…" {
		t.Fatal(got)
	}
	if got := around("林晚\n\n走了", "林晚", 5); got != "林晚 走了" {
		t.Fatal(got)
	}
}
