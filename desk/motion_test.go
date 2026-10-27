package desk

import (
	"image"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/desk/kit"
	"writer/internal/testshot"
)

func paperLeft(a *App) float32 {
	box, _ := shownEditor(a).RangeBoxUnclipped(0, 1)
	// A paragraph's first line is indented by two characters. The editor is
	// laid out where it ends; folding, its picture is moved by the offset.
	return a.LastProse.X + box.X - 2*shownEditor(a).FontSize() + a.fold.offset
}

func paperLine(a *App) (int, float32) {
	ed := shownEditor(a)
	first, _ := ed.RangeBoxUnclipped(0, 1)
	last := first
	for i := 1; i < ed.Doc.Len(); i++ {
		box, _ := ed.RangeBoxUnclipped(i, i+1)
		if box.Y != first.Y {
			return i, last.X - first.X
		}
		last = box
	}
	return ed.Doc.Len(), last.X - first.X
}

func TestFoldingMovesThePaperAndRowsWithoutStretching(t *testing.T) {
	a, tt, _ := writingFixture(t, strings.Repeat("雨从傍晚开始下。", 80))
	tt.SetPreferences(ui.Preferences{})
	tt.Frame()
	old := paperLeft(a)
	click(t, tt, "收起右栏")
	if got := paperLeft(a); math.Abs(float64(got-old)) > 0.5 {
		t.Fatalf("first frame jumped from %v to %v", old, got)
	}
	end, width := paperLine(a)
	last := old
	for range 3 {
		tt.Advance(50 * time.Millisecond)
		got := paperLeft(a)
		if got <= last || a.fold.offset >= 0 {
			t.Fatalf("paper did not move toward its new place: %v after %v", got, last)
		}
		pane := find(t, tt, "右栏标签行")
		want := a.fold.from.width - (a.fold.from.paneWidth+1)*(1-kit.Out(a.fold.progress))
		if math.Abs(float64(pane.X-1-want)) > 0.5 {
			t.Fatalf("pane edge %v, paper edge %v", pane.X-1, want)
		}
		nowEnd, nowWidth := paperLine(a)
		if math.Abs(float64(nowWidth-width)) > 0.01 || nowEnd != end {
			t.Fatalf("line width or break changed: %v at %d, was %v at %d", nowWidth, nowEnd, width, end)
		}
		arrow := find(t, tt, "上一个打开的章节图形")
		if math.Abs(float64(arrow.X+arrow.W*9/24-got)) > 3 {
			t.Fatalf("tabs at %v, paper at %v", arrow.X, got)
		}
		last = got
	}
	tt.Advance(100 * time.Millisecond)
	if a.fold.offset != 0 || tt.FramePending() {
		t.Fatal("finished fold keeps requesting frames")
	}
	if _, ok := tt.Find("右栏标签行"); ok {
		t.Fatal("closed pane is still drawn")
	}
	click(t, tt, "展开右栏")
	tt.Advance(250 * time.Millisecond)
	if a.fold.offset != 0 {
		t.Fatal("opening did not settle")
	}
}

// The sidebar coming back puts the prose's place to the right of where the
// text still is: the text is drawn whole on its way there, and follows
// the same ease-out as the column.
func TestUnfoldingDrawsTheProseWholeAndEasesOut(t *testing.T) {
	a, tt, _ := writingFixture(t, strings.Repeat("雨从傍晚开始下。", 80))
	tt.SetPreferences(ui.Preferences{})
	click(t, tt, "收起侧栏")
	tt.Advance(time.Second)
	from := paperLeft(a)
	click(t, tt, "展开侧栏")
	tt.Advance(kit.MoveTime / 5)
	got, end := paperLeft(a), paperLeft(a)-a.fold.offset
	if got >= a.LastProse.X {
		t.Fatalf("the text at %v is not over the sidebar, the editor at %v", got, a.LastProse.X)
	}
	if way := (got - from) / (end - from); math.Abs(float64(way-kit.Out(a.fold.progress))) > 0.001 {
		t.Fatalf("a fifth of the time in, the paper had gone %.2f of its way", way)
	}
	ed := shownEditor(a)
	box, _ := ed.RangeBoxUnclipped(2, 3) // the first character, after the indent
	at := image.Rect(int(got+2*ed.FontSize()), int(a.LastProse.Y+box.Y), int(got+3*ed.FontSize()), int(a.LastProse.Y+box.Y+box.H))
	img, ink := tt.Image(), 0
	for y := at.Min.Y; y < at.Max.Y; y++ {
		for x := at.Min.X; x < at.Max.X; x++ {
			if c := img.RGBAAt(x, y); c.R < 128 && c.G < 128 && c.B < 128 {
				ink++
			}
		}
	}
	if ink < 10 {
		t.Fatalf("the first character at %v is cut off", at)
	}
}

