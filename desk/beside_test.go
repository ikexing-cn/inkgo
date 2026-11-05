package desk

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"writer/prefs"
	"writer/workspace"
)

func TestPaneTabPreferences(t *testing.T) {
	a, _, id := writingFixture(t, "灯灭了。")
	config, root := a.ai.ConfigFile(), a.Store.Root()
	path := a.Store.PrefsPath()
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		old  map[string]any
		want paneTab
	}{
		{"legacy talk", map[string]any{"talk_tab": true}, paneTalk},
		{"legacy fore", map[string]any{"fore_tab": true}, paneFore},
		{"legacy fore wins", map[string]any{"talk_tab": true, "fore_tab": true}, paneFore},
		{"named phone wins", map[string]any{"pane_tab": "phone.preview", "talk_tab": true, "fore_tab": true}, paneTab("phone.preview")},
		{"named margin", map[string]any{"pane_tab": "margin"}, paneMargin},
		{"named setting", map[string]any{"pane_tab": "setting"}, paneSetting},
		{"empty tab falls back", map[string]any{"pane_tab": "", "talk_tab": true, "fore_tab": true}, paneMargin},
		{"no remembered tab", map[string]any{}, paneMargin},
		{"unknown tab falls back", map[string]any{"pane_tab": "unknown"}, paneMargin},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.old["writing"], tc.old["chapter"] = true, id
			data, err := json.Marshal(tc.old)
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			b := newApp(config)
			defer b.Close()
			if err = b.Open(root); err != nil {
				t.Fatal(err)
			}
			tt := stillTester(b.View, 1266, 780)
			tt.Frame()
			tt.Frame()
			if b.doc.tab != tc.want {
				t.Fatalf("tab %q want %q", b.doc.tab, tc.want)
			}
			text := map[paneTab]string{paneMargin: "本章没有批注", paneSetting: "主角怕黑", paneTalk: "附上本章内容", paneFore: "暂无伏笔", paneTab("phone.preview"): "第 1 屏，共 1 屏"}[tc.want]
			if !tt.HasText(text) {
				t.Fatal("wrong visible pane", tt.Texts())
			}
			if tc.want == paneTalk {
				b.turnAI(!(true))
				tt.Frame()
				if b.doc.tab != paneTalk || talkOf(b).TalkShown() || !tt.HasText("主角怕黑") {
					t.Fatal("AI off changed the remembered tab")
				}
				b.turnAI(!(false))
				tt.Frame()
				if !talkOf(b).TalkShown() || !tt.HasText("附上本章内容") {
					t.Fatal("AI on lost the talk tab")
				}
			}
			click(t, tt, "右栏-预览")
			b.savePrefs()
			data, err = os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var saved map[string]json.RawMessage
			if err = json.Unmarshal(data, &saved); err != nil {
				t.Fatal(err)
			}
			if string(saved["pane_tab"]) != `"phone.preview"` || saved["talk_tab"] != nil || saved["fore_tab"] != nil {
				t.Fatalf("new prefs: %s", data)
			}
			if err = b.Close(); err != nil {
				t.Fatal(err)
			}
			c := newApp(config)
			defer c.Close()
			if err = c.Open(root); err != nil {
				t.Fatal(err)
			}
			ct := stillTester(c.View, 1266, 780)
			ct.Frame()
			ct.Frame()
			if c.doc.tab != paneTab("phone.preview") || !ct.HasText("第 1 屏，共 1 屏") {
				t.Fatal("preview not restored", ct.Texts())
			}
			click(t, ct, "右栏-设定")
			data, err = os.ReadFile(filepath.Join(root, ".writer", "desk.json"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(data), `"pane_tab":"setting"`) || strings.Contains(string(data), "talk_tab") || strings.Contains(string(data), "fore_tab") {
				t.Fatalf("setting should keep its named tab: %s", data)
			}
		})
	}
}

type inspectedPrefs struct {
	prefs.Prefs
	Shelved  []string `json:"shelved_sessions"`
	NoAttach bool     `json:"no_attach"`
}

func savedPrefs(t *testing.T, a *App) inspectedPrefs {
	t.Helper()
	var p prefs.Prefs
	b, err := os.ReadFile(a.Store.PrefsPath())
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(b, &p); err != nil {
		t.Fatal(err)
	}
	out := inspectedPrefs{Prefs: p}
	_ = json.Unmarshal(p.Extensions["talk"], &out)
	return out
}

