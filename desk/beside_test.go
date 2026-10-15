package desk

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/workspace"
)

func savedPrefs(t *testing.T, a *App) prefs {
	t.Helper()
	var p prefs
	b, err := os.ReadFile(a.prefsPath())
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(b, &p); err != nil {
		t.Fatal(err)
	}
	return p
}

// The writing page's right column holds the talk too, and keeps which it
// shows and how wide the author dragged it.
func TestTalkBesideTheProse(t *testing.T) {
	a, tt, _ := writingFixture(t, "雨下了一夜。她推开门。")
	if !tt.HasText("主角怕黑") {
		t.Fatal("the setting is not shown first", tt.Texts())
	}
	if err := tt.Click("右栏-讨论"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("附上本章内容") || tt.HasText("主角怕黑") || !savedPrefs(t, a).TalkTab {
		t.Fatal("the talk is not shown", tt.Texts())
	}
	if !tt.HasText("这一章的节奏怎么样") || tt.HasText("右边的设定文档") {
		t.Fatal("the empty talk is not about the chapter", tt.Texts())
	}
	// Enter is a new line here as on the page.
	if err := tt.Click("讨论输入"); err != nil {
		t.Fatal(err)
	}
	tt.Type("雨")
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if a.input != "雨\n" || a.busy != "" {
		t.Fatalf("Enter in the column: input %q, busy %q", a.input, a.busy)
	}
	a.input = ""

	// Dragging the column's edge left widens it; the width is kept once
	// the author lets go.
	r := a.LastSetting
	tt.Press(r.X, r.Y+300)
	tt.Move(r.X-120, r.Y+300)
	tt.Frame()
	tt.Release(r.X-120, r.Y+300)
	tt.Frame()
	if got := a.LastSetting.W; got < 500 || got > 520 {
		t.Fatalf("width %v after dragging from %v", got, r.W)
	}
	if p := savedPrefs(t, a); p.PanelWidth != a.doc.width {
		t.Fatalf("width saved %v", p.PanelWidth)
	}
	tt.Press(a.LastSetting.X, r.Y+300)
	tt.Move(a.LastSetting.X+900, r.Y+300)
	tt.Frame()
	tt.Release(a.LastSetting.X+900, r.Y+300)
	tt.Frame()
	if a.LastSetting.W != panelMin {
		t.Fatalf("narrowed to %v", a.LastSetting.W)
	}

	// On a folder it is the volume's outline that goes along.
	if _, err := a.Store.NewChapter("", "第一卷", true); err != nil {
		t.Fatal(err)
	}
	a.write.checked = time.Time{}
	tt.Frame()
	tt.Frame()
	if err := openOutline(tt, "第一卷"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("附上本卷内容") || !tt.HasText("这一卷的节奏怎么样") || tt.HasText("附上本章内容") {
		t.Fatal("the folder's talk", tt.Texts())
	}
}

// Cmd+L quotes the words selected into the talk; the turn carries the
// chapter and the text before the caret unless the author turns it off.
func TestQuoteProseIntoTheTalk(t *testing.T) {
	a, tt, id := writingFixture(t, "雨下了一夜。她推开门。")
	a.write.outline = "她回到旧城。"
	a.write.ed.Select(6, 11)
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyL)
	tt.Frame()
	if len(a.quotes) != 1 || a.quotes[0] != (workspace.Quote{Chapter: id, Title: "第一章", Text: "她推开门。"}) || !a.doc.talk {
		t.Fatalf("quotes %+v", a.quotes)
	}
	if !tt.HasText("《第一章》") || !tt.HasText("她推开门。") {
		t.Fatal("the quote is not in the composer", tt.Texts())
	}
	b := a.beside()
	if b == nil || b.Title != "第一章" || b.Outline != "她回到旧城。" || b.Before != "雨下了一夜。" {
		t.Fatalf("beside %+v", b)
	}
	if err := tt.Click("去掉引用-new-0"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(a.quotes) != 0 {
		t.Fatal("the quote stayed")
	}
	if err := tt.Click("附上本章内容"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.beside() != nil || !savedPrefs(t, a).NoAttach {
		t.Fatal("the chapter is still sent")
	}
}

// A quote in the talk, and an entry taken from one, find their words in
// the chapter; the quote goes by the chapter's name now.
func TestQuotesFindTheirProse(t *testing.T) {
	a, tt, id := writingFixture(t, "雨下了一夜。她推开门。")
	q := workspace.Quote{Chapter: id, Title: "第一章", Text: "她推开门。"}
	u, err := a.Store.Append(a.session, workspace.Event{Role: workspace.RoleUser, Text: "这里太急？", Chapter: id, Quotes: []workspace.Quote{q}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.Store.Revise(workspace.Revision{By: workspace.ByAuthor, Ops: []workspace.Op{{Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: "人物", Text: "她做事急",
		Sources: []workspace.Source{{Session: a.session, Message: u.ID, Chapter: id, Quote: "她推开门。"}}}}}); err != nil {
		t.Fatal(err)
	}
	a.setError(a.refresh())
	if err = a.Store.RenameChapter(id, "雨夜"); err != nil {
		t.Fatal(err)
	}
	a.warn(a.loadTree())
	a.write.ed.MoveTo(0, false)
	a.page = 0
	tt.Frame()
	tt.Frame()
	if !tt.HasText("《雨夜》") || tt.HasText("《第一章》") {
		t.Fatal("the quote does not go by the chapter's name now", tt.Texts())
	}
	if err := tt.Click("引用-" + u.ID + "-0"); err != nil {
		t.Fatal(err, tt.Texts())
	}
	tt.Frame()
	if s, e := a.write.ed.Selection(); a.page != 3 || s != 6 || e != 11 {
		t.Fatalf("page %d selection %d-%d", a.page, s, e)
	}

	a.write.ed.MoveTo(0, false)
	a.doc.talk = false
	tt.Frame()
	var entry string
	for _, e := range a.set.Entries {
		if e.Text == "她做事急" {
			entry = e.ID
		}
	}
	a.doc.shown = entry // its page has the way to where it comes from
	tt.Frame()
	if err := tt.Click("原文-" + entry); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if s, e := a.write.ed.Selection(); s != 6 || e != 11 {
		t.Fatalf("selection %d-%d", s, e)
	}
	a.write.ed.Select(6, 7)
	a.write.ed.Insert("他")
	tt.Frame()
	if err := tt.Click("原文-" + entry); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !strings.Contains(a.notice, "在本章中未找到") {
		t.Fatalf("notice %q", a.notice)
	}
}

// A turn beside the prose shows its progress in the column, not again
// across the top of the page, which would push the prose down.
func TestTalkBesideShowsItsOwnProgress(t *testing.T) {
	a, _, _ := writingFixture(t, "雨下了一夜。")
	var c *ui.Context
	tt := ui.NewTester(func(ctx *ui.Context) { c = ctx; a.View(ctx) }, 1266, 780)
	tt.Frame()
	if err := tt.Click("右栏-讨论"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	block := make(chan struct{})
	a.pending = "这一章怎么样？"
	a.run(c, "AI 正在思考", func(ctx context.Context) (func(), error) { <-block; return nil, nil })
	a.talking, a.talkSession = true, a.session
	tt.Frame()
	if !tt.HasText("正在思考…") || tt.HasText("AI 正在思考") {
		t.Fatal("the progress is shown twice", tt.Texts())
	}
	a.setDocHidden(true)
	tt.Frame()
	if !tt.HasText("AI 正在思考") {
		t.Fatal("with the column shut the progress is not shown", tt.Texts())
	}
	a.setDocHidden(false)
	tt.Frame()
	if tt.HasText("AI 正在思考") {
		t.Fatal("the talk is back and the top still says it", tt.Texts())
	}
	if err := tt.Click("全部讨论"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("AI 正在思考") {
		t.Fatal("with the talks listed the progress is not shown", tt.Texts())
	}
	a.doc.talks = false
	tt.Key(0, ui.KeyF11)
	tt.Frame()
	if !a.focused() || !tt.HasText("AI 正在思考") {
		t.Fatal("with the prose alone the progress is not shown", tt.Texts())
	}
	close(block)
	settle(t, a, tt)
}
