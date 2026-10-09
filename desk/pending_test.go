package desk

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"writer/workspace"
)

// A foreshadowing lost, or waiting thirty chapters, is pending; choosing it
// goes to it, and ignored it stays away.
func TestPendingForeshadowing(t *testing.T) {
	a, tt, one := writingFixture(t, "井边有一把钥匙。")
	if _, err := a.Store.WriteChapter(one, "井边有一把钥匙。", []workspace.ChapterMark{{Kind: workspace.MarkForeshadow, Start: 5, End: 7}}); err != nil {
		t.Fatal(err)
	}
	var two string
	for i := 2; i <= 31; i++ {
		id, err := a.Store.NewChapter("", fmt.Sprintf("第%d章", i), false)
		if err != nil {
			t.Fatal(err)
		}
		if i == 2 {
			two = id
		}
	}
	// Thirty chapters after it: not yet.
	if err := a.loadTree(); err != nil {
		t.Fatal(err)
	}
	a.openNode(two)
	tt.Frame()
	if tt.HasText("待处理 1") || len(a.pendingGroups()) != 0 {
		t.Fatal("pending too soon", tt.Texts())
	}
	if _, err := a.Store.WriteChapter(two, "", []workspace.ChapterMark{{Kind: workspace.MarkForeshadow, Orphaned: true, Quote: "雨夜的电话"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Store.NewChapter("", "第32章", false); err != nil {
		t.Fatal(err)
	}
	if err := a.loadTree(); err != nil {
		t.Fatal(err)
	}
	a.write.node = ""
	a.openNode(two)
	tt.Frame()
	if err := tt.Click("待处理"); err != nil {
		t.Fatal(err, tt.Texts())
	}
	tt.Frame()
	if !tt.HasText("待处理 2") || !tt.HasText("「钥匙」31 章没回收") || !tt.HasText("第2章 · 当前章") {
		t.Fatal("not listed", tt.Texts())
	}
	a.write.pendHere = 1
	tt.Frame()
	if tt.HasText("「钥匙」31 章没回收") || !tt.HasText("「雨夜的电话」找不到原句了") {
		t.Fatal("this chapter's alone", tt.Texts())
	}
	a.write.pendHere = 0
	tt.Frame()
	if err := tt.Click("「钥匙」31 章没回收"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.write.node != one || a.write.ed.SelectedText() != "钥匙" || a.write.pendOpen {
		t.Fatalf("went to %q, selecting %q", a.write.node, a.write.ed.SelectedText())
	}
	// Its words deleted as the chapter is written, it is lost at once.
	tt.Key(0, ui.KeyBackspace)
	tt.Frame()
	if err := tt.Click("待处理"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("「钥匙」找不到原句了") || !tt.HasText("第一章 · 当前章") {
		t.Fatal("not lost", tt.Texts())
	}
	if err := tt.Click("全部忽略"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if tt.HasText("待处理 2") || a.write.pendOpen {
		t.Fatal("still pending", tt.Texts())
	}
	a.write.ignored = nil
	a.loadPrefs()
	if len(a.write.ignored) != 2 {
		t.Fatalf("ignored %v", a.write.ignored)
	}
}

// TestShotPending writes screenshots of the pending list when SHOT is a
// directory: SHOT=dir go test ./desk -run ShotPending
func TestShotPending(t *testing.T) {
	dir := os.Getenv("SHOT")
	if dir == "" {
		t.Skip("set SHOT=dir")
	}
	text := "整条街的灯是同时灭的。\n林晚站在巷口，手电在工具箱最底下。她没有去摸。\n她把右手按在门框上，听见楼上有人在走动。"
	a, tt, one := writingFixture(t, text)
	if _, err := a.Store.WriteChapter(one, text, []workspace.ChapterMark{
		{Kind: workspace.MarkForeshadow, Start: 23, End: 27, Note: "红色钥匙"},
		{Kind: workspace.MarkForeshadow, Start: 30, End: 34, Note: "母亲的怀表"},
		{Kind: workspace.MarkForeshadow, Start: 36, End: 40},
		{Kind: workspace.MarkForeshadow, Orphaned: true, Quote: "雨夜的电话"},
	}); err != nil {
		t.Fatal(err)
	}
	for i := 2; i <= 42; i++ {
		if _, err := a.Store.NewChapter("", fmt.Sprintf("第%d章", i), false); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.loadTree(); err != nil {
		t.Fatal(err)
	}
	a.write.node = ""
	a.openNode(one)
	a.write.pendOpen = true
	shot := func(name string) {
		tt.Frame()
		tt.Frame()
		f, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
	tt.SetTitleBar(ui.TitleBar{Height: 40, Right: 138}) // the Windows buttons
	shot("pending-light.png")
	tt.SetDark(true)
	shot("pending-dark.png")
	tt.SetDark(false)
	a.write.pendOpen = false
	a.doc.hidden = true
	shot("titlebar-pane-hidden.png")
	a.write.moreOpen = true
	shot("toolbar-more.png")
	a.write.moreOpen = false
	a.write.ed.Select(17, 26)
	shot("toolbar-selection.png")
	tt.SetDark(true)
	shot("toolbar-selection-dark.png")
	tt.SetDark(false)
	a.write.ed.Select(0, 0)
	a.page = 1
	shot("titlebar-settings.png")
}
