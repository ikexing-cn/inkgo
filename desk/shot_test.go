package desk

import (
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"writer/workspace"
)

// TestShot writes screenshots of the desk when SHOT is a directory, to look
// at the design: SHOT=dir go test ./desk -run Shot
func TestShot(t *testing.T) {
	dir := os.Getenv("SHOT")
	if dir == "" {
		t.Skip("set SHOT=dir")
	}
	a, tt, _ := fixture(t)
	user, _ := a.Store.Append(a.session, workspace.Event{Role: "user", Text: "我想写一个总在下雨的城市，雨停的那天会有人消失。"})
	reply, _ := a.Store.Append(a.session, workspace.Event{Role: "assistant", ReplyTo: user.ID, Text: "这个设定很有张力。可以让“雨停”成为城里人最恐惧的天气预报：广播里一说明天转晴，家家户户就开始清点人数。\n\n消失的人也许并没有死，而是被雨水“记住”了，只在下一场雨里出现。这样主角就有了动机：盼雨，又怕雨。"})
	if _, err := a.Store.Revise(workspace.Revision{By: workspace.ByBookkeeper, Session: a.session, From: user.ID, Through: reply.ID, Ops: []workspace.Op{
		{Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: "规则", Text: "雨停那天会有人消失，城里人因此惧怕晴天", Sources: []workspace.Source{{Session: a.session, Message: user.ID, Quote: "雨停的那天会有人消失"}}},
		{Op: workspace.OpAdd, Place: workspace.PlaceOpen, Category: "人物", Text: "消失的人会在下一场雨里出现，主角因此盼雨又怕雨", Sources: []workspace.Source{{Session: a.session, Message: reply.ID, Quote: "只在下一场雨里出现"}}},
	}}); err != nil {
		t.Fatal(err)
	}
	if err := a.refresh(); err != nil {
		t.Fatal(err)
	}
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
	shot("chat-light.png")
	tt.SetDark(true)
	shot("chat-dark.png")
	tt.SetDark(false)
	a.page = 1
	shot("settings.png")
}

// TestShotAI writes screenshots of the AI on the writing page when SHOT is
// a directory: SHOT=dir go test ./desk -run ShotAI
func TestShotAI(t *testing.T) {
	dir := os.Getenv("SHOT")
	if dir == "" {
		t.Skip("set SHOT=dir")
	}
	a, tt, _ := writingFixture(t, "雨从傍晚开始下，一直没有停。\n林晚背着工具箱，沿着亮着的灯往旧城走。她修了七年路灯，却从来不敢在天黑以后走进没有灯的巷子。\n井边那盏灯坏了三天。")
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
	f := &stream{pieces: []string{"她蹲下去拧开灯座，", "手电的光在雨里抖得厉害。"}}
	penWith(a, f)
	ed := a.write.ed
	ed.MoveTo(ed.Doc.Len(), false)
	ed.Focus()
	tt.Key(ui.Cmd, ui.KeyJ)
	until(t, tt, "the sentence", func() bool { return ed.Ghost() != "" })
	shot("ai-continue.png")
	ed.DismissGhost()

	ed.Select(15, 34)
	tt.Frame()
	a.openRewrite()
	a.write.instruction = "节奏放慢，多写她的犹豫"
	shot("ai-rewrite-ask.png")
	f.pieces = []string{"林晚背着工具箱，在亮着的灯下站了很久，", "才一盏一盏地往旧城走。"}
	if err := tt.Click("改写"); err != nil {
		t.Fatal(err)
	}
	until(t, tt, "the rewrite", func() bool { return a.write.proposed != nil })
	shot("ai-rewrite.png")
	a.acceptRewrite()

	a.write.outline = "林晚夜里去旧城修灯，发现井边的灯是被人拧坏的。"
	a.openDraft()
	a.write.words = "1500"
	shot("ai-draft-ask.png")
	f.pieces, f.hold = []string{"她把手电咬在嘴里，", "拧下灯罩。"}, make(chan struct{})
	if err := tt.Click("开始起草"); err != nil {
		t.Fatal(err)
	}
	until(t, tt, "the draft", func() bool { return strings.HasSuffix(ed.Doc.String(), "咬在嘴里，") })
	shot("ai-drafting.png")
	close(f.hold)
	until(t, tt, "the draft to end", func() bool { return a.write.run == nil })
	a.openSnapshots()
	shot("ai-snapshots.png")
}
