package desk

import (
	"context"
	"reflect"
	"slices"
	"testing"
	continuation "writer/ext/continue"
	"writer/workspace"

	"github.com/egoist/mygo/ui"
	"writer/ext"
	"writer/ext/ai/aiapi"
	"writer/ext/hostapi"
	"writer/ext/talk/talkapi"
)

func TestTalkOffRemovesPageAndReturnsToWritingGloballyAndPerBook(t *testing.T) {
	for _, book := range []bool{false, true} {
		a, tt, _ := fixture(t)
		showPage(a, pageTalk)
		a.setExtension("talk", false, book)
		tt.Frame()
		if shownPage(a) != pageWrite || a.pages[pageTalk].available(a) {
			t.Fatal("discussion page remained")
		}
		if _, ok := ext.Lookup(a.extensions, talkapi.Service); ok {
			t.Fatal("conversation service remained")
		}
		a.setExtension("talk", true, book)
		tt.Frame()
		if !a.pages[pageTalk].available(a) {
			t.Fatal("discussion page did not return")
		}
	}
}

func TestTalkOffRemovesPaneTab(t *testing.T) {
	a, tt, _ := writingFixture(t, "正文")
	a.setExtension("talk", false, false)
	tt.Frame()
	for _, tab := range a.paneTabs {
		if tab.id == paneTalk && tab.available(a) {
			t.Fatal("discussion tab remained")
		}
	}
}

func TestTalkOffMakesControlLDoNothingAndRemovesQuoteAction(t *testing.T) {
	a, tt, _ := writingFixture(t, "正文引用")
	shownEditor(a).Select(0, 4)
	a.setExtension("talk", false, false)
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyL)
	tt.Frame()
	if len(talkOf(a).Quotes) != 0 || slices.ContainsFunc(ext.Items(a.extensions, hostapi.Commands), func(c ext.Item[hostapi.Command]) bool { return c.ID == "talk.quote" }) {
		t.Fatal("quote action remained")
	}
}

func TestTalkOffRemovesDraftAndRewrite(t *testing.T) {
	for _, bookScope := range []bool{false, true} {
		t.Run(map[bool]string{false: "global", true: "book"}[bookScope], func(t *testing.T) {
			a, tt, _ := writingFixture(t, "正文")
			setChapterOutline(a, "章纲")
			a.setExtension("talk", false, bookScope)
			tt.Frame()
			if draftCommand(a).present(a) || a.rewriteCommand().present(a) {
				t.Fatal("draft or rewrite remained")
			}

		})
	}
}

type cancellableTalk struct{ started chan context.Context }

func (t cancellableTalk) Say(ctx context.Context, _ string, _ workspace.Event) (bool, error) {
	t.started <- ctx
	<-ctx.Done()
	return false, ctx.Err()
}
func (t cancellableTalk) Retry(ctx context.Context, _, _ string) error {
	<-ctx.Done()
	return ctx.Err()
}

func TestTalkOffSynchronouslyCancelsReply(t *testing.T) {
	a, _, _ := fixture(t)
	started := make(chan context.Context, 1)
	talkOf(a).Build = func(*workspace.Store, aiapi.Material, []aiapi.Tool, *talkapi.Beside, func(talkapi.Progress), func(workspace.Event)) (talkapi.Talk, error) {
		return cancellableTalk{started}, nil
	}
	sent := false
	stillTester(func(c *ui.Context) {
		if !sent {
			sent = true
			talkOf(a).Say(c, talkOf(a).Session, workspace.Event{Text: "test"}, nil)
		}
	}, 64, 64)
	ctx := <-started
	a.setExtension("talk", false, false)
	if ctx.Err() != context.Canceled || a.busy != "" {
		t.Fatal("reply survived extension shutdown")
	}
}

func TestTalkModelSectionBelongsToTalkAndHostChoicesContainNoTalkFields(t *testing.T) {
	a, _, _ := fixture(t)
	found := false
	for _, item := range ext.Items(a.extensions, hostapi.SettingsSections) {
		if item.Owner == "talk" && item.Value.Placement == "ai.models" {
			found = true
		}
	}
	if !found {
		t.Fatal("talk model settings contribution missing")
	}
	typ := reflect.TypeOf(continuation.ModelChoices{})
	for _, name := range []string{"TalkProvider", "TalkModel", "TalkEffort"} {
		if _, ok := typ.FieldByName(name); ok {
			t.Fatal("host owns talk setting", name)
		}
	}
}

