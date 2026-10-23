package desk

import (
	"errors"
	"testing"

	"github.com/egoist/mygo/ui"
	"writer/aiport"
)

// Each way into a dialog from the prose gives the keyboard back to the
// prose as the dialog closes, by Escape or by its own button, as DESIGN.md
// asks: the button, menu or bar that opened it never took the keyboard.
func TestDialogsGiveTheKeyboardBack(t *testing.T) {
	a, tt, _ := writingFixture(t, "整条街的灯是同时灭的。")
	click := func(labels ...string) func() {
		return func() {
			for _, l := range labels {
				if err := tt.Click(l); err != nil {
					t.Fatal(l, err)
				}
				tt.Frame()
			}
		}
	}
	escape := func() { tt.Key(0, ui.KeyEscape) }
	for _, d := range []struct {
		name        string
		open, close func()
	}{
		{"齿轮", click("设置"), escape},
		{"侧栏的回收站", click("回收站"), escape},
		{"回收站的关闭", click("回收站"), click("关闭")},
		{"菜单里的设置", click("菜单栏-文件", "设置…"), escape},
		{"快照", click("菜单栏-文件", "快照"), escape},
		{"快照的关闭", click("菜单栏-文件", "快照"), click("关闭")},
		{"导出", click("菜单栏-文件", "导出…"), escape},
		{"快捷键", click("菜单栏-帮助", "快捷键…"), escape},
		{"加入设定", func() { shownEditor(a).Select(0, 3); tt.Frame(); click("加入设定")() }, escape},
		{"提示里的打开设置", func() {
			a.SetError(&aiport.SetupError{Err: errors.New("尚未选择讨论模型")})
			tt.Frame()
			click("打开设置")()
		}, escape},
	} {
		shownEditor(a).Focus()
		tt.Frame()
		d.open()
		tt.Frame()
		if tt.Focused("正文") && !a.bk.prefs && !a.keysOpen && !a.write.trashOpen && !a.write.snapsOpen && !a.exporting.open && !a.write.takeOpen {
			t.Fatalf("%s: no dialog opened", d.name)
		}
		d.close()
		tt.Frame()
		tt.Frame()
		tt.Frame()
		tt.Frame()
		if !tt.Focused("正文") {
			t.Errorf("%s: the keyboard did not come back to the prose", d.name)
		}
	}
}

// Only what opens a dialog or a menu keeps the keyboard where it is: a
// button that does something else takes it, and a chapter being renamed
// is named as it goes, as before.
func TestARenameEndsAtAnotherButton(t *testing.T) {
	a, tt, one := writingFixture(t, "整条街的灯。")
	w := &a.write
	w.renaming, w.title, w.titleWas = one, "第一章", "第一章"
	tt.Frame()
	tt.Frame()
	tt.Type(" 停电")
	if err := tt.Click("新建章节"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if got := chapters(a).Root.Children[0].Title; got != "第一章 停电" {
		t.Fatalf("the rename was lost: %q", got)
	}
}

// A question over the settings shades the window once: the page behind
// is as dark as with the settings alone, and the settings under the
// question as dark as that.
func TestAQuestionOverTheSettingsShadesOnce(t *testing.T) {
	a, tt, _ := writingFixture(t, "整条街的灯。")
	for _, dark := range []bool{false, true} {
		tt.SetDark(dark)
		a.openPrefs("AI")
		tt.Frame()
		tt.Frame()
		img := tt.Image()
		b := img.Bounds()
		corner := img.At(b.Min.X+3, b.Max.Y-3) // the page, outside the settings
		a.bk.askAI = true
		tt.Frame()
		tt.Frame()
		if got := tt.Image().At(b.Min.X+3, b.Max.Y-3); got != corner {
			t.Errorf("dark %v: the page under two dialogs is %v, under one %v", dark, got, corner)
		}
		a.bk.askAI, a.bk.prefs = false, false
		tt.Frame()
	}
}

// The words a talk is started with are in its box with the caret after
// them, to go on with them, as the prototype puts it.
func TestAStarterLeavesTheCaretAfterIt(t *testing.T) {
	a, tt, _ := writingFixture(t, "整条街的灯。")
	if err := tt.Click("讨论"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("哪些设定还没想清楚"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	tt.Type("，她怕黑")
	if a.input != "哪些设定还没想清楚，她怕黑" {
		t.Fatalf("typed into the words offered at %q", a.input)
	}
}
