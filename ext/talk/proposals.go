package talk

import (
	"github.com/egoist/mygo/ui"
	"slices"
	"writer/desk/kit"
	"writer/ext/talk/talkapi"
	"writer/ext/talk/talktext"
	"writer/locales"
	"writer/workspace"
)

// fateNames say what came of a proposal, in the card of its talk.
var fateNames = map[string]func() string{
	workspace.FateTaken:    talktext.ChatFateAdopted,
	workspace.FateDropped:  locales.HooksDropped,
	workspace.FateReplaced: talktext.ChatFateReplaced,
	workspace.FateVoid:     talktext.ChatFateChanged,
	workspace.FateClosed:   talktext.ChatFateBookClosed,
	workspace.FateOff:      talktext.ChatFateAiOff,
}

// whereNames say where a proposal goes, in its card.
var whereNames = map[string]func() string{
	workspace.ProposeReplace: talktext.ChatWhereSwap,
	workspace.ProposeAfter:   talktext.ChatWhereAfter,
	workspace.ProposeEnd:     talktext.ChatWhereEnd,
	workspace.ProposeAll:     talktext.ChatWhereAll,
}

// propCard shows, in the reply of the talk session, a change it proposed
// in the prose, by the result e of the call that proposed it.
// PropCard runs on the frame thread.
func (a *Panel) PropCard(c *ui.Context, session string, e workspace.Event) {
	from := slices.IndexFunc(a.Events, func(u workspace.Event) bool { return u.ID == e.ReplyTo })
	a.PropCardOf(c, *e.Proposal, a.pending(session, e.Call), workspace.FateOf(a.Events, max(from, 0), e.Call))
}

// turnCards show what the turn in flight proposed: as it comes, and as
// it came whole, in the order it began.
// TurnCards runs on the frame thread.
func (a *Panel) TurnCards(c *ui.Context, steps []workspace.Event) {
	var calls []string
	ps := map[string]workspace.Proposal{}
	for _, s := range a.Turn.Live.Proposals() {
		calls, ps[s.Call] = append(calls, s.Call), s.Proposal
	}
	for _, e := range steps {
		if e.Role == workspace.RoleResult && e.Proposal != nil {
			if _, ok := ps[e.Call]; !ok {
				calls = append(calls, e.Call)
			}
			ps[e.Call] = *e.Proposal
		}
	}
	for _, call := range calls {
		a.PropCardOf(c, ps[call], a.pending(a.Turn.Session, call), workspace.FateOf(a.Events, 0, call))
	}
}

// writingProps tells whether the turn in flight is writing a proposal.
// WritingProps runs on the frame thread.
func (a *Panel) WritingProps() bool {
	for _, s := range a.Turn.Live.Proposals() {
		if p := a.pending(a.Turn.Session, s.Call); p != nil && p.writing {
			return true
		}
	}
	return false
}

// propCardOf shows the proposal p: which chapter, where in it, how long,
// and what came of it, by fate or by live, the proposal while undecided.
// Undecided, it links to where it shows.
// PropCardOf runs on the frame thread.
func (a *Panel) PropCardOf(c *ui.Context, p workspace.Proposal, live *pendingProposal, fate string) {
	t := a.palette().Theme
	state := ""
	if label := fateNames[fate]; label != nil {
		state = label()
	}
	switch {
	case state != "":
	case live != nil && live.writing:
		state = locales.EditorWriting()
	case live != nil:
		state = talktext.ChatFateWaiting()
	default:
		state = talktext.ChatFateGone()
	}
	where := ""
	if label := whereNames[p.At]; label != nil {
		where = label()
	}
	title := a.nodeTitle(p.Chapter)
	if title == "" {
		title = p.Title
	}
	ui.Column(c).FillWidth().Gap(2).Padding(8, 10).Radius(kit.RCard).Border(1, a.palette().Line).Background(a.palette().Side).Children(func() {
		ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
			color := t.TextMuted
			if live != nil {
				color = t.Success
			}
			kit.Glyph(c, kit.IcPen, kit.IconSm, color)
			ui.Text(c, talktext.ChatProposeEdit(title)).FontSize(kit.FsControl).SingleLine().Ellipsis("…").Grow(1).Shrink(1)
			if live != nil && a.palette().Ghost(c, talktext.ChatViewInText(), ui.Color{}).Clicked() {
				a.suggestions.ShowProposal(live.session, live.call)
			}
		})
		ui.Text(c, talktext.ChatProposalMeta(where, a.Store.Work().Language.Count(p.Text), state, a.Store.Work().Language.Unit())).FontSize(kit.FsNote).TextColor(t.TextMuted)
	})
}

type pendingProposal struct {
	session, call string
	writing       bool
}

func (a *Panel) pending(session, call string) *pendingProposal {
	ok, writing := a.suggestions.State(session, call)
	if !ok {
		return nil
	}
	return &pendingProposal{session, call, writing}
}

// UpdateSuggestions transfers a copied stream snapshot on the frame thread.
func (a *Panel) UpdateSuggestions() {
	if a.Store == nil || a.Turn == nil {
		return
	}
	_, _, steps, _ := a.Turn.Live.Read()
	f := talkapi.ProposalFrame{Active: a.Turn.Active, Session: a.Turn.Session, Chapter: a.Turn.Chapter, Visible: a.TalkShown() && a.Session == a.Turn.Session, Looking: len(workspace.Looks(steps, "")) > 0, Stop: a.cancel}
	models := map[string]string{}
	for _, e := range steps {
		for _, c := range e.Calls {
			models[c.ID] = e.Model
		}
	}
	for _, s := range a.Turn.Live.Proposals() {
		f.Updates = append(f.Updates, talkapi.ProposalUpdate{Call: s.Call, Proposal: s.Proposal, Writing: true})
	}
	for _, e := range steps {
		if e.Role != workspace.RoleResult {
			continue
		}
		if e.Proposal != nil {
			f.Updates = append(f.Updates, talkapi.ProposalUpdate{Call: e.Call, Proposal: *e.Proposal, Model: models[e.Call]})
		} else {
			f.Failed = append(f.Failed, e.Call)
		}
	}
	a.suggestions.Follow(f)
}