func TestDisablingTalkDuringRealStreamingDraftStopsReplyAndDiscardsPartialProposal(t *testing.T) {
	for _, bookScope := range []bool{false, true} {
		t.Run(map[bool]string{false: "global", true: "book"}[bookScope], func(t *testing.T) {
			a, tt, _ := writingFixture(t, "")
			f := &script{}
			penWith(a, f)
			hold := f.holding()
			defer close(hold)
			f.then(proposal("第一章", "end", "", "雨下了一夜。\n她醒了。"))
			setChapterOutline(a, "她在雨夜醒来。")
			draftOf(a).OpenDraft()
			stillTester(func(c *ui.Context) {
				if draftOf(a).Open {
					draftOf(a).Draft(c)
				}
			}, 64, 64)
			until(t, tt, "streamed proposal", func() bool { return len(shownProposals(a)) > 0 })
			a.setExtension("talk", false, bookScope)
			until(t, tt, "cancelled worker", func() bool { return a.busy == "" })
			if len(shownProposals(a)) != 0 || len(proposeOf(a).Props) != 0 || shownEditor(a).Doc.String() != "" {
				t.Fatal("partial reply remained")
			}
			a.setExtension("talk", true, bookScope)
			tt.Frame()
			if len(talkOf(a).Events) == 0 {
				events, err := a.Store.Events(talkOf(a).Session)
				t.Fatal("conversation disappeared", talkOf(a).Session, len(events), err, a.extensions.Ledger("talk").Errors)
			}

		})
	}
}

func TestTalkOffRemovesLoreConversationLinksAndReenableRestoresThem(t *testing.T) {
	for _, bookScope := range []bool{false, true} {
		t.Run(map[bool]string{false: "global", true: "book"}[bookScope], func(t *testing.T) {
			a, tt, _ := writingFixture(t, "正文")
			user, err := a.Store.Append(talkOf(a).Session, workspace.Event{Role: workspace.RoleUser, Text: "conversation link"})
			if err != nil {
				t.Fatal(err)
			}
			_, err = a.Store.Revise(workspace.Revision{By: workspace.ByTalk, Session: talkOf(a).Session, Turn: user.ID, Model: "deepseek/deepseek-flash", Ops: []workspace.Op{{Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: "人物", Text: "来自讨论的设定"}}})
			if err != nil {
				t.Fatal(err)
			}
			if err := a.refresh(); err != nil {
				t.Fatal(err)
			}
			showTab(a, paneSetting)
			for _, on := range []bool{true, false, true} {
				a.setExtension("talk", on, bookScope)
				tt.Frame()
				if on && !tt.HasText("修改记录") {
					click(t, tt, "更多")
					click(t, tt, "修改记录")
				}
				tt.Frame()
				found := tt.HasText("查看讨论")
				if found != on {
					t.Fatalf("talk link on=%v found=%v: %v", on, found, tt.Texts())
				}
			}

		})
	}
}

func TestTalkLedgerRecordsPagePaneSelectionMenuAndControlLWhenOnAndOff(t *testing.T) {
	a, _, _ := fixture(t)
	want := map[string]string{"talk.page": string(hostapi.Pages), "talk.pane": string(hostapi.PaneTabs), "talk.quote-menu": string(hostapi.SelectionMenus), "talk.quote": string(hostapi.Commands)}
	for _, on := range []bool{true, false, true} {
		a.setExtension("talk", on, false)
		got := map[string]string{}
		for _, c := range a.extensions.Ledger("talk").Contributions {
			got[c.ID] = c.Point
		}
		for id, point := range want {
			if got[id] != point {
				t.Fatalf("%s recorded at %q, want %q (on=%v)", id, got[id], point, on)
			}
		}
		active := map[string]bool{}
		for _, c := range ext.Items(a.extensions, hostapi.Pages) {
			active[c.ID] = true
		}
		for _, c := range ext.Items(a.extensions, hostapi.PaneTabs) {
			active[c.ID] = true
		}
		for _, c := range ext.Items(a.extensions, hostapi.SelectionMenus) {
			active[c.ID] = true
		}
		for _, c := range ext.Items(a.extensions, hostapi.Commands) {
			active[c.ID] = true
		}
		for id := range want {
			if active[id] != on {
				t.Fatalf("%s active=%v, want %v", id, active[id], on)
			}
		}
		for _, c := range ext.Items(a.extensions, hostapi.Commands) {
			if c.ID == "talk.quote" && (!on || c.Value.Key != ui.KeyL || c.Value.Mods != ui.Cmd) {
				t.Fatal("invalid quote binding", c)
			}
		}
	}
}

