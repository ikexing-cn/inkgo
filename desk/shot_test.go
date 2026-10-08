package desk

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"writer/workspace"
)

// TestShot writes screenshots of the desk when SHOT is a directory, to look
// at the design: SHOT=dir go test ./desk -run Shot
func TestShot(t *testing.T) {
	dir := os.Getenv("SHOT")
	if dir == "" {
		t.Skip("set SHOT=dir")
	}
	a, tt, i := fixture(t)
	user, _ := a.Store.Append(a.session, workspace.Event{Kind: "message", Role: "user", Text: "我想写一个总在下雨的城市，雨停的那天会有人消失。"})
	reply, _ := a.Store.Append(a.session, workspace.Event{Kind: "message", Role: "assistant", ReplyTo: user.ID, Text: "这个设定很有张力。可以让“雨停”成为城里人最恐惧的天气预报：广播里一说明天转晴，家家户户就开始清点人数。\n\n消失的人也许并没有死，而是被雨水“记住”了，只在下一场雨里出现。这样主角就有了动机：盼雨，又怕雨。"})
	var ids []string
	for _, idea := range []workspace.Idea{
		{Text: "雨停那天会有人消失，城里人因此惧怕晴天", Category: "人物", Source: "human", Origin: workspace.Origin{Session: a.session, Message: user.ID, Span: workspace.Span{Start: 0, End: 10}}},
		{Text: "消失的人会在下一场雨里出现，主角因此盼雨又怕雨", Category: "人物", Source: "ai", Origin: workspace.Origin{Session: a.session, Message: reply.ID, Span: workspace.Span{Start: 0, End: 10}}},
	} {
		saved, err := a.Store.NewIdea(idea)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, saved.ID)
	}
	if _, err := a.Store.Append(a.session, workspace.Event{Kind: "bookkeeping", ReplyTo: reply.ID, IdeaIDs: ids, Text: "[]"}); err != nil {
		t.Fatal(err)
	}
	kept, _ := a.Store.Idea(i.ID)
	kept.Status = workspace.Kept
	a.Store.SaveIdea(kept)
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
