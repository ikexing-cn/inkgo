package desk

import (
	"context"
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"

	"writer/workspace"
)

// Where the talk was compacted, a line says how many turns above it the
// digest stands for, and opens it to read; it is no message of the talk.
// A later digest takes its place.
func TestTheTalkShowsWhereItWasCompacted(t *testing.T) {
	a, tt, _ := fixture(t)
	exchange := func(said, reply string, sent *workspace.Sent) workspace.Event {
		t.Helper()
		u, err := a.Store.Append(a.session, workspace.Event{Role: workspace.RoleUser, Text: said})
		if err != nil {
			t.Fatal(err)
		}
		r, err := a.Store.Append(a.session, workspace.Event{Role: workspace.RoleAssistant, ReplyTo: u.ID, Text: reply, Sent: sent})
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	digest := func(through workspace.Event, text string) {
		t.Helper()
		if _, err := a.Store.Append(a.session, workspace.Event{Role: workspace.RoleDigest, Through: through.ID, Text: text, Model: "deepseek/deepseek-flash"}); err != nil {
			t.Fatal(err)
		}
	}
	r, err := a.Store.Revise(workspace.Revision{By: workspace.ByAuthor, Ops: []workspace.Op{{Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: "人物", Names: []string{"林晚"}, Text: "修路灯的人。"}}})
	if err != nil {
		t.Fatal(err)
	}
	exchange("停电这一章想写得紧一点", "紧，靠的是少交代。", nil)
	second := exchange("回忆那段会不会拖", "和眼前的黑写在同一段里，就不拖。", nil)
	third := exchange("这一章的节奏怎么样", "前半段偏慢。", &workspace.Sent{Parts: []workspace.Part{
		{Name: workspace.PartRules, Tokens: 1100}, {Name: workspace.PartDigest, Tokens: 820}, {Name: workspace.PartInput, Tokens: 40}}})
	digest(second, "## 在聊什么\n停电那一章的节奏。\n\n## 作者的态度和偏好\n回忆不写成旁白。[2] 怕黑，写进 [1] 里；[9] 先不管。\n## 还在商量的\n无\n## 下一步\n- 作者先改前半段。\n- 再看 [2] 那句。")
	a.SetError(a.refresh())
	tt.Frame()
	if !tt.HasText("以上 2 轮已整理成摘要") || tt.HasText("作者的态度和偏好") || !tt.HasText("6 条发言") {
		t.Fatal("the line", tt.Texts())
	}
	at := slices.IndexFunc(a.events, func(e workspace.Event) bool { return e.ID == second.ID })
	if k := slices.Index(a.rows, at); k < 0 || a.events[a.rows[k+1]].Role != workspace.RoleDigest {
		t.Fatal("the line is not under the last turn it covers", a.rows)
	}
	if err := tt.Click("以上 2 轮已整理成摘要"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	for _, s := range []string{"在聊什么", "停电那一章的节奏。", "作者的态度和偏好", "回忆不写成旁白。", "下一步", "作者先改前半段。", "摘要由 AI 写成，有误可在对话中指出。"} {
		if !tt.HasText(s) {
			t.Fatal(s, tt.Texts())
		}
	}
	if tt.HasText("## 在聊什么") {
		t.Fatal("a heading shows its marks")
	}
	if !tt.HasText("回忆不写成旁白。「林晚」 怕黑，写进 「主角怕黑」 里；[9] 先不管。") {
		t.Fatal("the entries by their names", tt.Texts())
	}
	if !tt.HasText("•") || !tt.HasText("作者先改前半段。") || !tt.HasText("再看 「林晚」 那句。") || tt.HasText("- 作者") {
		t.Fatal("a list in the digest is not laid out as in a reply", tt.Texts())
	}
	if err := tt.Click("林晚"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.doc.shown != r.Ops[0].Entry || a.doc.hidden {
		t.Fatalf("the entry named is not open: %q", a.doc.shown)
	}
	if err := tt.Click("以上 2 轮已整理成摘要"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if tt.HasText("作者的态度和偏好") {
		t.Fatal("the digest does not close")
	}
	if err := tt.Click("上下文-" + third.ID); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("规则 1.1K · 讨论摘要 820 · 本次发言 40") {
		t.Fatal("what was sent", tt.Texts())
	}

	digest(third, "## 在聊什么\n前半段的节奏。")
	a.SetError(a.refresh())
	tt.Frame()
	if !tt.HasText("以上 3 轮已整理成摘要") || tt.HasText("以上 2 轮已整理成摘要") {
		t.Fatal("the later digest", tt.Texts())
	}
}

// While the talk is compacted before a reply, the reply says so; the line
// shows once the digest is written, and the author's words, read back
// with it, show once.
func TestCompactingShowsItself(t *testing.T) {
	a, _, _ := fixture(t)
	var c *ui.Context
	tt := ui.NewTester(func(ctx *ui.Context) { c = ctx; a.View(ctx) }, 1266, 780)
	tt.Frame()
	session := a.session
	u, err := a.Store.Append(session, workspace.Event{Role: workspace.RoleUser, Text: "停电这一章想写得紧一点"})
	if err != nil {
		t.Fatal(err)
	}
	r, err := a.Store.Append(session, workspace.Event{Role: workspace.RoleAssistant, ReplyTo: u.ID, Text: "紧，靠的是少交代。"})
	if err != nil {
		t.Fatal(err)
	}
	a.SetError(a.refresh())
	tt.Frame()
	once := func() bool {
		n := 0
		for _, s := range tt.Texts() {
			if s == "回忆那段会不会拖" {
				n++
			}
		}
		return n == 1
	}
	sv := c.Services()
	asked, block := make(chan struct{}), make(chan struct{})
	a.pending = "回忆那段会不会拖"
	a.run(c, "AI 正在思考…", func(context.Context) (func(), error) {
		if _, err := a.Store.Append(session, workspace.Event{Role: workspace.RoleUser, Text: "回忆那段会不会拖"}); err != nil {
			return nil, err
		}
		a.stepped(sv, workspace.Event{Role: workspace.RoleDigest})
		<-asked
		d, err := a.Store.Append(session, workspace.Event{Role: workspace.RoleDigest, Through: r.ID, Text: "## 在聊什么\n停电那一章。"})
		if err != nil {
			return nil, err
		}
		a.stepped(sv, d)
		<-block
		return func() { a.pending = "" }, nil
	})
	a.talking, a.talkSession = true, session
	until(t, tt, "compacting", func() bool { return tt.HasText("正在压缩上下文…") })
	if tt.HasText("正在思考…") || tt.HasText("以上 1 轮已整理成摘要") || !once() {
		t.Fatal("compacting", tt.Texts())
	}
	close(asked)
	until(t, tt, "the digest", func() bool { return tt.HasText("以上 1 轮已整理成摘要") })
	if !tt.HasText("正在思考…") || tt.HasText("正在压缩上下文…") || !once() {
		t.Fatal("compacted", tt.Texts())
	}
	close(block)
	settle(t, a, tt)
}

func TestADigestSplitsAtItsHeadings(t *testing.T) {
	got := sections("开头的话\n## 在聊什么\n第一行\n\n第二行\n##不是标题\n## 下一步\n")
	want := [][2]string{{"", "开头的话"}, {"在聊什么", "第一行\n\n第二行\n##不是标题"}, {"下一步", ""}}
	if !slices.Equal(got, want) {
		t.Fatalf("%q", got)
	}
}