func TestTalkContributionsDisappearGloballyAndPerBookAndReturnWithConversations(t *testing.T) {
	for _, bookScope := range []bool{false, true} {
		a, tt, id := writingFixture(t, "引用正文")
		session := talkOf(a).Session
		_, err := a.Store.Append(session, workspace.Event{Role: workspace.RoleUser, Text: "保留的讨论"})
		requireExtension(t, err)
		requireExtension(t, a.refresh())
		for _, on := range []bool{false, true} {
			a.setExtension("talk", on, bookScope)
			tt.Frame()
			for point, active := range map[string]bool{
				"page": slices.ContainsFunc(ext.Items(a.extensions, hostapi.Pages), func(i ext.Item[hostapi.Page]) bool { return i.Owner == "talk" }),
				"pane": slices.ContainsFunc(ext.Items(a.extensions, hostapi.PaneTabs), func(i ext.Item[hostapi.PaneTab]) bool { return i.Owner == "talk" }),
				"menu": slices.ContainsFunc(ext.Items(a.extensions, hostapi.SelectionMenus), func(i ext.Item[hostapi.SelectionMenu]) bool { return i.Owner == "talk" }),
				"key":  slices.ContainsFunc(ext.Items(a.extensions, hostapi.Commands), func(i ext.Item[hostapi.Command]) bool { return i.ID == "talk.quote" }),
			} {
				if active != on {
					t.Fatalf("book=%v %s active=%v want=%v", bookScope, point, active, on)
				}
			}
			a.page = pageWrite
			a.openNode(id)
			shownEditor(a).Select(0, 4)
			n := len(talkOf(a).Quotes)
			tt.Key(ui.Cmd, ui.KeyL)
			tt.Frame()
			if (len(talkOf(a).Quotes) > n) != on {
				t.Fatal("Ctrl+L does not follow the extension switch")
			}
		}
		events, err := a.Store.Events(session)
		requireExtension(t, err)
		if len(events) != 1 || events[0].Text != "保留的讨论" {
			t.Fatal("talk lost", events)
		}
	}
}

func TestTalkProposalBridgeKeepsProseSuggestionsAndPendingSettingIdeas(t *testing.T) {
	a, tt, id := writingFixture(t, "原来的正文。")
	p := talkOf(a)
	a.busy = "reply"
	p.Turn.Active = true
	p.Turn.Session = p.Session
	t.Cleanup(func() { a.busy = ""; p.Turn.Active = false })
	proposal := workspace.Proposal{Chapter: id, Title: "第一章", At: workspace.ProposeEnd, Text: "新增的句子。"}
	p.Turn.Live.Add(talkapi.Progress{Call: "edit-call", Proposal: &proposal})
	p.UpdateSuggestions()
	tt.Frame()
	if len(proposeOf(a).Props) != 1 || proposeOf(a).Props[0].Text != "新增的句子。" || shownEditor(a).Doc.String() != "原来的正文。" {
		t.Fatal("stream did not reach prose as an undecided suggestion")
	}
	p.Turn.Live.Step(workspace.Event{Role: workspace.RoleResult, Call: "edit-call", Proposal: &proposal})
	p.UpdateSuggestions()
	if proposeOf(a).Props[0].Writing {
		t.Fatal("completed proposal still streaming")
	}
	user, err := a.Store.Append(p.Session, workspace.Event{Role: workspace.RoleUser, Text: "记下一条待确认的设定"})
	requireExtension(t, err)
	rev, err := a.Store.Revise(workspace.Revision{By: workspace.ByTalk, Session: p.Session, Turn: user.ID, Ops: []workspace.Op{{Op: workspace.OpAdd, Place: workspace.PlaceOpen, Category: "人物", Text: "待作者确认的设定"}}})
	requireExtension(t, err)
	stillTester(func(c *ui.Context) {
		p.Stepped(p.Turn, c.Services(), workspace.Event{Role: workspace.RoleResult, Revision: rev.ID})
	}, 64, 64)
	tt.Frame()
	lorePanel(a).Waiting()
	tt.Frame()
	if !tt.HasText("待作者确认的设定") {
		t.Fatal("talk's pending setting idea did not reach the pending list", tt.Texts())
	}
}
