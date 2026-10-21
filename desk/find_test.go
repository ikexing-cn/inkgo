package desk

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestFindAndReplaceInAChapter(t *testing.T) {
	a, tt, _ := writingFixture(t, "林晚说，林晚走了。")
	if err := tt.Click("查找替换"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	a.write.query, a.write.with = "林晚", "沈砚"
	tt.Frame()
	if !tt.HasText("1 / 2") {
		t.Fatal(tt.Texts())
	}
	if err := tt.Click("全部替换"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := a.write.ed.Doc.String(); got != "沈砚说，沈砚走了。" {
		t.Fatalf("replaced %q", got)
	}
	a.write.ed.Undo()
	if got := a.write.ed.Doc.String(); got != "林晚说，林晚走了。" {
		t.Fatalf("undo %q", got)
	}
}

// Words picked in a paragraph are what Ctrl+F looks for, from where they
// are, selected in the box to be typed over; a pick over two paragraphs is
// not looked for.
func TestFindWhatIsPicked(t *testing.T) {
	a, tt, _ := writingFixture(t, "林晚说，林晚走了。\n雨还在下。")
	w := &a.write
	w.ed.Select(4, 6)
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyF)
	tt.Frame()
	if w.query != "林晚" || !tt.HasText("2 / 2") || !tt.Focused("查找") {
		t.Fatalf("query %q, %v, focused %v", w.query, tt.Texts(), tt.Focused("查找"))
	}
	tt.Type("雨")
	tt.Frame()
	if w.query != "雨" || !tt.HasText("1 / 1") {
		t.Fatalf("typed over: query %q", w.query)
	}
	w.ed.Select(6, 12)
	w.ed.Focus()
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyF)
	tt.Frame()
	if w.query != "雨" || !tt.Focused("查找") {
		t.Fatalf("a pick over two paragraphs: query %q, focused %v", w.query, tt.Focused("查找"))
	}
}

// Stepping through what is found from the find bar picks it with no bar of
// what to do with a pick over the prose; Escape puts the find bar away and
// the caret goes back to the prose, and a second lets go of the pick.
func TestEscapeBacksOutOfFinding(t *testing.T) {
	a, tt, _ := writingFixture(t, "林晚说，林晚走了。")
	tt.Key(ui.Cmd, ui.KeyF)
	tt.Frame()
	tt.Frame()
	a.write.query = "林晚"
	tt.Frame()
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if s, e := a.write.ed.Selection(); !tt.HasText("2 / 2") || s != 4 || e != 6 {
		t.Fatalf("Enter in the find bar went to %d-%d: %q", s, e, tt.Texts())
	}
	if tt.HasText("加入设定") {
		t.Error("stepping through what is found shows the bar over a pick")
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	tt.Frame()
	if a.write.finding || tt.HasText("2 / 2") || !tt.Focused("正文") {
		t.Fatalf("Escape left the find bar: finding %v, the prose focused %v", a.write.finding, tt.Focused("正文"))
	}
	if !tt.HasText("加入设定") {
		t.Fatal("what was found is no longer picked in the prose")
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if s, e := a.write.ed.Selection(); s != e || e != 6 || tt.HasText("加入设定") {
		t.Fatalf("a second Escape left the pick %d-%d", s, e)
	}
}
