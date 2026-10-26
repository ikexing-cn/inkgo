package desk

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"writer/shelf"
)

// clickMenu clicks the row named in a menu of the title bar.
func clickMenu(tt *ui.Tester, menu, name string) error {
	if err := tt.Click(barKey + menu); err != nil {
		return err
	}
	tt.Frame()
	return tt.Click(name)
}

// The appearance is chosen in the menu of views, kept on this machine and
// handed to the window.
func TestChooseTheAppearance(t *testing.T) {
	a, tt, _ := writingFixture(t, "第一版。")
	var got []string
	a.Appearance = func(v string) { got = append(got, v) }
	if err := clickMenu(tt, "视图", "外观-深色"); err != nil {
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
	shownEditor(a).Select(3, 7)
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

// Selected from the first line, with no room over it, the bar goes under
// the selection rather than over its later lines.
func TestTheSelectionBarLeavesTheSelectionInSight(t *testing.T) {
	a, tt, _ := writingFixture(t, strings.Repeat("雾从海上来，港口的灯一盏接一盏地熄了。", 6))
	a.sideHidden = true // whose 正文 is not the prose's
	ed := shownEditor(a)
	ed.Select(0, ed.Doc.Len())
	tt.Frame()
	tt.Frame()
	box, ok := ed.RangeBox(ed.Selection())
	prose, found := a.LastProse, a.LastProse.W > 0
	bar, shown := tt.Find("AI 改写")
	if !ok || !found || !shown || box.H < 60 { // over several lines
		t.Fatal("the selection or its bar is not there", ok, found, shown, box)
	}
	if bar.Y < prose.Y+box.Y+box.H {
		t.Fatalf("the bar at %v covers the selection %v", bar.Y-prose.Y, box)
	}
}
