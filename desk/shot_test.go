package desk

import (
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
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
	if _, err := a.Store.Revise(workspace.Revision{By: workspace.ByTalk, Session: a.session, Turn: user.ID, Ops: []workspace.Op{
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

	// The talk proposes in the prose; the rewrite and the draft are its
	// first words.
	g := &script{}
	penWith(a, g)
	take := func() {
		time.Sleep(settleFor)
		a.takeProps(a.write.props)
	}
	ed.Select(15, 34)
	tt.Frame()
	a.openRewrite()
	a.write.instruction = "节奏放慢，多写她的犹豫"
	shot("ai-rewrite-ask.png")
	hold := g.holding()
	g.then(proposal("第一章", "replace", ed.Doc.Slice(15, 34), "林晚背着工具箱，在亮着的灯下站了很久，才一盏一盏地往旧城走。"), []string{"放慢了：她先在灯下站了很久，再一盏一盏往旧城走，犹豫落在动作上。"})
	if err := tt.Click("改写"); err != nil {
		t.Fatal(err)
	}
	until(t, tt, "the rewrite", func() bool { return len(ed.Proposals()) == 1 })
	shot("ai-rewriting.png")
	close(hold)
	replied(t, a, tt)
	shot("ai-rewrite.png")
	take()

	a.write.outline = "林晚夜里去旧城修灯，发现井边的灯是被人拧坏的。"
	a.openDraft()
	a.write.words = "1500"
	shot("ai-draft-ask.png")
	hold = g.holding()
	g.then(proposal("第一章", "end", "", "她把手电咬在嘴里，拧下灯罩。灯座上的螺丝少了两颗，断口是新的。"), []string{"按章纲接着写了一段：她发现灯是被人拧坏的。"})
	if err := tt.Click("开始起草"); err != nil {
		t.Fatal(err)
	}
	until(t, tt, "the draft", func() bool { return len(ed.Proposals()) == 1 })
	shot("ai-drafting.png")
	close(hold)
	replied(t, a, tt)
	shot("ai-drafted.png")
	take()

	// Asked again in the talk: leaving the work asks about it; the words
	// it replaces changed, it is void.
	g.then(proposal("第一章", "replace", "井边那盏灯坏了三天。", "井边那盏灯已经坏了三天，没人来修。"), []string{"补了一句没人来修。"})
	a.input = "最后一句补一下没人修"
	tt.Frame()
	if err := tt.Click("发送"); err != nil {
		t.Fatal(err)
	}
	replied(t, a, tt)
	a.Leave(func() {})
	shot("ai-leave.png")
	a.write.leaving = nil
	text := ed.Doc.String()
	at := utf8.RuneCountInString(text[:strings.Index(text, "三天")])
	ed.Select(at, at+2)
	ed.Insert("五天")
	shot("ai-void.png")

	// Proposed in another chapter, it opens in a tab beside.
	two, err := a.Store.NewChapter("", "第二章", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.Store.WriteChapter(two, "档案室白天也要开灯。", nil); err != nil {
		t.Fatal(err)
	}
	a.warn(a.loadTree())
	g.then(proposal("第二章", "replace", "档案室白天也要开灯。", "档案室白天也得开着灯。"), []string{"第二章那句顺了一下。"})
	a.input = "第二章开头那句顺一下"
	tt.Frame()
	if err := tt.Click("发送"); err != nil {
		t.Fatal(err)
	}
	replied(t, a, tt)
	shot("ai-beside.png")
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
