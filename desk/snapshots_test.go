package desk

import (
	"fmt"
	"strings"
	"testing"

	"writer/workspace"
)

// Snapshots list what was kept; restoring one keeps the text it replaces.
func TestRestoreASnapshot(t *testing.T) {
	a, tt, id := writingFixture(t, "第一版。")
	shownEditor(a).MoveTo(shownEditor(a).Doc.Len(), false)
	shownEditor(a).Insert("加了一句。")
	shownEditor(a).Select(0, 3)
	tt.Frame()
	if err := clickMenu(tt, "文件", "菜单-快照"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("打开时 · 4 字") || !tt.HasText("加了一句。") {
		t.Fatal(tt.Texts())
	}
	if err := tt.Click("恢复这个版本"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := disk(t, a, "第一章.txt"); got != "第一版。" || shownEditor(a).Doc.String() != "第一版。" {
		t.Fatalf("restored %q", got)
	}
	if s, e := shownEditor(a).Selection(); s != e || tt.HasText("AI 改写") || tt.HasText("加入设定") {
		t.Fatal("the selection bar remained after restoring", tt.Texts())
	}
	snaps, _ := a.Store.Snapshots(id)
	if snaps[0].Reason != workspace.SnapRestore {
		t.Fatalf("snapshots %+v", snaps)
	}
	if text, _ := a.Store.SnapshotText(id, snaps[0].Name); text != "第一版。加了一句。" {
		t.Fatalf("the text replaced was not kept: %q", text)
	}
}

// A snapshot the same as the chapter says so and has nothing to restore.
func TestASnapshotLikeTheChapter(t *testing.T) {
	a, tt, _ := writingFixture(t, "第一版。")
	if err := clickMenu(tt, "文件", "菜单-快照"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("该快照与当前正文相同") {
		t.Fatal(tt.Texts())
	}
	if err := tt.Click("恢复这个版本"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.write.snapsOpen || noticeOf(a) != "" {
		t.Fatal("restored the same text", noticeOf(a))
	}
}

// The snapshot shows a paragraph to an element, and they select and copy
// as the one text they are.
func TestASnapshotSelectsAcrossItsParagraphs(t *testing.T) {
	old := "第一段旧的。\n\n第三段。\n第四段。"
	a, tt, id := writingFixture(t, "第一段新的。\n\n第三段。\n第四段。")
	if _, err := a.Store.TakeSnapshot(id, workspace.SnapInterval, old, nil); err != nil {
		t.Fatal(err)
	}
	a.openSnapshots()
	tt.Frame()
	shown := &a.write.snapShown
	if shown.same || shown.long || len(shown.paras) != 4 || len(shown.paras[0]) != 4 || len(shown.paras[1]) != 0 {
		t.Fatalf("compared: %+v", shown.paras)
	}
	// Made as the snapshot or the comparing changes, and not in each frame.
	first := &shown.paras[0][0]
	tt.Move(500, 300)
	tt.Frame()
	if first != &a.write.snapShown.paras[0][0] {
		t.Fatal("the comparison was made again in a frame nothing changed in")
	}
	a.write.compare = false
	tt.Frame()
	if len(shown.paras) != 4 || len(shown.paras[0]) != 1 || shown.paras[0][0].Text != "第一段旧的。" {
		t.Fatalf("plain: %+v", shown.paras)
	}
	from, ok := tt.Find("第一段旧的。")
	to, ok2 := tt.Find("第四段。")
	third, ok3 := tt.Find("第三段。")
	if !ok || !ok2 || !ok3 || to.Y <= third.Y {
		t.Fatal(from, to, tt.Texts())
	}
	// The empty line between takes the room of a line.
	if line := to.Y - third.Y; third.Y-from.Y < 2*line-1 || third.Y-from.Y > 2*line+1 {
		t.Fatalf("the first at %v, the third at %v, a line is %v", from.Y, third.Y, line)
	}
	tt.Press(from.X+1, from.Y+from.H/2)
	tt.Move(to.X+to.W-1, to.Y+to.H/2)
	tt.Frame()
	tt.Release(to.X+to.W-1, to.Y+to.H/2)
	tt.Frame()
	tt.Command("copy")
	tt.Frame()
	if got := tt.Clipboard(); got != old {
		t.Fatalf("copied %q", got)
	}
}

// A snapshot too long to build whole builds the paragraphs in view, and
// restores as any other.
func TestALongSnapshot(t *testing.T) {
	var old strings.Builder
	n := 0
	for ; old.Len() <= snapWhole; n++ {
		fmt.Fprintf(&old, "第%d段，街上的灯一盏一盏地灭下去。\n", n)
	}
	last := fmt.Sprintf("第%d段，街上的灯一盏一盏地灭下去。", n-1)
	a, tt, id := writingFixture(t, "只有一句。")
	if _, err := a.Store.TakeSnapshot(id, workspace.SnapInterval, old.String(), nil); err != nil {
		t.Fatal(err)
	}
	a.openSnapshots()
	a.write.compare = false
	tt.Frame()
	tt.Frame()
	shown := &a.write.snapShown
	if !shown.long || len(shown.paras) != n+1 {
		t.Fatalf("long %v, %d paragraphs of %d", shown.long, len(shown.paras), n+1)
	}
	if !tt.HasText("第0段，街上的灯一盏一盏地灭下去。") || tt.HasText(last) {
		t.Fatal("the paragraphs in view, and only those, are built")
	}
	// It scrolls to the others.
	at, _ := tt.Find("第0段，街上的灯一盏一盏地灭下去。")
	for range 4 {
		tt.Scroll(at.X+10, at.Y+10, 0, 600)
		tt.Frame()
	}
	if first, _ := shown.list.Visible(); first == 0 || tt.HasText("第0段，街上的灯一盏一盏地灭下去。") {
		t.Fatal("the long snapshot did not scroll: its first paragraph in view is", first)
	}
	if err := tt.Click("恢复这个版本"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := disk(t, a, "第一章.txt"); got != old.String() {
		t.Fatalf("restored %d bytes of %d", len(got), old.Len())
	}
}
