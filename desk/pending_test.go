package desk

import (
	"fmt"
	"testing"

	"github.com/egoist/mygo/ui"

	"writer/internal/testshot"
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
	for i := 2; i <= 30; i++ {
		id, err := a.Store.NewChapter("", fmt.Sprintf("第%d章", i), false)
		if err != nil {
			t.Fatal(err)
		}
		if i == 2 {
			two = id
		}
	}
	// Twenty-nine chapters after it: not yet.
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
	for _, title := range []string{"第31章", "第32章"} {
		if _, err := a.Store.NewChapter("", title, false); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.loadTree(); err != nil {
		t.Fatal(err)
	}
	forgetChapter(a)
	a.openNode(two)
	tt.Frame()
	if err := tt.Click("待处理"); err != nil {
		t.Fatal(err, tt.Texts())
	}
	tt.Frame()
	if !tt.HasText("待处理 2") || !tt.HasText("「钥匙」31 章未回收") || !tt.HasText("第2章 · 本章") {
		t.Fatal("not listed", tt.Texts())
	}
	a.write.pendHere = 1
	tt.Frame()
	if tt.HasText("「钥匙」31 章未回收") || !tt.HasText("「雨夜的电话」原句已删") {
		t.Fatal("this chapter's alone", tt.Texts())
	}
	a.write.pendHere = 0
	tt.Frame()
	if err := tt.Click("「钥匙」31 章未回收"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if shownChapter(a) != one || shownEditor(a).SelectedText() != "钥匙" || a.write.pendOpen {
		t.Fatalf("went to %q, selecting %q", shownChapter(a), shownEditor(a).SelectedText())
	}
	// Its words deleted as the chapter is written, it is lost at once.
	tt.Key(0, ui.KeyBackspace)
	tt.Frame()
	if err := tt.Click("待处理"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("「钥匙」原句已删") || !tt.HasText("第一章 · 本章") {
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

func TestPendingForeshadowingOpensPane(t *testing.T) {
	for _, hidden := range []bool{false, true} {
		t.Run(fmt.Sprintf("pane hidden %v", hidden), func(t *testing.T) {
			a, tt, one := writingFixture(t, "井边有一把钥匙。")
			if _, err := a.Store.WriteChapter(one, "井边有一把钥匙。", []workspace.ChapterMark{
				{Kind: workspace.MarkForeshadow, Start: 5, End: 7},
			}); err != nil {
				t.Fatal(err)
			}
			two, err := a.Store.NewChapter("", "第二章", false)
			if err != nil {
				t.Fatal(err)
			}
			a.warn(a.loadTree())
			a.openNode(two)
			a.write.lateAfter = 1
			a.doc.hidden = hidden
			showTab(a, paneSetting)
			tt.Frame()
			groups := a.pendingGroups()
			if len(groups) != 1 || len(groups[0].items) != 1 {
				t.Fatalf("pending groups = %+v", groups)
			}
			id := a.foreshadows()[0].ID
			if err := tt.Click("待处理"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if err := tt.Click(groups[0].items[0].text); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if a.write.pendOpen || shownChapter(a) != one || shownEditor(a).SelectedText() != "钥匙" {
				t.Fatalf("pending foreshadowing: chapter %s, selected %q, list open %v", shownChapter(a), shownEditor(a).SelectedText(), a.write.pendOpen)
			}
			if hidden {
				if !a.doc.hidden || shownTab(a) != paneSetting {
					t.Fatal("the hidden pane changed")
				}
			} else if shownTab(a) != paneFore || a.write.fore.shown != id {
				t.Fatalf("pane shows %s, foreshadowing %s, want %s", shownTab(a), a.write.fore.shown, id)
			}
		})
	}
}

// TestShotPending writes screenshots of the pending list when SHOT is a
// directory: SHOT=dir go test ./desk -run ShotPending
func TestShotPending(t *testing.T) {
	testshot.Dir(t)
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
	forgetChapter(a)
	a.openNode(one)
	a.write.pendOpen = true

	tt.SetTitleBar(ui.TitleBar{Height: 40, Right: 138}) // the Windows buttons
	shot(t, tt, "pending-light.png")
	tt.SetDark(true)
	shot(t, tt, "pending-dark.png")
	tt.SetDark(false)
	a.write.pendOpen = false
	a.doc.hidden = true
	shot(t, tt, "titlebar-pane-hidden.png")
	for _, name := range []string{"文件", "编辑", "视图", "帮助"} {
		a.menu = menuState{owner: barKey + name, drop: true, seen: true}
		shot(t, tt, "titlebar-menu-"+name+".png")
	}
	a.menu = menuState{}
	a.sideHidden = true
	shot(t, tt, "titlebar-side-hidden.png")
	a.sideHidden, a.write.focus = false, true
	shot(t, tt, "titlebar-focus.png")
	a.write.focus = false
	shownEditor(a).Select(17, 26)
	shot(t, tt, "toolbar-selection.png")
	tt.SetDark(true)
	shot(t, tt, "toolbar-selection-dark.png")
	tt.SetDark(false)
	shownEditor(a).Select(0, 0)
	a.openPrefs("AI")
	shot(t, tt, "titlebar-settings.png")
}