func TestOnlyExplicitSwitchesAnimateTheColumns(t *testing.T) {
	a, tt, _ := writingFixture(t, "雨从傍晚开始下。")
	tt.SetPreferences(ui.Preferences{})
	tt.Frame()
	for _, change := range []func(){
		func() { a.doc.width = 440 },
		func() { a.setDocHidden(true) },
		func() { a.setDocHidden(false) },
		func() { a.write.focus = true },
		func() { a.write.focus = false },
		func() { tt.SetSize(1400, 800) },
	} {
		change()
		tt.Frame()
		if a.fold.offset != 0 || a.fold.progress != 1 {
			t.Fatal("an implicit layout change animated")
		}
	}
	edge := find(t, tt, "调整右栏宽度")
	tt.Press(edge.X+4, edge.Y+100)
	tt.Move(edge.X-30, edge.Y+100)
	if a.fold.offset != 0 {
		t.Fatal("dragging the pane animated")
	}
	tt.Release(edge.X-30, edge.Y+100)
	click(t, tt, "收起侧栏")
	tt.Advance(60 * time.Millisecond)
	if a.fold.offset == 0 {
		t.Fatal("explicit sidebar switch did not animate")
	}
	tt.SetSize(1500, 800)
	tt.Frame()
	if a.fold.offset != 0 {
		t.Fatal("resize did not cancel a running fold")
	}
}

func TestReducedMotionPutsColumnsAndNotesInPlaceAtOnce(t *testing.T) {
	a, tt, _ := marginFixture(t, true)
	for _, label := range []string{"收起右栏", "展开右栏", "收起侧栏", "展开侧栏"} {
		click(t, tt, label)
		if a.fold.offset != 0 || a.fold.progress != 1 {
			t.Fatalf("%s animated", label)
		}
	}
	click(t, tt, "批注-check-name-无需修改")
	if _, ok := tt.Find("批注-check-name"); ok {
		t.Fatal("reduced motion kept the removed note")
	}
	marginPositions(t, a, tt)
	click(t, tt, "撤销提示")
	marginPositions(t, a, tt)
	if len(a.write.margin.departed) != 0 {
		t.Fatal("reduced motion retained a departure")
	}
}

func TestRemovingAMarginNoteMovesThoseBelowIt(t *testing.T) {
	a, tt, _ := marginFixture(t, true)
	tt.SetPreferences(ui.Preferences{})
	tt.Frame()
	before := find(t, tt, "批注-check-conflict")
	click(t, tt, "批注-check-name-无需修改")
	if _, ok := tt.Find("批注-check-name"); !ok {
		t.Fatal("removed note vanished before its exit")
	}
	tt.Move(0, 0)
	tt.Advance(100 * time.Millisecond)
	mid := find(t, tt, "批注-check-conflict")
	if mid.Y >= before.Y {
		t.Fatalf("next note did not rise: %v, was %v", mid, before)
	}
	if _, ok := tt.Find("批注-check-name"); !ok {
		t.Fatal("departing note is not drawn halfway")
	}
	tt.Advance(150 * time.Millisecond)
	end := find(t, tt, "批注-check-conflict")
	if mid.Y <= end.Y {
		t.Fatalf("next note jumped to its end: %v, end %v", mid, end)
	}
	if _, ok := tt.Find("批注-check-name"); ok {
		t.Fatal("departure outlived 200ms")
	}
	marginPositions(t, a, tt)
	click(t, tt, "撤销提示")
	if _, ok := tt.Find("批注-check-name"); !ok {
		t.Fatal("undo did not return the note")
	}
	tt.Move(0, 0)
	tt.Advance(250 * time.Millisecond)
	marginPositions(t, a, tt)
	if tt.FramePending() {
		t.Fatal("notes kept asking for frames after settling")
	}
}

func TestShotMotion(t *testing.T) {
	testshot.Dir(t)
	_, tt, _ := marginFixture(t, true)
	tt.SetPreferences(ui.Preferences{})
	tt.Frame()
	click(t, tt, "收起右栏")
	tt.Advance(100 * time.Millisecond)
	testshot.Save(t, "fold-pane-half.png", tt.Image())
	tt.Advance(kit.MoveTime)
	click(t, tt, "展开右栏")
	tt.Advance(100 * time.Millisecond)
	testshot.Save(t, "unfold-pane-half.png", tt.Image())
	tt.Advance(kit.MoveTime)
	click(t, tt, "批注-check-name-无需修改")
	tt.Move(0, 0)
	tt.Advance(100 * time.Millisecond)
	testshot.Save(t, "remove-note-half.png", tt.Image())
}

