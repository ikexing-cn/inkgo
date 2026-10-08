package desk

import (
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"writer/shelf"

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

// TestShotBeside writes screenshots of the talk beside the prose when SHOT
// is a directory: SHOT=dir go test ./desk -run ShotBeside
func TestShotBeside(t *testing.T) {
	dir := os.Getenv("SHOT")
	if dir == "" {
		t.Skip("set SHOT=dir")
	}
	a, tt, id := writingFixture(t, "雨从傍晚开始下，一直没有停。\n林晚背着工具箱，沿着亮着的灯往旧城走。她修了七年路灯，却从来不敢在天黑以后走进没有灯的巷子。\n井边那盏灯坏了三天。")
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
	u, _ := a.Store.Append(a.session, workspace.Event{Role: workspace.RoleUser, Text: "这句是不是交代得太早了？", Chapter: id,
		Quotes: []workspace.Quote{{Chapter: id, Title: "第一章", Text: "她修了七年路灯，却从来不敢在天黑以后走进没有灯的巷子。"}}})
	_, _ = a.Store.Append(a.session, workspace.Event{Role: workspace.RoleAssistant, ReplyTo: u.ID, Model: "deepseek/deepseek-flash",
		Text: "有一点。“怕黑”是她的底牌，第一章就直接说出来，后面井边那场戏就少了悬念。\n\n可以只留动作：她在巷口停住，把手电换到另一只手。读者会自己猜。"})
	a.setError(a.refresh())
	a.doc.talk = true
	a.write.ed.Select(15, 34)
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyL)
	a.input = "换成动作的话，放在哪里合适？"
	shot("beside-talk.png")
	a.doc.width = 560
	shot("beside-wide.png")
	a.page = 0
	shot("beside-on-talk-page.png")
}

// TestShotShelf writes screenshots of the shelf when SHOT is a directory:
// SHOT=dir go test ./desk -run ShotShelf
func TestShotShelf(t *testing.T) {
	dir := os.Getenv("SHOT")
	if dir == "" {
		t.Skip("set SHOT=dir")
	}
	a, tt, _ := fixture(t)
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
	root := filepath.Dir(a.Store.Root())
	for i, w := range []struct{ name, genre, text string }{
		{"雨城来信", "网文", strings.Repeat("雨从傍晚开始下。", 15000)},
		{"旧城灯匠", "出版小说", strings.Repeat("她修了七年路灯。", 400)},
		{"井边", "剧本", ""},
	} {
		s, err := workspace.Create(filepath.Join(root, w.name), w.name, w.genre)
		if err != nil {
			t.Fatal(err)
		}
		if w.text != "" {
			id, _ := s.NewChapter("", "第一章", false)
			_, _ = s.WriteChapter(id, w.text, nil)
		}
		b := book(s)
		b.Opened = time.Now().Add(-time.Duration(i*30+2) * time.Hour)
		a.shelf.Put(b)
		s.Close()
	}
	a.shelf.Put(shelf.Book{Path: filepath.Join(root, "早年的稿子"), Name: "早年的稿子", Genre: "非虚构", Words: 52000, Opened: time.Now().AddDate(-1, 0, 0)})
	a.PickFolder = func(string) (string, error) { return "", nil }
	shot("shelf-sidebar.png")
	a.closeWork()
	shot("shelf-list.png")
	a.shelf.Cards = true
	shot("shelf-cards.png")
	a.openCreate()
	a.name = "灯下"
	shot("shelf-create.png")
	a.books.creating = false
	a.shelf.Cards = false
	a.shelf.Books = nil
	shot("shelf-empty.png")
}

// TestShotSummary writes screenshots of the chapter summaries when SHOT is a
// directory: SHOT=dir go test ./desk -run ShotSummary
func TestShotSummary(t *testing.T) {
	dir := os.Getenv("SHOT")
	if dir == "" {
		t.Skip("set SHOT=dir")
	}
	a, tt, one := writingFixture(t, longProse)
	shot := func(name string) {
		for i := 0; i < 20; i++ {
			tt.Frame()
		}
		f, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
	penWith(a, &stream{pieces: []string{"林晚冒雨进城，", "在井边发现那盏灯是被人拧坏的。"}})
	two, err := a.Store.NewChapter("", "第二章", false)
	if err != nil {
		t.Fatal(err)
	}
	three, err := a.Store.NewChapter("", "第三章", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.Store.WriteChapter(two, longProse, nil); err != nil {
		t.Fatal(err)
	}
	a.setError(a.loadTree())
	a.openNode(one)
	a.write.summaryOpen = true
	a.write.ed.MoveTo(a.write.ed.Doc.Len(), false)
	a.write.ed.Insert("她推开门。")
	a.openNode(three)
	until(t, tt, "the summary", func() bool { return a.sums.running == "" && len(a.sums.queue) == 0 })
	a.openNode(one)
	shot("summary-box.png")
	a.write.ed.Insert("又一句。")
	a.flush()
	shot("summary-stale.png")
	a.openNode(three)
	a.openDraft()
	shot("summary-draft.png")
	tt.SetDark(true)
	shot("summary-draft-dark.png")
	a.write.draftOpen = false
	a.openNode(one)
	shot("summary-box-dark.png")
}