// The writing page's right column holds the talk too, and keeps which it
// shows and how wide the author dragged it.
func TestTalkBesideTheProse(t *testing.T) {
	a, tt, _ := writingFixture(t, "雨下了一夜。她推开门。")
	if !tt.HasText("本章没有批注") || a.doc.tab != paneMargin {
		t.Fatal("the margin is not shown first", tt.Texts())
	}
	if err := tt.Click("右栏-讨论"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("附上本章内容") || tt.HasText("主角怕黑") || paneTab(savedPrefs(t, a).PaneTab) != paneTalk {
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
	if talkOf(a).Input != "雨\n" || a.busy != "" {
		t.Fatalf("Enter in the column: input %q, busy %q", talkOf(a).Input, a.busy)
	}
	talkOf(a).Input = ""

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
	markUnchecked(a)
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
	setChapterOutline(a, "她回到旧城。")
	shownEditor(a).Select(6, 11)
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyL)
	tt.Frame()
	if len(talkOf(a).Quotes) != 1 || talkOf(a).Quotes[0] != (workspace.Quote{Chapter: id, Title: "第一章", Text: "她推开门。"}) || shownTab(a) != paneTalk {
		t.Fatalf("quotes %+v", talkOf(a).Quotes)
	}
	if !tt.HasText("《第一章》") || !tt.HasText("她推开门。") {
		t.Fatal("the quote is not in the composer", tt.Texts())
	}
	b := talkOf(a).Beside()
	if b == nil || b.Title != "第一章" || b.Outline != "她回到旧城。" || b.Before != "雨下了一夜。" {
		t.Fatalf("beside %+v", b)
	}
	if err := tt.Click("去掉引用-new-0"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(talkOf(a).Quotes) != 0 {
		t.Fatal("the quote stayed")
	}
	if err := tt.Click("附上本章内容"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if talkOf(a).Beside() != nil || !savedPrefs(t, a).NoAttach {
		t.Fatal("the chapter is still sent")
	}
}

// A quote in the talk, and an entry taken from one, find their words in
// the chapter; the quote goes by the chapter's name now.
func TestQuotesFindTheirProse(t *testing.T) {
	a, tt, id := writingFixture(t, "雨下了一夜。她推开门。")
	q := workspace.Quote{Chapter: id, Title: "第一章", Text: "她推开门。"}
	u, err := a.Store.Append(talkOf(a).Session, workspace.Event{Role: workspace.RoleUser, Text: "这里太急？", Chapter: id, Quotes: []workspace.Quote{q}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.Store.Revise(workspace.Revision{By: workspace.ByAuthor, Ops: []workspace.Op{{Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: "人物", Text: "她做事急",
		Sources: []workspace.Source{{Session: talkOf(a).Session, Message: u.ID, Chapter: id, Quote: "她推开门。"}}}}}); err != nil {
		t.Fatal(err)
	}
	a.SetError(a.refresh())
	if err = a.Store.RenameChapter(id, "雨夜"); err != nil {
		t.Fatal(err)
	}
	a.warn(a.loadTree())
	shownEditor(a).MoveTo(0, false)
	showPage(a, pageTalk)
	tt.Frame()
	tt.Frame()
	if !tt.HasText("《雨夜》") || tt.HasText("《第一章》") {
		t.Fatal("the quote does not go by the chapter's name now", tt.Texts())
	}
	if err := tt.Click("引用-" + u.ID + "-0"); err != nil {
		t.Fatal(err, tt.Texts())
	}
	tt.Frame()
	if s, e := shownEditor(a).Selection(); shownPage(a) != pageWrite || s != 6 || e != 11 {
		t.Fatalf("page %d selection %d-%d", shownPage(a), s, e)
	}

	shownEditor(a).MoveTo(0, false)
	showTab(a, paneSetting)
	tt.Frame()
	var entry string
	for _, e := range setting(a).Entries {
		if e.Text == "她做事急" {
			entry = e.ID
		}
	}
	showEntry(a, entry) // its page has the way to where it comes from
	tt.Frame()
	if err := tt.Click("原文-" + entry); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if s, e := shownEditor(a).Selection(); s != 6 || e != 11 {
		t.Fatalf("selection %d-%d", s, e)
	}
	shownEditor(a).Select(6, 7)
	shownEditor(a).Insert("他")
	tt.Frame()
	if err := tt.Click("原文-" + entry); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !strings.Contains(hintOf(a), "在本章中未找到") {
		t.Fatalf("notice %q", hintOf(a))
	}
}

// A turn beside the prose shows its progress in the column, not again
// across the top of the page, which would push the prose down.
func TestTalkBesideShowsItsOwnProgress(t *testing.T) {
	a, _, _ := writingFixture(t, "雨下了一夜。")
	var c *ui.Context
	tt := stillTester(func(ctx *ui.Context) { c = ctx; a.View(ctx) }, 1266, 780)
	tt.Frame()
	if err := tt.Click("右栏-讨论"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	block := make(chan struct{})
	talkOf(a).Turn.Pending = "这一章怎么样？"
	a.run(c, "AI 正在思考", func(ctx context.Context) (func(), error) { <-block; return nil, nil })
	talkOf(a).Turn.Active, talkOf(a).Turn.Session = true, talkOf(a).Session
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
	talkOf(a).Talks = false
	tt.Key(0, ui.KeyF11)
	tt.Frame()
	if !a.focused() || !tt.HasText("AI 正在思考") {
		t.Fatal("with the prose alone the progress is not shown", tt.Texts())
	}
	close(block)
	settle(t, a, tt)
}