// sameIn says two pictures of the window agree inside r, widened by the
// distance a note travels.
func sameIn(x, y *image.RGBA, r ui.Rect) bool {
	in := image.Rect(int(r.X)-int(kit.NoteExit), int(r.Y), int(r.X+r.W)+int(kit.NoteExit), int(r.Y+r.H)).Intersect(x.Bounds())
	for py := in.Min.Y; py < in.Max.Y; py++ {
		for px := in.Min.X; px < in.Max.X; px++ {
			if x.RGBAAt(px, py) != y.RGBAAt(px, py) {
				return false
			}
		}
	}
	return true
}

func TestMarginNotesArriveInTurnWithinAQuarterSecond(t *testing.T) {
	a, tt, _ := marginFixture(t, true)
	tt.SetPreferences(ui.Preferences{})
	tt.Move(0, 0)
	a.doc.tab = paneSetting
	tt.Frame()
	a.doc.tab = paneMargin
	tt.Frame()
	if !tt.FramePending() {
		t.Fatal("the notes were in place at once")
	}
	tt.Advance(170 * time.Millisecond)
	part := tt.Image()
	list := a.marginNotes()
	if len(list) < 5 {
		t.Fatal("the fixture needs notes past the fourth", len(list))
	}
	tt.Advance(80 * time.Millisecond)
	done := tt.Image()
	// At 170 ms the first, 160 ms long, has stopped; the second, 30 ms
	// later, has not.
	first, _ := tt.Find("批注-" + list[0].id)
	second, _ := tt.Find("批注-" + list[1].id)
	if !sameIn(part, done, first) || sameIn(part, done, second) {
		t.Fatal("the notes did not come in one after another")
	}
	// mygo asks for one frame more once a motion has ended.
	tt.Advance(100 * time.Millisecond)
	if !sameIn(done, tt.Image(), ui.Rect{W: 1266, H: 780}) || tt.FramePending() {
		t.Fatal("the last note was still moving after a quarter second")
	}
}

// settledIn says nothing in r moves after now (the caret elsewhere may
// blink), and nothing asks for frames once mygo's frame past the end has
// been drawn.
func settledIn(tt *ui.Tester, r ui.Rect) bool {
	now := tt.Image()
	tt.Advance(100 * time.Millisecond)
	return sameIn(now, tt.Image(), r) && !tt.FramePending()
}

// menuBox is where the open menu's rows are.
func menuBox(tt *ui.Tester) ui.Rect {
	var box ui.Rect
	for _, label := range rows(tt) {
		r, _ := tt.Find("菜单-" + label)
		if box.W == 0 {
			box = r
		}
		x, y := min(box.X, r.X), min(box.Y, r.Y)
		box = ui.Rect{X: x, Y: y, W: max(box.X+box.W, r.X+r.W) - x, H: max(box.Y+box.H, r.Y+r.H) - y}
	}
	return box
}

// A popover is placed, and starts moving, a frame after it opens.
const placed = time.Second / 60

// Frames with a fading panel are slow to draw in the tester, so only the
// first menu is followed to its end; the rest show they start to move
// and are gone the frame they close.
func TestMenusAndDialogsComeInOnceAndCloseAtOnce(t *testing.T) {
	_, tt, _ := writingFixture(t, "雨从傍晚开始下。")
	tt.SetPreferences(ui.Preferences{})
	tt.Frame()
	shut := func(what string, gone func() bool) {
		t.Helper()
		tt.Key(0, ui.KeyEscape)
		tt.Frame()
		if !gone() || tt.FramePending() {
			t.Fatalf("the %s did not close at once", what)
		}
	}
	noMenu := func() bool { return len(rows(tt)) == 0 }
	tt.Click("文件")
	tt.Frame()
	tt.Advance(kit.PopTime + placed)
	if !settledIn(tt, menuBox(tt)) {
		t.Fatal("the menu kept moving")
	}
	shut("menu", noMenu)
	for _, open := range []func(){
		func() { tt.Click("文件") }, // each opening pops in again
		func() { tt.RightClickAt(550, 300) },
	} {
		open()
		tt.Frame()
		if len(rows(tt)) == 0 || !tt.FramePending() {
			t.Fatal("the menu did not pop in", rows(tt))
		}
		shut("menu", noMenu)
	}
	if err := clickMenu(tt, "文件", "菜单-设置"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if _, ok := tt.Find("关闭设置"); !ok || !tt.FramePending() {
		t.Fatal("the dialog did not come in")
	}
	shut("dialog", func() bool { _, ok := tt.Find("关闭设置"); return !ok })
}
