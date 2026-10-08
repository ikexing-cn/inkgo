package discussion

import (
	"context"
	"strings"
	"testing"

	"writer/workspace"
)

// A turn beside the prose carries the chapter, its outline and the text
// before the caret, listed as sent; the quotes stay with the words.
func TestTalkBesideTheProse(t *testing.T) {
	e, session := setup(t)
	f := &fake{replies: []string{"这里可以慢一点。", "好。"}}
	e.Talk = Model{Completer: f, Name: "deepseek/deepseek-flash"}
	id, err := e.Store.NewChapter("", "第一章", false)
	if err != nil {
		t.Fatal(err)
	}
	before := strings.Repeat("雨", BesideCap+500) + "她推开门。"
	e.Beside = &Writing{Title: "第一章", Outline: "她回到旧城。", Volume: "第一卷", VolumeOutline: "秋天。", Before: before}
	q := workspace.Quote{Chapter: id, Title: "第一章", Text: "她推开门。"}
	turn, err := e.Say(context.Background(), session, workspace.Event{Text: "这句太急了？", Quotes: []workspace.Quote{q}, Chapter: id})
	if err != nil {
		t.Fatal(err)
	}
	if u := turn.User; u.Chapter != id || len(u.Quotes) != 1 || u.Quotes[0] != q || u.Text != "这句太急了？" {
		t.Fatalf("saved %+v", u)
	}
	r := f.calls[0]
	sent := r.Messages[len(r.Messages)-1].Content
	for _, want := range []string{"【所在卷：第一卷】卷纲：\n秋天。", "【作者正在写的章：第一章】", "【章纲】\n她回到旧城。", "【引用正文《第一章》】\n她推开门。", "【作者说】\n"} {
		if !strings.Contains(sent, want) {
			t.Fatalf("missing %q in %s", want, sent)
		}
	}
	if strings.Contains(sent, id) || strings.Count(sent, "雨") != BesideCap-len([]rune("她推开门。")) {
		t.Fatalf("sent an ID or not the last %d characters", BesideCap)
	}
	if !strings.Contains(r.System, "不要替作者写成段的正文") {
		t.Fatal("the rules do not know the author is writing")
	}
	var shown []string
	for _, it := range turn.Reply.Context {
		shown = append(shown, it.Text)
	}
	if got := strings.Join(shown[:4], "|"); got != "卷纲：第一卷|所在章：第一章|章纲：第一章|本章光标前 2000 字" {
		t.Fatalf("listed %q", got)
	}

	// The next turn, from the talk page, sees the quote in the history but
	// neither the chapter nor the rules for it.
	e.Beside = nil
	if _, err = e.Send(context.Background(), session, "继续"); err != nil {
		t.Fatal(err)
	}
	r = f.calls[1]
	if !strings.Contains(r.Messages[0].Content, "【引用正文《第一章》】\n她推开门。\n\n这句太急了？") || strings.Contains(r.Messages[0].Content, "光标前") || strings.Contains(r.System, "不要替作者写成段的正文") {
		t.Fatalf("history %q", r.Messages[0].Content)
	}
}

// Words the bookkeeper quotes from prose the author quoted name its
// chapter, and stay a source in the message after the chapter changes.
func TestBookkeeperQuotesTheProseQuoted(t *testing.T) {
	e, session := setup(t)
	id, err := e.Store.NewChapter("", "第一章", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = e.Store.WriteChapter(id, "她从不在夜里出门。", nil); err != nil {
		t.Fatal(err)
	}
	u, err := e.Store.Append(session, workspace.Event{Role: workspace.RoleUser, Text: "这说明她怕黑。", Chapter: id,
		Quotes: []workspace.Quote{{Chapter: id, Title: "第一章", Text: "她从不在夜里出门。"}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = e.Store.Append(session, workspace.Event{Role: workspace.RoleAssistant, Text: "记下了。", ReplyTo: u.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err = e.Store.WriteChapter(id, "她改了。", nil); err != nil {
		t.Fatal(err)
	}
	b, f := book(e, ops(
		map[string]any{"op": "add", "place": "body", "category": "人物", "text": "她怕黑，从不在夜里出门", "sources": []map[string]string{{"msg": "m1", "quote": "她从不在夜里出门"}, {"msg": "m1", "quote": "这说明她怕黑"}}},
	))
	if did, err := b.Run(context.Background(), session); err != nil || !did {
		t.Fatal(did, err)
	}
	if !strings.Contains(f.calls[0].Messages[0].Content, "【引用正文《第一章》】") {
		t.Fatal("the bookkeeper did not see the quote")
	}
	src := setting(t, e).Entries[0].Sources
	if len(src) != 2 || src[0].Chapter != id || src[0].Message != u.ID || src[1].Chapter != "" {
		t.Fatalf("sources %+v", src)
	}
}
