package desk

import (
	"slices"
	"testing"

	"writer/workspace"
)

// On the writing page each entry of the setting reads as it does at the
// chapter open, its row saying from where, or that it changes later; its
// page lists how it changes from chapters on, marks the one the chapter
// reads as, and notes, changes and deletes one. What the AI suggests
// waits till the author takes it. A change whose chapter is in the trash
// holds no more.
func TestAnEntryChangesFromAChapter(t *testing.T) {
	a, tt, ids := paneFixture(t)
	one := shownChapter(a)
	node := func(title string) string {
		id, err := a.Store.NewChapter("", title, false)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	two, three := node("第2章：井沿"), node("第3章：停电")
	r, err := a.Store.Revise(workspace.Revision{By: workspace.ByAuthor, Ops: []workspace.Op{
		{Op: workspace.OpChange, Entry: ids.zhou, From: three, Place: workspace.PlaceBody, Text: "配角，左手的事说开了。"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	later := r.Ops[0].Change
	a.SetError(a.loadTree())
	a.SetError(a.refresh())
	tt.Frame()
	has := func(why string, want ...string) {
		t.Helper()
		for _, s := range want {
			if !tt.HasText(s) {
				t.Fatalf("%s: no %q in %q", why, s, tt.Texts())
			}
		}
	}
	hasNot := func(why string, not ...string) {
		t.Helper()
		for _, s := range not {
			if tt.HasText(s) {
				t.Fatalf("%s: %q in %q", why, s, tt.Texts())
			}
		}
	}
	has("before the change", "之后有变化", "配角，林晚的同事。左手总揣在兜里，从来不拿出来。")
	a.openNode(three)
	tt.Frame()
	has("at the change", "第3章起", "配角，左手的事说开了。")
	hasNot("at the change", "之后有变化")

	if err = tt.Click("设定-" + ids.zhou); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	has("the page", "配角，林晚的同事。左手总揣在兜里，从来不拿出来。", "随章节的变化", "第3章起", "本章适用")
	hasNot("the page", "从本章起有变化")

	// From a chapter before, a change starts from how the entry reads
	// there.
	a.openNode(two)
	tt.Frame()
	has("a chapter before", "本章适用上面原来的样子。", "从本章起有变化")
	if slices.Contains(tt.Texts(), "本章适用") {
		t.Fatal("a change holds before its chapter")
	}
	if err = tt.Click("从本章起有变化"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	has("the form", "周祥 · 第2章起", "从哪一章起", "从这一章起，这一条就是下面这段话；起草、改写这一章及以后的章时，AI 看到的是它。")
	if lorePanel(a).Doc.Text != "配角，林晚的同事。左手总揣在兜里，从来不拿出来。" || lorePanel(a).Doc.From != two {
		t.Fatalf("the form starts at %q from %s", lorePanel(a).Doc.Text, lorePanel(a).Doc.From)
	}
	hasNot("a new change", "删除这处变化")
	lorePanel(a).Doc.Text = "配角，在井边等她。"
	if err = tt.Click("保存"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	has("saved", "第2章起", "配角，在井边等她。", "本章适用", "第3章起")
	e, _ := setting(a).Entry(ids.zhou)
	if len(e.Changes) != 2 || e.Changes[1].From != two || !e.Changes[1].Author {
		t.Fatalf("%+v", e.Changes)
	}
	mine := e.Changes[1].ID

	// The one from the third chapter, moved to the second, takes the place
	// of the one there.
	if err = tt.Click("修改变化-" + later); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	has("changing", "周祥 · 第3章起", "删除这处变化")
	lorePanel(a).Doc.From = two
	if err = tt.Click("保存"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	e, _ = setting(a).Entry(ids.zhou)
	if hold := e.Holding(chapters(a).Order()); len(hold) != 1 || hold[0].ID != later || hold[0].From != two || e.Changes[1].ID != mine || e.Changes[1].Place != workspace.PlaceDropped {
		t.Fatalf("%+v", e.Changes)
	}
	if err = tt.Click("修改变化-" + later); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err = tt.Click("删除这处变化"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	has("deleted", "从本章起有变化")
	hasNot("deleted", "随章节的变化", "配角，左手的事说开了。")

	// What the AI suggests waits in what the author decides.
	u, _ := a.Store.Append(talkOf(a).Session, workspace.Event{Role: workspace.RoleUser, Text: "第3章起他学会了修灯"})
	r, err = a.Store.Revise(workspace.Revision{By: workspace.ByTalk, Session: talkOf(a).Session, Turn: u.ID, Ops: []workspace.Op{
		{Op: workspace.OpChange, Entry: ids.zhou, From: three, Place: workspace.PlaceOpen, Text: "配角，学会了修灯。"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	idea := r.Ops[0].Change
	a.SetError(a.refresh())
	tt.Frame()
	has("an idea on the page", "AI 建议", "第3章起", "配角，学会了修灯。")
	hasNot("an idea on the page", "随章节的变化")
	showEntry(a, "")
	showTodo(a, true)
	tt.Frame()
	has("an idea waiting", "关于 周祥 · 第3章起", "配角，学会了修灯。")
	if err = tt.Click("采纳-" + idea); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	e, _ = setting(a).Entry(ids.zhou)
	if g, ok := e.At(chapters(a).Order(), three); !ok || g.ID != idea {
		t.Fatalf("taken %+v", e.Changes)
	}

	// The chapter in the trash, the change holds no more; the history says
	// what was done.
	if err = a.Store.TrashChapter(three); err != nil {
		t.Fatal(err)
	}
	a.SetError(a.loadTree())
	a.openNode(one)
	showTodo(a, false)
	showEntry(a, ids.zhou)
	tt.Frame()
	has("in the trash", "所在章节已删除 · 不起作用", "配角，学会了修灯。", "从本章起有变化")
	showEntry(a, "")
	showHistory(a, true)
	tt.Frame()
	has("the history", "周祥 · 第2章起：配角，在井边等她。", "删除变化 · 周祥 · 第2章起", "新增到待确认 · 周祥 · 所在章节已删除：配角，学会了修灯。")
	showHistory(a, false)

	// With the AI off the form says nothing of it.
	a.turnAI(false)
	showEntry(a, ids.zhou)
	editChange(a, "new")
	lorePanel(a).Doc.From, lorePanel(a).Doc.Text = one, "x"
	tt.Frame()
	has("off", "从这一章起，这一条就是下面这段话。")
	hasNot("off", "AI 看到的是它")
}
