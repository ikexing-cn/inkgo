package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"writer/desk"
	"writer/ext"
	aimod "writer/ext/ai"
	"writer/internal/testshot"
	"writer/workspace"
)

func layoutBook(t *testing.T, root string) *workspace.Store {
	t.Helper()
	s, err := workspace.Create(filepath.Join(root, "work"), "雨城来信", "网文", "zh-Hans")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func newLayoutApp(root string) *desk.App {
	a := desk.New(filepath.Join(root, "providers.yaml"), append(testModules(), aimod.New(aimod.Options{ConfigFile: filepath.Join(root, "providers.yaml")}))...)
	return a
}

func openLayoutApp(t *testing.T, root string) *desk.App {
	t.Helper()
	a := newLayoutApp(root)
	if err := a.Open(filepath.Join(root, "work")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.Close() })
	return a
}

func TestLayoutDiscussion(t *testing.T) {
	root := t.TempDir()
	s := layoutBook(t, root)
	var err error
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
	if _, err = s.Revise(workspace.Revision{By: workspace.ByTalk, Session: session, Turn: user.ID, Ops: []workspace.Op{
		{Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: "规则", Text: "故事发生在一个总在下雨的城市。", Sources: src(user, "总在下雨的城市")},
		{Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: "人物", Text: "主角靠维修路灯谋生，但她很怕黑。", Sources: src(user, "主角修路灯，但她其实很怕黑")},
		{Op: workspace.OpAdd, Place: workspace.PlaceOpen, Category: "人物", Text: "她宁可绕路，沿着亮着的灯走回家。", Sources: src(reply, "她也宁可绕上半个城区")},
	}}); err != nil {
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	// Left on the discussion page.
	if err = os.WriteFile(filepath.Join(root, "work", ".writer", "desk.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	a := openLayoutApp(t, root)
	tt := ui.NewTester(a.View, 1266, 780)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	tt.Frame()
	if a.LastChat.H < 600 || a.LastSetting.H < 600 || a.LastChat.W < 600 || a.LastSetting.W < 390 {
		t.Fatalf("discussion bounds chat=%+v setting=%+v", a.LastChat, a.LastSetting)
	}
	if !tt.HasText("设定") || !tt.HasText("待确认") || !tt.HasText("总讨论") {
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
	if err = tt.Click("更多"); err != nil {
		t.Fatal(err)
	}
	if err = tt.Click("菜单-修改记录"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("记在「") {
		t.Fatal("history not shown", tt.Texts())
	}
	shot(t, tt, "-history")
	if err = tt.Click("菜单栏-文件"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err = tt.Click("菜单-设置"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err = tt.Click("设置-AI"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("选用模型") {
		t.Fatal(tt.Texts())
	}
	shot(t, tt, "-settings")
	tt.Scroll(633, 400, 0, 400) // the services, under what the work uses and spent
	tt.Frame()
	if err = tt.Click("提供商-deepseek"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Scroll(633, 400, 0, 1200) // down to the service's models
	tt.Frame()
	if !tt.HasText("压缩阈值 %") {
		t.Fatal(tt.Texts())
	}
	shot(t, tt, "-service")
	w := newLayoutApp(root)
	wt := ui.NewTester(w.View, 1266, 780)
	wt.Frame()
	wt.Frame()
	if !wt.HasText("我的作品") || !wt.HasText("雨城来信") {
		t.Fatal(wt.Texts())
	}
	shot(t, wt, "-shelf")
	e := newLayoutApp(t.TempDir())
	et := ui.NewTester(e.View, 1266, 780)
	et.Frame()
	et.Frame()
	if !et.HasText("从一个想法开始") {
		t.Fatal(et.Texts())
	}
	shot(t, et, "-welcome")
}

func TestLayoutWriting(t *testing.T) {
	root := t.TempDir()
	s := layoutBook(t, root)
	var err error
	vol, err := s.NewChapter("", "第一卷：雨季", true)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.WriteOutline(vol, "她接下修灯的活，第一次走进没有灯的旧城。"); err != nil {
		t.Fatal(err)
	}
	one, err := s.NewChapter(vol, "第一章：路灯", false)
	if err != nil {
		t.Fatal(err)
	}
	text := "雨从傍晚开始下，一直没有停。\n林晚背着工具箱，沿着亮着的灯往旧城走。她修了七年路灯，却从来不敢在天黑以后走进没有灯的巷子。\n井边那盏灯坏了三天。她蹲下去拧开灯座，手电的光在雨里抖得厉害。"
	if _, err = s.WriteChapter(one, text, []workspace.ChapterMark{{Kind: workspace.MarkForeshadow, Start: 78, End: 80}}); err != nil {
		t.Fatal(err)
	}
	if err = s.WriteOutline(one, "林晚夜里去旧城修灯，发现井边的灯是被人拧坏的。"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.NewChapter(vol, "第二章：井", false); err != nil {
		t.Fatal(err)
	}
	if _, err = s.NewChapter("", "番外：晴天", false); err != nil {
		t.Fatal(err)
	}
	if _, err = s.TakeSnapshot(one, workspace.SnapInterval, "雨从傍晚开始下。\n林晚背着工具箱往旧城走。", nil); err != nil {
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	a := openLayoutApp(t, root)
	tt := ui.NewTester(a.View, 1266, 780)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	if err = tt.Click("正文"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if a.LastWrite.W < 500 || a.LastWrite.H < 600 {
		t.Fatalf("writing bounds %+v", a.LastWrite)
	}
	if !tt.HasText("第二章：井") || !tt.HasText("本章 91 字 · 全书 91 字") {
		t.Fatal(tt.Texts())
	}
	shot(t, tt, "-write")
	// Zoomed layouts keep enough window DIPs for the minimum content size.
	for _, zoom := range []struct {
		scale         float32
		width, height int
		suffix        string
	}{{1.25, 1400, 900, "-write-125"}, {1.5, 1560, 1050, "-write-150"}} {
		tt.SetSize(zoom.width, zoom.height)
		tt.Key(ui.Cmd, ui.Key0)
		for a.Zoom() < zoom.scale {
			tt.Key(ui.Cmd, ui.KeyEqual)
		}
		tt.Frame()
		shot(t, tt, zoom.suffix)
		if zoom.scale == 1.25 {
			if err = tt.Click("菜单栏-视图"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if !tt.HasText("缩放 125%") {
				t.Fatal(tt.Texts())
			}
			shot(t, tt, "-view-125")
			tt.Key(0, ui.KeyEscape)
		}
	}
	tt.Key(ui.Cmd, ui.Key0)
	tt.SetSize(1266, 780)
	tt.Frame()
	if err = clickMenu(tt, "文件", "菜单-历史版本"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	shot(t, tt, "-snapshots")
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if err = clickMenu(tt, "文件", "菜单-导出"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	shot(t, tt, "-export")
	for _, s := range []string{"纯文本", "指定范围"} {
		if err = tt.Click(s); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
	}
	if !tt.HasText("段首加两个全角空格") || !tt.HasText("番外：晴天") {
		t.Fatal("the export as text, of chapters from one to another", tt.Texts())
	}
	shot(t, tt, "-export-span")
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	p := filepath.Join(root, "work", "chapters", "第一卷：雨季", "第一章：路灯.txt")
	if err = os.WriteFile(p, []byte("别处改过的一句。"), 0644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(2100 * time.Millisecond) // the page looks at the disk every two seconds
	tt.Frame()
	if !tt.HasText("保留当前版本") {
		t.Fatal("the outside change was not noticed", tt.Texts())
	}
	shot(t, tt, "-outside")
	if err = tt.Click("使用磁盘版本"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	// A folder's page opens from its menu; a click on it only folds it.
	if err = tt.RightClick("章节-第一卷：雨季"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err = tt.Click("菜单-编辑卷纲"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	shot(t, tt, "-folder")
	if err = tt.Click("章节-第一章：路灯"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err = tt.Click("查找替换"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	shot(t, tt, "-find")
}

// clickMenu clicks the row named in a menu of the title bar.
func clickMenu(tt *ui.Tester, menu, name string) error {
	if err := tt.Click("菜单栏-" + menu); err != nil {
		return err
	}
	tt.Frame()
	return tt.Click(name)
}

// shot writes a named screenshot in SHOT, when set.
func shot(t *testing.T, tt *ui.Tester, suffix string) {
	t.Helper()
	if !testshot.Enabled() {
		return
	}
	// What unfolds or fades runs on the window's clock, which stands still
	// in a test: run it past them.
	tt.Advance(time.Second)
	tt.Frame()
	name := "layout" + suffix + ".png"
	testshot.Save(t, name, tt.Image())
}

func newModuleApp(config string, mods ...ext.Module) *desk.App {
	return desk.New(config, append(append([]ext.Module(nil), mods...), aimod.New(aimod.Options{ConfigFile: config}))...)
}
