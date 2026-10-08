package desk

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/discussion"
	"writer/model"
	"writer/workspace"
)

func fixture(t *testing.T) (*App, *ui.Tester, workspace.Idea) {
	t.Helper()
	root := t.TempDir()
	s, err := workspace.Create(filepath.Join(root, "work"), "测试作品", "网文")
	if err != nil {
		t.Fatal(err)
	}
	i, err := s.NewIdea(workspace.Idea{Text: "主角怕黑", Category: "人物"})
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	a := NewWithConfig(filepath.Join(root, "providers.yaml"))
	if err = a.Open(filepath.Join(root, "work")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.Close() })
	tt := ui.NewTester(a.View, 1266, 780)
	tt.Frame()
	tt.Frame()
	return a, tt, i
}
func settle(t *testing.T, a *App, tt *ui.Tester) {
	t.Helper()
	if a.busy == "" {
		return
	}
	select {
	case r := <-a.results:
		a.results <- r
		tt.Frame()
	case <-time.After(3 * time.Second):
		t.Fatal("worker did not finish")
	}
}

func TestAuthorShortcutsAndInputIsolation(t *testing.T) {
	a, tt, i := fixture(t)
	if err := tt.Click("讨论输入"); err != nil {
		t.Fatal(err)
	}
	tt.TypeKey(0, ui.Key1, "1")
	tt.TypeKey(0, ui.Key2, "2")
	tt.TypeKey(0, ui.Key3, "3")
	if a.input != "123" || a.busy != "" || a.rejectID != "" {
		t.Fatalf("input shortcuts leaked: %q %q", a.input, a.busy)
	}
	if err := tt.Click("思路-" + i.ID); err != nil {
		t.Fatal(err)
	}
	tt.Key(0, ui.Key2)
	tt.Frame()
	if a.rejectID != i.ID || a.busy != "" {
		t.Fatal("reject must allow an optional reason")
	}
	tt.Type("与故事基调不符")
	tt.Key(0, ui.KeyEnter)
	settle(t, a, tt)
	saved, err := a.Store.Idea(i.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Status != workspace.Rejected || saved.History[0].Reason != "与故事基调不符" {
		t.Fatalf("rejection %+v", saved)
	}
	tt.Frame()
	if len(a.visibleIdeas()) != 0 {
		t.Fatal("a decided card must leave the pending queue")
	}
	a.filter = len(filters) - 1
	tt.Frame()
	if err = tt.Click("捞回待定"); err != nil {
		t.Fatal(err)
	}
	settle(t, a, tt)
	saved, _ = a.Store.Idea(i.ID)
	if saved.Status != workspace.Deferred {
		t.Fatal(saved.Status)
	}
	if err = tt.Click("思路-" + i.ID); err != nil {
		t.Fatal(err)
	}
	tt.Key(0, ui.Key1)
	settle(t, a, tt)
	saved, _ = a.Store.Idea(i.ID)
	if saved.Status != workspace.Kept {
		t.Fatal("offline human decision did not persist", a.Error)
	}
}

func TestUndoLastDecision(t *testing.T) {
	a, tt, i := fixture(t)
	// The pool's tab also reads 留下; the card's button is named by its card.
	if err := tt.Click("留下-" + i.ID); err != nil {
		t.Fatal(err)
	}
	settle(t, a, tt)
	tt.Frame()
	if saved, _ := a.Store.Idea(i.ID); saved.Status != workspace.Kept {
		t.Fatal(saved.Status, a.Error)
	}
	if err := tt.Click("撤销"); err != nil {
		t.Fatal(err, tt.Texts())
	}
	settle(t, a, tt)
	if saved, _ := a.Store.Idea(i.ID); saved.Status != workspace.Unprocessed || a.last != nil {
		t.Fatal("undo did not restore", saved.Status)
	}
}

func TestSessionTitlesAndTurnFilter(t *testing.T) {
	a, tt, i := fixture(t)
	user, _ := a.Store.Append(a.session, workspace.Event{Kind: "message", Role: "user", Text: "雨城的故事\n第二行"})
	reply, _ := a.Store.Append(a.session, workspace.Event{Kind: "message", Role: "assistant", ReplyTo: user.ID, Text: "好"})
	turnIdea, err := a.Store.NewIdea(workspace.Idea{Text: "本轮思路", Category: "人物", Source: "human", Origin: workspace.Origin{Session: a.session, Message: user.ID, Span: workspace.Span{Start: 0, End: 2}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.Store.Append(a.session, workspace.Event{Kind: "bookkeeping", ReplyTo: reply.ID, IdeaIDs: []string{turnIdea.ID}, Text: "[]"}); err != nil {
		t.Fatal(err)
	}
	if err := a.refresh(); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if title := a.titles[a.session]; !strings.HasSuffix(title, "雨城的故事") {
		t.Fatalf("title %q", title)
	}
	if err := tt.Click("本轮思路-" + reply.ID); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if v := a.visibleIdeas(); len(v) != 1 || v[0].Text != "本轮思路" || tt.HasText(i.Text) {
		t.Fatalf("turn filter %+v", v)
	}
}

func TestCancellationKeepsUIResponsive(t *testing.T) {
	a, tt, _ := fixture(t)
	var viewContext *ui.Context
	tt = ui.NewTester(func(c *ui.Context) { viewContext = c; a.View(c) }, 1266, 780)
	tt.Frame()
	a.run(viewContext, "测试中的请求", func(ctx context.Context) (func(), error) { <-ctx.Done(); return nil, ctx.Err() })
	tt.Frame()
	if err := tt.Click("取消请求"); err != nil {
		t.Fatal(err)
	}
	settle(t, a, tt)
	if a.busy != "" || a.cancel != nil || a.Error != context.Canceled.Error() {
		t.Fatalf("cancel result %q %q", a.busy, a.Error)
	}
}

func TestFailureRefreshesPersistedWork(t *testing.T) {
	a, tt, _ := fixture(t)
	var c *ui.Context
	tt = ui.NewTester(func(ctx *ui.Context) { c = ctx; a.View(ctx) }, 1266, 780)
	tt.Frame()
	a.run(c, "记账", func(context.Context) (func(), error) {
		_, err := a.Store.NewIdea(workspace.Idea{Text: "失败前已经落盘的内容"})
		if err != nil {
			return nil, err
		}
		return nil, errors.New("网络失败")
	})
	settle(t, a, tt)
	if len(a.ideas) != 2 || a.Error != "网络失败" {
		t.Fatalf("persisted partial result lost: %d %q", len(a.ideas), a.Error)
	}
}

func TestNewWorkDoesNotInheritLastWork(t *testing.T) {
	a, _, _ := fixture(t)
	a.closeWork()
	if a.path != "" || a.name != "" {
		t.Fatalf("welcome form kept the last work: %q %q", a.path, a.name)
	}
	a.path = filepath.Join(t.TempDir(), "雨城来信")
	if err := a.create(); err != nil {
		t.Fatal(err)
	}
	if name := a.Store.Work().Name; name != "雨城来信" {
		t.Fatalf("an unnamed work is named after its folder, got %q", name)
	}
}

// Inputs the store would reject are caught first, in words the author reads.
func TestIncompleteFormsExplainInChinese(t *testing.T) {
	a, tt, i := fixture(t)
	check := func(name string, err error) {
		t.Helper()
		if err == nil {
			t.Fatalf("%s: accepted", name)
		}
		for _, field := range []string{"discussion:", "bookkeeping:", "categories:", "text:", "verified:"} {
			if strings.Contains(err.Error(), field) {
				t.Fatalf("%s: store error leaked: %v", name, err)
			}
		}
	}
	a.talkProvider, a.talkModel = "deepseek", ""
	check("discussion model", a.saveUsage())
	a.talkModel, a.bookProvider, a.bookModel = "deepseek-flash", "deepseek", ""
	check("bookkeeping model", a.saveUsage())
	a.categoryName, a.categoryTarget = "伏笔", ""
	check("category target", a.saveCategory())

	a.selectIdea(i)
	a.draft.text = "  "
	a.saveDetails(nil, i)
	check("empty text", shown(a))
	a.draft.text, a.draft.fact, a.draft.verified, a.draft.factSource = i.Text, true, true, ""
	a.saveDetails(nil, i)
	check("unsourced fact", shown(a))
	tt.Frame()
	if saved, _ := a.Store.Idea(i.ID); saved.Text != i.Text || saved.Verified {
		t.Fatalf("rejected edit was saved: %+v", saved)
	}
}

// shown is the error the banner shows, if any.
func shown(a *App) error {
	if a.Error == "" {
		return nil
	}
	return errors.New(a.Error)
}

func TestMarkdownBlocks(t *testing.T) {
	src := "# 标题\n\n有**粗体**、*斜体*、`代码`和[链接](https://example.com)。\n\n- 一\n- 二\n\n1. 甲\n\n> 引用\n\n```\ncode\n```\n\n| 列 | 值 |\n|---|---|\n| 甲 | 1 |\n\n<script>x</script>\n\n---\n"
	blocks := mdCache{}.get(src)
	var kinds []mdKind
	for _, b := range blocks {
		kinds = append(kinds, b.kind)
	}
	want := []mdKind{mdHeading, mdParagraph, mdList, mdList, mdQuote, mdCode, mdTable, mdParagraph, mdRule}
	if fmt.Sprint(kinds) != fmt.Sprint(want) {
		t.Fatalf("kinds %v, want %v", kinds, want)
	}
	styled := map[string]mdSpan{}
	for _, s := range blocks[1].inline {
		styled[s.text] = s
	}
	if !styled["粗体"].bold || !styled["斜体"].italic || !styled["代码"].code || !styled["链接"].link {
		t.Fatalf("inline styles %+v", blocks[1].inline)
	}
	if !blocks[3].ordered || blocks[3].start != 1 || blocks[5].code != "code" || len(blocks[6].rows) != 2 || !blocks[6].rows[0][0][0].bold {
		t.Fatalf("blocks %+v", blocks)
	}
	if blocks[7].inline[0].text != "<script>x</script>" {
		t.Fatal("raw HTML must show as text")
	}
}

// A turn in flight shows its reasoning, then its reply, as they stream.
func TestLiveTurnStreams(t *testing.T) {
	a, tt, _ := fixture(t)
	var c *ui.Context
	tt = ui.NewTester(func(ctx *ui.Context) { c = ctx; a.View(ctx) }, 1266, 780)
	tt.Frame()
	block := make(chan struct{})
	a.pending = "她做什么工作？"
	a.run(c, "AI 正在思考", func(ctx context.Context) (func(), error) { <-block; return nil, nil })
	a.talking = true
	a.live.add(discussion.StepReply, model.Delta{})
	a.live.add(discussion.StepReply, model.Delta{Reasoning: "她怕黑，职业要和光有关"})
	tt.Frame()
	if !tt.HasText("她怕黑，职业要和光有关") || !tt.HasText("正在思考…") {
		t.Fatal("reasoning not streamed", tt.Texts())
	}
	a.live.add(discussion.StepReply, model.Delta{Text: "可以让她**修灯**。"})
	tt.Frame()
	if !tt.HasText("正在回复…") || tt.HasText("**") {
		t.Fatal("reply not streamed as markdown", tt.Texts())
	}
	a.live.add(discussion.StepBookkeep, model.Delta{})
	tt.Frame()
	if !tt.HasText("正在把这一轮整理成思路卡片…") {
		t.Fatal("bookkeeping step not shown", tt.Texts())
	}
	close(block)
	settle(t, a, tt)
}

func TestEffortMenuSavesToWork(t *testing.T) {
	a, tt, _ := fixture(t)
	a.setTalk(func(s *workspace.Selection) { s.Provider, s.Model = "deepseek", "deepseek-flash" })
	a.setTalk(func(s *workspace.Selection) { s.Effort = string(model.EffortMax) })
	tt.Frame()
	if w := a.Store.Work().Discussion; w.Effort != "max" || w.Model != "deepseek-flash" || a.talkEffort != "max" {
		t.Fatalf("selection %+v", w)
	}
	if !tt.HasText("deepseek-flash") || !tt.HasText("Max") {
		t.Fatal("chip does not show model and effort", tt.Texts())
	}
}

// A reply's paragraphs, headings and lists are one text, which the author
// selects across; Copy takes its Markdown.
func TestReplyTextSelectsAcrossBlocksAndCopies(t *testing.T) {
	a, tt, _ := fixture(t)
	reply := "第一段。\n\n## 小标题\n\n- 甲\n- 乙\n\n第二段。\n\n```\ncode\n```"
	u, err := a.Store.Append(a.session, workspace.Event{Role: workspace.RoleUser, Text: "说说"})
	if err != nil {
		t.Fatal(err)
	}
	r, err := a.Store.Append(a.session, workspace.Event{Role: workspace.RoleAssistant, ReplyTo: u.ID, Text: reply})
	if err != nil {
		t.Fatal(err)
	}
	a.setError(a.refresh())
	tt.Frame()
	if !tt.HasText("第一段。\n\n小标题\n\n• 甲\n• 乙\n\n第二段。") || !tt.HasText("code") {
		t.Fatal("text blocks do not run together", tt.Texts())
	}
	flow, _ := tt.Find("第一段。\n\n小标题\n\n• 甲\n• 乙\n\n第二段。")
	tt.Press(flow.X+2, flow.Y+4)
	tt.Move(flow.X+flow.W/2, flow.Y+flow.H-4)
	tt.Release(flow.X+flow.W/2, flow.Y+flow.H-4)
	tt.Key(ui.Ctrl, ui.KeyC)
	if got := tt.Clipboard(); !strings.Contains(got, "第一段。") || !strings.Contains(got, "第二段") {
		t.Fatalf("a drag selected %q", got)
	}
	if err = tt.Click("复制-" + r.ID); err != nil {
		t.Fatal(err)
	}
	if tt.Clipboard() != reply {
		t.Fatalf("copied %q", tt.Clipboard())
	}
}
