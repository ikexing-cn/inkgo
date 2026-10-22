package desk

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/workspace"
)

// quick makes the writing page save at once and look at the disk only
// when a test asks.
func quick(t *testing.T) {
	saved, checked := saveAfter, checkEvery
	saveAfter, checkEvery = 0, time.Hour
	t.Cleanup(func() { saveAfter, checkEvery = saved, checked })
}

func disk(t *testing.T, a *App, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(a.Store.Root(), "chapters", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// The author makes a chapter, names it and writes: the text is on disk
// once they pause, with no saving to do.
func TestWritingSavesAsTheAuthorPauses(t *testing.T) {
	quick(t)
	a, tt, _ := fixture(t)
	if err := tt.Click("正文"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("暂无章节") {
		t.Fatal(tt.Texts())
	}
	if err := tt.Click("新建第一章"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	// The name is typed in the row itself, which stays as high as the others
	// and has no button of its own: Enter settles it.
	if !tt.Focused("章节标题") {
		t.Fatal("the name of a new chapter is not ready to type")
	}
	if box, ok := tt.Find("章节标题"); !ok || box.H > hRow {
		t.Fatalf("the box for the name is higher than a row: %v", box)
	}
	tt.Type("第一章：雨夜")
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if !tt.HasText("第一章：雨夜") || a.write.renaming != "" {
		t.Fatal("the title did not take", tt.Texts())
	}
	tt.Frame()
	if !tt.Focused("正文") {
		t.Fatal("named, the chapter does not take the caret")
	}
	shownEditor(a).Focus()
	tt.Frame()
	tt.Type("雨下了一夜。")
	tt.Key(0, ui.KeyEnter)
	tt.Type("她没有回答。")
	tt.Frame()
	if got := disk(t, a, "第一章：雨夜.txt"); got != "雨下了一夜。\n她没有回答。" {
		t.Fatalf("on disk %q", got)
	}
	if !tt.HasText("本章 12 字 · 全书 12 字") {
		t.Fatal(tt.Texts())
	}
	a.write.outlineOpen = true
	tt.Frame()
	setChapterOutline(a, "她在雨夜里等人")
	tt.Frame()
	if got := disk(t, a, "第一章：雨夜.outline.txt"); got != "她在雨夜里等人" {
		t.Fatalf("outline %q", got)
	}
	// Coming back opens the chapter last written.
	tt.Click("讨论")
	tt.Frame()
	b := newApp(a.ai.ConfigFile())
	if err := b.Open(a.Store.Root()); err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	bt := ui.NewTester(b.View, 1266, 780)
	bt.Frame()
	bt.Click("正文")
	bt.Frame()
	if b.write.ed == nil || b.write.ed.Doc.String() != "雨下了一夜。\n她没有回答。" {
		t.Fatal("the chapter last open did not open again")
	}
}

// A chapter changed outside while open asks which side stays; the other
// becomes a snapshot.
func TestOutsideChangeAsksTheAuthor(t *testing.T) {
	a, tt, id := writingFixture(t, "原来的。")
	p := filepath.Join(a.Store.Root(), "chapters", "第一章.txt")
	if err := os.WriteFile(p, []byte("　　别处改的。"), 0644); err != nil {
		t.Fatal(err)
	}
	markUnchecked(a)
	tt.Frame()
	if !tt.HasText("本章文件已被其他程序修改。请选择保留的版本，另一版本将存为快照。") {
		t.Fatal(tt.Texts())
	}
	if err := tt.Click("使用磁盘版本"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := shownEditor(a).Doc.String(); got != "别处改的。" {
		t.Fatalf("editor %q", got)
	}
	// The snapshot of opening it already holds the editor's text.
	snaps, _ := a.Store.Snapshots(id)
	if text, _ := a.Store.SnapshotText(id, snaps[0].Name); text != "原来的。" {
		t.Fatalf("the editor's text was not kept: %q", text)
	}
	if err := os.WriteFile(p, []byte("又改了。"), 0644); err != nil {
		t.Fatal(err)
	}
	markUnchecked(a)
	tt.Frame()
	if err := tt.Click("保留当前版本"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := disk(t, a, "第一章.txt"); got != "别处改的。" {
		t.Fatalf("on disk %q", got)
	}
	snaps, _ = a.Store.Snapshots(id)
	if text, _ := a.Store.SnapshotText(id, snaps[0].Name); text != "又改了。" || snaps[0].Reason != workspace.SnapExternal {
		t.Fatalf("the disk's text was not kept: %q", text)
	}
	// Only the indent differing, as the editor saves it, is no change.
	if err := os.WriteFile(p, []byte("　　别处改的。"), 0644); err != nil {
		t.Fatal(err)
	}
	markUnchecked(a)
	tt.Frame()
	if diskChanged(a) {
		t.Fatal("an indent alone asked the author")
	}
	// Renamed outside, the chapter is gone as far as the work knows; the
	// editor's text can still be saved.
	if err := os.Rename(p, filepath.Join(a.Store.Root(), "chapters", "改名.txt")); err != nil {
		t.Fatal(err)
	}
	markUnchecked(a)
	tt.Frame()
	if err := tt.Click("存为新章节"); err != nil {
		t.Fatal(err, tt.Texts())
	}
	tt.Frame()
	if got := disk(t, a, "第一章.txt"); got != "别处改的。" || chapterLost(a) {
		t.Fatalf("saved %q", got)
	}
}

// Ctrl+S saves the chapter at once and says so.
func TestCtrlSSavesNow(t *testing.T) {
	a, tt, one := writingFixture(t, "一")
	shownEditor(a).Focus()
	tt.Frame()
	shownEditor(a).Insert("二")
	tt.Key(ui.Cmd, ui.KeyS)
	tt.Frame()
	ch, err := a.Store.ReadChapter(one)
	if err != nil {
		t.Fatal(err)
	}
	if ch.Text != shownEditor(a).Doc.String() || !strings.Contains(ch.Text, "二") {
		t.Fatalf("disk has %q", ch.Text)
	}
	if !strings.HasPrefix(a.notice, "已保存") {
		t.Fatal(a.notice)
	}
}

// A chapter changed outside is not written over by the save that comes
// before the next look at the disk: the author is asked, as when the look
// finds it.
func TestSavingDoesNotWriteOverAnOutsideChange(t *testing.T) {
	a, tt, id := writingFixture(t, "原来的。")
	p := filepath.Join(a.Store.Root(), "chapters", "第一章.txt")
	if err := os.WriteFile(p, []byte("别处改的，很重要。"), 0644); err != nil {
		t.Fatal(err)
	}
	shownEditor(a).Focus()
	tt.Frame()
	tt.Type("新字")
	tt.Frame()
	tt.Frame()
	if got := disk(t, a, "第一章.txt"); got != "别处改的，很重要。" {
		t.Fatalf("the save wrote over the outside change: %q", got)
	}
	if !tt.HasText("本章文件已被其他程序修改。请选择保留的版本，另一版本将存为快照。") {
		t.Fatal(tt.Texts())
	}
	// Keeping the editor's writes it, the disk's kept as a snapshot.
	if err := tt.Click("保留当前版本"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := disk(t, a, "第一章.txt"); got != "新字原来的。" {
		t.Fatalf("on disk %q", got)
	}
	snaps, _ := a.Store.Snapshots(id)
	if text, _ := a.Store.SnapshotText(id, snaps[0].Name); text != "别处改的，很重要。" || snaps[0].Reason != workspace.SnapExternal {
		t.Fatalf("the disk's text was not kept: %q", text)
	}
	// And the next save goes through.
	shownEditor(a).Focus()
	tt.Frame()
	tt.Type("再写")
	tt.Frame()
	tt.Frame()
	if got := disk(t, a, "第一章.txt"); got != "新字再写原来的。" || diskChanged(a) {
		t.Fatalf("on disk %q", got)
	}
}

// The same words under a new date, as a syncing program leaves them, are
// no change: the save goes through.
func TestSavingOverAFileOnlyTouched(t *testing.T) {
	a, tt, _ := writingFixture(t, "原来的。")
	p := filepath.Join(a.Store.Root(), "chapters", "第一章.txt")
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(p, later, later); err != nil {
		t.Fatal(err)
	}
	shownEditor(a).Focus()
	tt.Frame()
	tt.Type("新字")
	tt.Frame()
	tt.Frame()
	if got := disk(t, a, "第一章.txt"); got != "新字原来的。" || diskChanged(a) || a.errorText() != "" {
		t.Fatalf("on disk %q, error %q", got, a.errorText())
	}
}

// Words the work is left with, the disk holding others the author had not
// chosen between, are kept as a snapshot.
func TestLeavingWithAnOutsideChangeKeepsTheEditorsWords(t *testing.T) {
	a, tt, id := writingFixture(t, "原来的。")
	p := filepath.Join(a.Store.Root(), "chapters", "第一章.txt")
	shownEditor(a).Focus()
	tt.Frame()
	if err := os.WriteFile(p, []byte("别处改的，很重要。"), 0644); err != nil {
		t.Fatal(err)
	}
	saveAfter = time.Hour // the words are not saved before the work is left
	tt.Type("新字")
	tt.Frame()
	store := a.Store
	a.flush()
	a.keepUnsettled()
	if got := disk(t, a, "第一章.txt"); got != "别处改的，很重要。" {
		t.Fatalf("on disk %q", got)
	}
	snaps, _ := store.Snapshots(id)
	if text, _ := store.SnapshotText(id, snaps[0].Name); text != "新字原来的。" || snaps[0].Reason != workspace.SnapExternal {
		t.Fatalf("the editor's words were not kept: %q (%s)", text, snaps[0].Reason)
	}
}
