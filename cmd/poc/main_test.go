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
	user, err := s.Append(session, workspace.Event{Kind: "message", Role: "user", Text: "我想写一个总在下雨的城市。主角修路灯，但她其实很怕黑。"})
	if err != nil {
		t.Fatal(err)
	}
	reply, err := s.Append(session, workspace.Event{Kind: "message", Role: "assistant", ReplyTo: user.ID,
		Model: "deepseek/deepseek-flash", Effort: "high", Reasoning: "作者给了两个设定：城市总在下雨，主角修路灯却怕黑。职业和恐惧正好相反，这是张力所在。",
		Text: "修路灯和怕黑放在一起很有**张力**：\n\n- 她每天维护别人的光；\n- 却不敢走进没有灯的巷子。\n\n> 她为什么选择了这份工作？"})
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, text := range []string{"故事发生在一个总在下雨的城市。", "主角靠维修路灯谋生，但她很怕黑。", "她维护别人的光，却害怕无灯的巷子。"} {
		source, msg := "human", user.ID
		if strings.HasPrefix(text, "她") {
			source, msg = "ai", reply.ID
		}
		i, err := s.NewIdea(workspace.Idea{Text: text, Source: source, Category: "人物", Origin: workspace.Origin{Session: session, Message: msg, Span: workspace.Span{Start: 0, End: 5}}})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, i.ID)
	}
	if _, err = s.Append(session, workspace.Event{Kind: "bookkeeping", ReplyTo: reply.ID, IdeaIDs: ids, Text: "[]"}); err != nil {
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
	if a.LastChat.H < 600 || a.LastPool.H < 600 || a.LastChat.W < 600 || a.LastPool.W < 390 {
		t.Fatalf("discussion bounds chat=%+v pool=%+v", a.LastChat, a.LastPool)
	}
	if !tt.HasText("思路池") || !tt.HasText("总讨论") {
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
