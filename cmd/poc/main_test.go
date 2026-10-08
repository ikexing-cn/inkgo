package main

import (
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"writer/desk"
	"writer/editor"
	"writer/workspace"
)

func TestLayoutDiscussion(t *testing.T) {
	root := t.TempDir()
	s, err := workspace.Create(filepath.Join(root, "work"), "雨城来信", "网文")
	if err != nil {
		t.Fatal(err)
	}
	session, err := s.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	user, err := s.Append(session, workspace.Event{Role: "user", Text: "我想写一个总在下雨的城市。主角修路灯，但她其实很怕黑。"})
	if err != nil {
		t.Fatal(err)
	}
	reply, err := s.Append(session, workspace.Event{Role: "assistant", ReplyTo: user.ID,
		Model: "deepseek/deepseek-flash", Effort: "high", Reasoning: "作者给了两个设定：城市总在下雨，主角修路灯却怕黑。职业和恐惧正好相反，这是张力所在。",
		Text: "修路灯和怕黑放在一起很有**张力**：\n\n- 她每天维护别人的光；\n- 却不敢走进没有灯的巷子，哪怕那是回家最近的路，她也宁可绕上半个城区，沿着亮着的灯一盏一盏走回去。\n  - 绕路本身就是一种`仪式`\n\n### 可以往哪走\n\n1. 先定她怕黑的来历\n2. 再定雨和光的关系\n\n这两件事会互相牵动。\n\n> 她为什么选择了这份工作？"})
	if err != nil {
		t.Fatal(err)
	}
	src := func(m workspace.Event, q string) []workspace.Source {
		return []workspace.Source{{Session: session, Message: m.ID, Quote: q}}
	}
	if _, err = s.Revise(workspace.Revision{By: workspace.ByBookkeeper, Session: session, From: user.ID, Through: reply.ID, Ops: []workspace.Op{
		{Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: "规则", Text: "故事发生在一个总在下雨的城市。", Sources: src(user, "总在下雨的城市")},
		{Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: "人物", Text: "主角靠维修路灯谋生，但她很怕黑。", Sources: src(user, "主角修路灯，但她其实很怕黑")},
		{Op: workspace.OpAdd, Place: workspace.PlaceOpen, Category: "人物", Text: "她宁可绕路，沿着亮着的灯走回家。", Sources: src(reply, "她也宁可绕上半个城区")},
	}}); err != nil {
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	a := desk.NewWithConfig(filepath.Join(root, "providers.yaml"))
	if err = a.Open(filepath.Join(root, "work")); err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	tt := ui.NewTester(a.View, 1266, 780)
	tt.Frame()
	tt.Frame()
	if a.LastChat.H < 600 || a.LastSetting.H < 600 || a.LastChat.W < 600 || a.LastSetting.W < 390 {
		t.Fatalf("discussion bounds chat=%+v setting=%+v", a.LastChat, a.LastSetting)
	}
	if !tt.HasText("设定") || !tt.HasText("悬而未决 · 1") || !tt.HasText("总讨论") {
		t.Fatal(tt.Texts())
	}
	shot(t, tt, "-m1")
	if err = tt.Click("思考过程"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("这是张力所在") {
		t.Fatal("reasoning did not unfold", tt.Texts())
	}
	shot(t, tt, "-thought")
	if err = tt.Click("设定历史"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("后台整理") {
		t.Fatal("history not shown", tt.Texts())
	}
	shot(t, tt, "-history")
	if err = tt.Click("模型设置"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("这部作品用哪个模型") {
		t.Fatal(tt.Texts())
	}
	shot(t, tt, "-settings")
	w := desk.NewWithConfig(filepath.Join(root, "providers.yaml"))
	wt := ui.NewTester(w.View, 1266, 780)
	wt.Frame()
	wt.Frame()
	if !wt.HasText("从一个念头开始") {
		t.Fatal(wt.Texts())
	}
	shot(t, wt, "-welcome")
}

// shot writes a screenshot beside SHOT with suffix, when SHOT is set.
func shot(t *testing.T, tt *ui.Tester, suffix string) {
	t.Helper()
	path := os.Getenv("SHOT")
	if path == "" {
		return
	}
	ext := filepath.Ext(path)
	f, err := os.Create(strings.TrimSuffix(path, ext) + suffix + ext)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err = png.Encode(f, tt.Image()); err != nil {
		t.Fatal(err)
	}
}

// TestLayout checks the editor fills the window, and writes a screenshot
// when SHOT is set.
func TestLayout(t *testing.T) {
	a := &app{ed: editor.New(editor.NewDoc(sample), editor.Options{})}
	var r ui.Rect
	tt := ui.NewTester(func(c *ui.Context) {
		a.view(c)
		r = a.lastEditor
	}, 1086, 680)
	tt.Frame()
	tt.Frame()
	if r.Y > 1 || r.H < 600 {
		t.Errorf("editor bounds %+v, want the full height", r)
	}
	if path := os.Getenv("SHOT"); path != "" {
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		png.Encode(f, tt.Image())
	}
}
