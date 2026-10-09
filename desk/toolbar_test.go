package desk

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"writer/shelf"
)

// clickMore clicks name in the toolbar's 「⋯」.
func clickMore(tt *ui.Tester, name string) error {
	if err := tt.Click("更多"); err != nil {
		return err
	}
	tt.Frame()
	return tt.Click(name)
}

// The appearance is chosen in 「⋯」, kept on this machine and handed to the
// window.
func TestChooseTheAppearance(t *testing.T) {
	a, tt, _ := writingFixture(t, "第一版。")
	var got []string
	a.Appearance = func(v string) { got = append(got, v) }
	if err := clickMore(tt, "外观-深色"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("外观-跟随系统"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("外观-浅色"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(got) != 3 || got[0] != "dark" || got[1] != "" || got[2] != "light" {
		t.Fatalf("appearances applied %q", got)
	}
	s, _, err := shelf.Load(a.books.file)
	if err != nil || s.Theme != "light" || a.Theme() != "light" {
		t.Fatalf("kept %q, %v", s.Theme, err)
	}
}

// Selecting words floats the rewrite and the taking over them; without a
// selection they are gone.
func TestTheSelectionBar(t *testing.T) {
	a, tt, _ := writingFixture(t, "她其实很怕黑。")
	if tt.HasText("AI 改写") {
		t.Fatal("a selection bar without a selection")
	}
	a.write.ed.Select(3, 7)
	tt.Frame()
	tt.Frame()
	if !tt.HasText("AI 改写") || !tt.HasText("加入设定") {
		t.Fatal(tt.Texts())
	}
	if err := tt.Click("AI 改写"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.write.rewriting || tt.HasText("加入设定") {
		t.Fatal("the bar stays over the rewrite")
	}
}
