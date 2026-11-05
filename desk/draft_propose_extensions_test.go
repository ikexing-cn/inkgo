package desk

import (
	"context"
	"encoding/json"
	"github.com/egoist/mygo/ui"
	"testing"
	"writer/ext"
	"writer/ext/ai/aiapi"
	"writer/ext/hostapi"
	"writer/ext/marks/marksapi"
	"writer/ext/talk/talkapi"
	"writer/workspace"
)

func TestDraftAndProposeOffRemoveEveryContributionAndOnRestoresThemGloballyAndPerBook(t *testing.T) {
	for _, id := range []string{"draft", "propose"} {
		for _, book := range []bool{false, true} {
			t.Run(id+map[bool]string{false: "/global", true: "/book"}[book], func(t *testing.T) {
				a, tt, ch := writingFixture(t, "原文。")
				before := a.extensions.Ledger(id).Contributions
				if len(before) == 0 {
					t.Fatal("empty ledger")
				}
				if id == "propose" {
					proposeOf(a).Frame.Session = talkOf(a).Session
					proposeOf(a).TakeProposal("test", workspace.Proposal{Chapter: ch, Title: "第一章", At: workspace.ProposeEnd, Text: "提议。"}, false, "model")
					tt.Frame()
					if len(shownEditor(a).Proposals()) == 0 {
						t.Fatal("proposal missing")
					}
				}
				a.setExtension(id, false, book)
				tt.Frame()
				if hasOwner(ext.Items(a.extensions, hostapi.Commands), id) || hasOwner(ext.Items(a.extensions, hostapi.Overlays), id) || hasOwner(ext.Items(a.extensions, hostapi.SelectionMenus), id) || hasOwner(ext.Items(a.extensions, hostapi.BeforeProse), id) || hasOwner(ext.Items(a.extensions, hostapi.PendingGroups), id) || hasOwner(ext.Items(a.extensions, marksapi.MarginSources), id) || hasOwner(ext.Items(a.extensions, aiapi.Tools), id) {
					t.Fatal("disabled contributions survived")
				}
				if id == "propose" && (len(shownEditor(a).Proposals()) != 0 || len(ext.Items(a.extensions, talkapi.Proposals)) != 0) {
					t.Fatal("disabled proposals survived")
				}
				a.setExtension(id, true, book)
				tt.Frame()
				if len(a.extensions.Ledger(id).Contributions) != len(before) {
					t.Fatal("ledger lost declarations")
				}
				if id == "propose" && len(shownEditor(a).Proposals()) == 0 {
					t.Fatal("pending proposal did not return")
				}
			})
		}
	}
}
func TestDraftAndRewriteAreAbsentWhenTalkOffAndReturnWhenOn(t *testing.T) {
	for _, book := range []bool{false, true} {
		a, tt, _ := writingFixture(t, "原文。")
		shownEditor(a).Select(0, 2)
		a.setExtension("talk", false, book)
		tt.Frame()
		if draftCommand(a).present(a) || a.rewriteAvailable() {
			t.Fatal("talk commands remained")
		}
		a.setExtension("talk", true, book)
		tt.Frame()
		if !draftCommand(a).present(a) || !a.rewriteAvailable() {
			t.Fatal("talk commands did not return")
		}
	}
}

// TestEveryWritingAIFeatureLedgerKeepsItsContributionsWhileOff checks the
// ledger only: the details page does not list it until the author approves
// the prototype's 「加了什么」.
func TestEveryWritingAIFeatureLedgerKeepsItsContributionsWhileOff(t *testing.T) {
	a, _, _ := writingFixture(t, "原文")
	for _, id := range []string{"continue", "draft", "propose", "usage"} {
		before := a.extensions.Ledger(id)
		if len(before.Contributions) == 0 {
			t.Fatal(id, "empty ledger")
		}
		a.setExtension(id, false, false)
		if after := a.extensions.Ledger(id); len(after.Contributions) != len(before.Contributions) {
			t.Fatal(id, "lost ledger")
		}
	}
}

func (a *App) rewriteCommand() *command {
	for _, c := range a.extensionCommands() {
		if c.id == "propose.rewrite" {
			return c.cmd
		}
	}
	return nil
}
func (a *App) rewriteAvailable() bool { c := a.rewriteCommand(); return c != nil && c.available(a) }
func (a *App) runRewrite() {
	if c := a.rewriteCommand(); c != nil && c.available(a) {
		c.run(a)
	}
}

func TestProposeOffReceivesNoProposalAndHasNoCardActions(t *testing.T) {
	a, tt, _ := writingFixture(t, "原文。")
	a.setExtension("propose", false, false)
	tt.Frame()
	if len(ext.Items(a.extensions, talkapi.Proposals)) != 0 {
		t.Fatal("receiver remained")
	}
	p := talkOf(a)
	p.Events = []workspace.Event{{Role: workspace.RoleResult, Call: "old", Proposal: &workspace.Proposal{Title: "第一章", At: workspace.ProposeEnd, Text: "建议。"}}}
	if len(shownEditor(a).Proposals()) != 0 {
		t.Fatal("proposal displayed while off")
	}
	cards := stillTester(func(c *ui.Context) { p.PropCard(c, p.Session, p.Events[0]) }, 600, 200)
	cards.Frame()
	if cards.HasText("在正文中查看") {
		t.Fatal("disabled proposal card has an action")
	}
}
func TestMarksOptionalForProposalAcceptanceAndUnderlineReturns(t *testing.T) {
	a, tt, ch := writingFixture(t, "原文。")
	a.setExtension("marks", false, false)
	p := proposeOf(a)
	p.Frame.Session = talkOf(a).Session
	p.TakeProposal("optional", workspace.Proposal{Chapter: ch, Title: "第一章", At: workspace.ProposeEnd, Text: "建议。"}, false, "model")
	tt.Frame()
	if len(shownEditor(a).Proposals()) != 1 {
		t.Fatal("marks disabled proposals")
	}
	p.TakeProps(p.Props)
	if len(shownEditor(a).Doc.Marks()) == 0 {
		t.Fatal("accepted text has no provenance")
	}
	if len(ext.Items(a.extensions, marksapi.MarginSources)) != 0 {
		t.Fatal("marks source active while marks off")
	}
	a.setExtension("marks", true, false)
	tt.Frame()
	if !hasOwner(ext.Items(a.extensions, marksapi.MarginSources), "propose") {
		t.Fatal("underline source did not return")
	}
}

func TestProposeToolReceivesItsOwnCompletedProposalWithoutTalkResultHandover(t *testing.T) {
	a, tt, ch := writingFixture(t, "原文。")
	session := talkOf(a).Session
	proposeOf(a).Begin()
	for _, i := range ext.Items(a.extensions, aiapi.Tools) {
		if i.Owner != "propose" {
			continue
		}
		result := i.Value.Run(context.Background(), aiapi.ToolEnv{Store: a.Store, Shared: aiapi.NewToolShared(a.Store), Session: session, Call: "own", Model: "test"}, json.RawMessage(`{"chapter":"第一章","at":"end","text":"工具自己收到。"}`))
		if result.Failed || result.Proposal == nil {
			t.Fatal(result)
		}
	}
	proposeOf(a).Follow(talkapi.ProposalFrame{Session: session})
	tt.Frame()
	if len(proposeOf(a).Props) != 1 || proposeOf(a).Props[0].Chapter != ch {
		t.Fatal("completion required talk result handover")
	}
}
