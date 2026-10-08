package desk

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
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
