package desk

import "testing"

// A tab opened once keeps the same openChapter, and the same editor, when
// it is shown again; nothing is copied.
func TestOneOpenChapterPerTab(t *testing.T) {
	a, tt, ids := tabsFixture(t)
	rereadBook(a)
	first := a.write.open[ids["第一章"]]
	if first == nil {
		t.Fatal("the first chapter has no openChapter")
	}
	a.openNode(ids["第二章"])
	tt.Frame()
	a.openNode(ids["第一章"])
	tt.Frame()
	if got := a.write.open[ids["第一章"]]; got != first {
		t.Fatal("the first chapter got another openChapter")
	}
	if a.write.cur() != first {
		t.Fatal("the shown chapter is not the first one's")
	}
}

// Size, predict mode and the AI switch reach an editor that is not shown.
func TestEditorSettingsReachEveryOpenTab(t *testing.T) {
	a, tt, ids := tabsFixture(t)
	rereadBook(a)
	a.openNode(ids["第二章"])
	tt.Frame()
	a.openNode(ids["第一章"])
	tt.Frame()
	a.openNode(ids["第二章"])
	tt.Frame()
	first := a.write.open[ids["第一章"]]
	a.sizeText(1)
	a.openPrefs("AI")
	tt.Frame()
	if err := tt.Click("停笔后自动续写"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	a.turnAI(false)
	if first.ed.FontSize() != 18 || !first.ed.NoAI() || first.ed.PredictOnDemand() {
		t.Fatalf("settings did not reach the hidden tab: size %v, noAI %v, on demand %v",
			first.ed.FontSize(), first.ed.NoAI(), first.ed.PredictOnDemand())
	}
}

// The 停笔后自动续写 box changes the chapter shown at once.
func TestAutoPredictChangesTheEditorShown(t *testing.T) {
	a, tt, _ := writingFixture(t, "正文。")
	ed := a.write.cur().ed
	if !ed.PredictOnDemand() {
		t.Fatal("the editor did not start on demand")
	}
	a.openPrefs("AI")
	tt.Frame()
	if err := tt.Click("停笔后自动续写"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if ed.PredictOnDemand() {
		t.Fatal("the box did not change the editor shown")
	}
}
