package desk

import (
	"context"
	"reflect"
	"testing"
	"writer/workspace"

	"github.com/egoist/mygo/ui"
	"writer/ext"
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
	if len(a.quotes) != 0 || cmdQuote.present(a) {
		t.Fatal("quote action remained")
	}
}

func TestTalkOffRemovesDraftAndRewrite(t *testing.T) {
	a, tt, _ := writingFixture(t, "正文")
	setChapterOutline(a, "章纲")
	a.setExtension("talk", false, false)
	tt.Frame()
	if cmdDraft.present(a) || cmdRewrite.present(a) {
		t.Fatal("draft or rewrite remained")
	}
}

func TestTalkOffSynchronouslyCancelsReply(t *testing.T) {
	a, _, _ := fixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	a.cancel, a.busy, a.turn.active = cancel, "reply", true
	a.setExtension("talk", false, false)
	if ctx.Err() != context.Canceled {
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
	typ := reflect.TypeOf(hostapi.ModelChoices{})
	for _, name := range []string{"TalkProvider", "TalkModel", "TalkEffort"} {
		if _, ok := typ.FieldByName(name); ok {
			t.Fatal("host owns talk setting", name)
		}
	}
}

func TestDisablingTalkDuringRealStreamingDraftStopsReplyAndDiscardsPartialProposal(t *testing.T) {
	a, tt, _ := writingFixture(t, "")
	f := &script{}
	penWith(a, f)
	hold := f.holding()
	defer close(hold)
	f.then(proposal("第一章", "end", "", "雨下了一夜。\n她醒了。"))
	setChapterOutline(a, "她在雨夜醒来。")
	a.openDraft()
	stillTester(func(c *ui.Context) {
		if a.write.draftOpen {
			a.draft(c)
		}
	}, 64, 64)
	until(t, tt, "streamed proposal", func() bool { return len(shownProposals(a)) > 0 })
	a.setExtension("talk", false, false)
	until(t, tt, "cancelled worker", func() bool { return a.busy == "" })
	if len(shownProposals(a)) != 0 || len(a.write.props) != 0 || shownEditor(a).Doc.String() != "" {
		t.Fatal("partial reply remained")
	}
	a.setExtension("talk", true, false)
	tt.Frame()
	if len(a.events) == 0 {
		t.Fatal("conversation disappeared")
	}
}

func TestTalkOffRemovesLoreConversationLinksAndReenableRestoresThem(t *testing.T) {
	a, tt, _ := writingFixture(t, "正文")
	user, err := a.Store.Append(a.session, workspace.Event{Role: workspace.RoleUser, Text: "conversation link"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = a.Store.Revise(workspace.Revision{By: workspace.ByTalk, Session: a.session, Turn: user.ID, Model: "deepseek/deepseek-flash", Ops: []workspace.Op{{Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: "人物", Text: "来自讨论的设定"}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.refresh(); err != nil {
		t.Fatal(err)
	}
	showTab(a, paneSetting)
	for _, on := range []bool{true, false, true} {
		a.setExtension("talk", on, false)
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
}
