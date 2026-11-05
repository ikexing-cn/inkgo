package desk

import "testing"

// Words of the prose go into the setting as the author's entry, quoting
// the chapter.
func TestTakeProseIntoTheSetting(t *testing.T) {
	a, tt, id := writingFixture(t, "她其实很怕黑。")
	shownEditor(a).Select(3, 7)
	tt.Frame()
	if err := tt.Click("加入设定"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if lorePanel(a).Take.Category != "人物" {
		t.Fatal("taken from the prose, an entry starts in", lorePanel(a).Take.Category)
	}
	lorePanel(a).Take.Text, lorePanel(a).Take.Category = "主角怕黑，从小如此", "关系"
	if err := tt.Click("加入"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	e := setting(a).Entries[len(setting(a).Entries)-1]
	if e.Text != "主角怕黑，从小如此" || e.Category != "关系" || len(e.Sources) != 1 || e.Sources[0].Chapter != id || e.Sources[0].Quote != "很怕黑。" {
		t.Fatalf("entry %+v", e)
	}
}
