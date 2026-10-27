package desk

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"writer/desk/kit"
)

// A work left on the writing page reopens there, on the same chapter; left
// in the talk, it reopens in the talk.
func TestReopenWhereLeft(t *testing.T) {
	a, tt, id := writingFixture(t, "雨从傍晚开始下。")
	tt.Frame()
	root := a.Store.Root()
	reopen := func() *App {
		a.Close()
		b := newApp(a.ai.ConfigFile())
		if err := b.Open(root); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { b.Close() })
		return b
	}
	// Settings saved on the way out do not forget the writing page.
	if err := clickMenu(tt, "文件", "菜单-设置"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("设置-AI"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	a.savePrefs()
	b := reopen()
	if b.page != pageWrite || b.write.node != id {
		t.Fatal(b.page, b.write.node)
	}
	a = b
	tb := stillTester(a.View, 1280, 800)
	tb.Frame()
	if err := tb.Click("讨论"); err != nil {
		t.Fatal(err)
	}
	tb.Frame()
	if b = reopen(); b.page != pageTalk {
		t.Fatal(b.page)
	}
}

// Ctrl+PageDown and Ctrl+PageUp go through the chapters in the tree's
// order, over folders, and stop at the ends.
func TestStepThroughChapters(t *testing.T) {
	a, tt, one := writingFixture(t, "一")
	vol, err := a.Store.NewChapter("", "第一卷", true)
	if err != nil {
		t.Fatal(err)
	}
	two, err := a.Store.NewChapter(vol, "第二章", false)
	if err != nil {
		t.Fatal(err)
	}
	a.warn(a.loadTree())
	step := func(key ui.Key, want string) {
		t.Helper()
		shownEditor(a).Focus()
		tt.Key(ui.Cmd, key)
		tt.Frame()
		if shownChapter(a) != want {
			t.Fatalf("at %s, want %s", shownChapter(a), want)
		}
	}
	step(ui.KeyPageDown, two)
	step(ui.KeyPageDown, two)
	step(ui.KeyPageUp, one)
	step(ui.KeyPageUp, one)
	a.openNode(vol)
	tt.Frame()
	a.stepChapter(1)
	if shownChapter(a) != two {
		t.Fatal("from a folder down is not its first chapter", shownChapter(a))
	}
	a.openNode(vol)
	a.stepChapter(-1)
	if shownChapter(a) != one {
		t.Fatal("from a folder up is not the chapter before it", shownChapter(a))
	}
}

// A chapter opens with the caret where it was left, also after the work
// is opened again.
func TestChapterOpensWhereItWasLeft(t *testing.T) {
	a, tt, one := writingFixture(t, "一二三四五六")
	two, err := a.Store.NewChapter("", "第二章", false)
	if err != nil {
		t.Fatal(err)
	}
	a.warn(a.loadTree())
	shownEditor(a).MoveTo(4, false)
	a.openNode(two)
	tt.Frame()
	a.openNode(one)
	tt.Frame()
	if got := shownEditor(a).Caret(); got != 4 {
		t.Fatal("caret at", got)
	}
	if savedPrefs(t, a).Carets[one] != 4 {
		t.Fatal("the caret is not kept with the work", savedPrefs(t, a).Carets)
	}
	a.write.carets = nil
	a.loadPrefs()
	a.openNode(two)
	a.openNode(one)
	if got := shownEditor(a).Caret(); got != 4 {
		t.Fatal("caret after opening again at", got)
	}
}

// The prose size stays within bounds and is kept with the work.
func TestSizeTheText(t *testing.T) {
	a, tt, one := writingFixture(t, "一")
	shownEditor(a).Focus()
	tt.Frame()
	a.sizeText(1)
	a.sizeText(1)
	tt.Frame()
	if shownEditor(a).FontSize() != 19 || savedPrefs(t, a).TextSize != 19 {
		t.Fatal("size", shownEditor(a).FontSize(), savedPrefs(t, a).TextSize)
	}
	a.loadPrefs()
	a.openNode(one)
	if shownEditor(a).FontSize() != 19 {
		t.Fatal("a chapter opens at", shownEditor(a).FontSize())
	}
	for range 20 {
		a.sizeText(-1)
	}
	if shownEditor(a).FontSize() != kit.TextSizeMin {
		t.Fatal("below the least", shownEditor(a).FontSize())
	}
	shownEditor(a).Focus()
	tt.Frame()
	a.sizeText(0)
	tt.Frame()
	if shownEditor(a).FontSize() != kit.TextSizeDefault || savedPrefs(t, a).TextSize != 0 {
		t.Fatal("not back", shownEditor(a).FontSize())
	}
}

// The title bar's arrows go back and forth between the chapters opened, as
// a browser's; opening another drops what was ahead, and a chapter deleted
// is passed over.
func TestChapterHistory(t *testing.T) {
	a, tt, one := writingFixture(t, "一。")
	two, err := a.Store.NewChapter("", "第二章", false)
	if err != nil {
		t.Fatal(err)
	}
	three, err := a.Store.NewChapter("", "第三章", false)
	if err != nil {
		t.Fatal(err)
	}
	if err = a.loadTree(); err != nil {
		t.Fatal(err)
	}
	a.openNode(two)
	a.openNode(three)
	tt.Frame()
	step := func(label, want string) {
		t.Helper()
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		if shownChapter(a) != want {
			t.Fatalf("after %s: shown %q, want %q", label, shownChapter(a), want)
		}
	}
	step("上一个打开的章节", two)
	step("上一个打开的章节", one)
	step("下一个打开的章节", two)
	a.openNode(three)
	if len(a.write.forward) != 0 {
		t.Fatalf("forward = %v after opening another", a.write.forward)
	}
	if err = a.Store.TrashChapter(two); err != nil {
		t.Fatal(err)
	}
	if err = a.loadTree(); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	step("上一个打开的章节", one) // 第二章 is gone; 第一章 was before it
}
