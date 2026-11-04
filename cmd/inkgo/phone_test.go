package main

import (
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
	"writer/workspace"
)

// Four phone states rendered by the desktop, using the same screenshot
// path convention as the other TestLayout cases.
func TestLayoutPhonePreview(t *testing.T) {
	root := t.TempDir()
	s, err := workspace.Create(filepath.Join(root, "work"), "雨城来信", "网文", "zh-Hans")
	if err != nil {
		t.Fatal(err)
	}
	id, err := s.NewChapter("", "第87章：停电", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.WriteChapter(id, "整条街的灯是同时灭的。\n林晚把手电攥得更紧，沿着巷子往前走。", nil); err != nil {
		t.Fatal(err)
	}
	s.Close()
	a := newModuleApp(filepath.Join(root, "providers.yaml"), testModules()...)
	if err = a.Open(filepath.Join(root, "work")); err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	tt := ui.NewTester(a.View, 1266, 780)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	for range 3 {
		tt.Frame()
	}
	if err = clickMenu(tt, "视图", "菜单-手机预览"); err != nil {
		t.Fatal(err)
	}
	for range 4 {
		tt.Frame()
	}
	if !tt.HasText("第 1 屏，共 1 屏") {
		t.Fatal(tt.Texts())
	}
	shot(t, tt, "-phone-day")
	click := func(label string) {
		t.Helper()
		if err = tt.Click(label); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		tt.Frame()
	}
	click("Aa")
	click("夜间")
	click("Aa")
	shot(t, tt, "-phone-night")
	click("Aa")
	click("恢复默认")
	click("Aa")
	r := a.LastProse
	tt.Press(r.X+100, r.Y+40)
	tt.Release(r.X+100, r.Y+40)
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyEnd)
	const long = "雨水顺着屋檐往下淌，巷子尽头那盏灯晃了两下，终于灭了。她数着自己的脚步，一步，两步，三步，数到第十七步的时候，听见身后有人喊她的名字，声音很轻，像是怕惊动了什么。她没有回头，只是把手电攥得更紧，指节都发了白。"
	for range 2 {
		tt.Key(0, ui.KeyEnter)
		tt.Type(long + long)
	}
	tt.Frame()
	if !tt.HasText("2 段超过 5 行，看下一处") {
		t.Fatal(tt.Texts())
	}
	shot(t, tt, "-phone-long")
	click("滚动")
	r = a.LastProse
	tt.Scroll(r.X+r.W/2, r.Y+r.H/2, 0, 100000)
	tt.Frame()
	shot(t, tt, "-phone-scroll")
}
