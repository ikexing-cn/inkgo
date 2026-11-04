package desk

import (
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"
	"writer/desk/kit"
	"writer/editor"
	"writer/locales"
	"writer/workspace"
)

// What an AI proposes in a talk to change the prose shows in its chapter:
// its new words green, after the words it would replace struck through,
// until the author takes it or lets it go. It is not saved; what came of
// it is, in its talk, which tells the AI the next time the author speaks
// there.

// prop is a change an AI proposed in a talk that the author has not
// decided on.
type prop struct {
	workspace.Proposal
	session, call string
	model         string // that proposed it, which the words taken are marked with
	base          string // the chapter's text as it was proposed, which one of all replaces
	near          int    // where its words were last found in the chapter
	writing       bool   // its text still comes
	fresh         bool   // came whole since it was last placed
	placed        bool   // shows in the editor of its chapter
	shown         bool   // was brought into view
}

// id names p to the editor.
func (p *prop) id() string { return p.session + "/" + p.call }

// propRoom is the room after each proposal in the prose for its buttons:
// 放弃 and 正在写… while it comes, then 放弃 and 采纳, 放弃 not moving. propInset is where
// they begin in it.
const (
	propRoom  = 132
	propInset = 10
)

// placedKey is what the proposals were last placed in the prose for: the
// editor shown, its text and the proposals.
type placedKey struct {
	ed        *editor.Editor
	ver, prop int
}

// propOf is the proposal the call of the talk session made, if it is
// still undecided.
func (w *writeView) propOf(session, call string) *prop {
	for _, p := range w.props {
		if p.session == session && p.call == call {
			return p
		}
	}
	return nil
}

// followProps takes in what the reply in flight proposes in the prose, as
// its text comes and once it came whole; one whose call failed goes, the
// AI being told why.
func (a *App) followProps() {
	f := a.proposalFrame
	if !f.Active || a.Store == nil {
		return
	}
	for _, u := range f.Updates {
		a.takeProposal(u.Call, u.Proposal, u.Writing, u.Model)
	}
	for _, call := range f.Failed {
		if p := a.write.propOf(f.Session, call); p != nil && p.writing {
			a.forget(p)
		}
	}
}

// takeProposal shows what the call of the turn in flight proposes: as far as
// its text came, or whole. A new one takes the place of those undecided
// where it goes, and its chapter, when not open, opens in a tab beside
// the one shown.
func (a *App) takeProposal(call string, p workspace.Proposal, writing bool, model string) {
	w := &a.write
	session := a.proposalFrame.Session
	key := session + "/" + call
	if w.seen[key] {
		pr := w.propOf(session, call)
		switch {
		case pr == nil || !pr.writing: // decided, or whole
		case writing:
			if pr.Text != p.Text {
				pr.Text = p.Text
				w.propVer++
			}
		default:
			pr.Proposal, pr.writing, pr.fresh, pr.model = p, false, true, model
			w.propVer++
		}
		return
	}
	if w.seen == nil {
		w.seen = map[string]bool{}
	}
	w.seen[key] = true
	pr := &prop{Proposal: p, session: session, call: call, model: model, writing: writing, base: a.chapterText(p.Chapter)}
	var over []*prop
	if at, ok := pr.forEditor(pr.base); ok {
		for _, q := range w.props {
			if q.Chapter != p.Chapter {
				continue
			}
			if qat, ok := q.forEditor(pr.base); ok && samePlace(at, qat, p.At == workspace.ProposeAll || q.At == workspace.ProposeAll) {
				over = append(over, q)
			}
		}
	}
	a.dropProps(over, workspace.FateReplaced)
	w.props = append(w.props, pr)
	w.propVer++
	if w.node != p.Chapter && !slices.Contains(w.tabs, p.Chapter) {
		w.tabs = append(w.tabs, p.Chapter)
		a.savePrefs()
	}
}

// samePlace tells whether two proposals go in the same place: the words of
// one among the other's, the same point, or either for the whole chapter.
func samePlace(p, q editor.Proposal, whole bool) bool {
	switch {
	case whole:
		return true
	case p.A == p.B && q.A == q.B:
		return p.A == q.A
	case p.A == p.B:
		return q.A < p.A && p.A < q.B
	case q.A == q.B:
		return p.A < q.A && q.A < p.B
	}
	return max(p.A, q.A) < min(p.B, q.B)
}

// unfinishedProps lets go, as a turn ends, what it was proposing and did
// not finish: it was cancelled, or failed.
func (a *App) unfinishedProps() {
	if !a.proposalFrame.Active {
		return
	}
	w := &a.write
	n := len(w.props)
	w.props = slices.DeleteFunc(w.props, func(p *prop) bool { return p.writing && p.session == a.proposalFrame.Session })
	if len(w.props) != n {
		w.propVer++
	}
}

// forget lets p go with nothing to tell of it.
func (a *App) forget(p *prop) {
	w := &a.write
	w.props = slices.DeleteFunc(w.props, func(q *prop) bool { return q == p })
	w.propVer++
}

// dropProps lets the proposals ps go, writing in their talks what came of
// them.
func (a *App) dropProps(ps []*prop, fate string) {
	if len(ps) == 0 {
		return
	}
	w := &a.write
	ps = slices.Clone(ps)
	shown := false
	for _, p := range ps {
		w.props = slices.DeleteFunc(w.props, func(q *prop) bool { return q == p })
		_, err := a.Store.Append(p.session, workspace.Event{Role: workspace.RoleFate, Call: p.call, Fate: fate})
		a.warn(err)
		shown = shown || true
	}
	w.propVer++
	if shown {
		a.warn(a.refresh())
	}
}

// letGo is the author letting the proposals ps go: one still coming stops
// the reply that writes it.
func (a *App) letGo(ps []*prop) {
	if slices.ContainsFunc(ps, func(p *prop) bool { return p.writing }) && a.proposalFrame.Stop != nil {
		a.proposalFrame.Stop()
	}
	a.dropProps(ps, workspace.FateDropped)
}

// takeProps puts the proposals ps of the chapter shown in its text, as one
// step to undo, after a snapshot of it as it was that names the talks they
// came from; their words are marked
// as the AI's, and the caret goes after the last. One still coming is
// left.
func (a *App) takeProps(ps []*prop) {
	w := &a.write
	text := w.cur().ed.Doc.String()
	var list []*prop
	var places []editor.Proposal
	var talks []string
	for _, p := range ps {
		if p.Chapter != w.node || p.writing {
			continue
		}
		if at, ok := p.forEditor(text); ok {
			list, places = append(list, p), append(places, at)
			if !slices.Contains(talks, p.session) {
				talks = append(talks, p.session)
			}
		}
	}
	if len(list) == 0 {
		return
	}
	if err := a.Store.SnapshotForTalks(w.node, text, storedMarks(w.cur().ed.Doc.Marks()), talks); err != nil {
		a.warn(err)
		return
	}
	w.cur().ed.Take(places, list[0].model)
	a.warn(a.saveText())
	a.dropProps(list, workspace.FateTaken)
	a.status.Did(locales.EditorProposalAccepted())
	w.cur().ed.Focus()
}

// chapterText is the text of the chapter of id as the page has it: in
// its tab's editor, or else on the disk.
func (a *App) chapterText(id string) string {
	if c := a.write.open[id]; c != nil && c.ed != nil {
		return c.ed.Doc.String()
	}
	ch, err := a.Store.ReadChapter(id)
	if err != nil {
		return ""
	}
	return ch.Text
}

func (p *prop) forEditor(text string) (editor.Proposal, bool) {
	at, near, ok := p.Proposal.Place(text, p.base, p.near)
	p.near = near
	return editor.Proposal{ID: p.id(), A: at.A, B: at.B, Keep: at.Keep, Apart: at.Apart, Text: at.Text}, ok
}

// placeProps shows the proposals of the chapter shown where they go in its
// text now. One whose words the author changed is void, once it came
// whole. One shown for the first time, or asked for, comes into view.
func (a *App) placeProps() {
	w := &a.write
	if w.cur().ed == nil {
		return
	}
	if key := (placedKey{w.cur().ed, w.cur().ed.Doc.Version(), w.propVer}); key != w.placed {
		text := w.cur().ed.Doc.String()
		var shown []editor.Proposal
		var void []*prop
		late := false
		for _, p := range w.props {
			if p.Chapter != w.node {
				continue
			}
			at, ok := p.forEditor(text)
			p.placed = ok
			switch {
			case ok:
				shown = append(shown, at)
				p.fresh = false
			case !p.writing:
				void = append(void, p)
				late = late || p.fresh
			}
		}
		w.cur().ed.SetProposals(shown, propRoom)
		w.placed = placedKey{w.cur().ed, w.cur().ed.Doc.Version(), w.propVer}
		switch {
		case len(void) == 1 && late:
			a.status.Hint(locales.EditorProposalVoid())
		case len(void) > 0:
			a.status.Hint(locales.EditorProposalsVoid(len(void)))
		}
		a.dropProps(void, workspace.FateVoid)
	}
	for _, p := range w.props {
		if p.Chapter == w.node && p.placed && !p.shown {
			p.shown = true
			w.cur().ed.RevealProposal(p.id())
		}
	}
}

// pruneProps lets go of what was proposed in a chapter that is no longer
// in the book.
func (a *App) pruneProps() {
	w := &a.write
	if a.book.Tree().Root == nil {
		return
	}
	var gone []*prop
	for _, p := range w.props {
		if _, ok := a.book.Tree().Node(p.Chapter); !ok {
			gone = append(gone, p)
		}
	}
	a.dropProps(gone, workspace.FateVoid)
}

// showProp shows the chapter of p on the writing page, with p in view.
func (a *App) showProp(p *prop) {
	w := &a.write
	a.page = pageWrite
	if w.cur().ed == nil || w.node != p.Chapter {
		a.openNode(p.Chapter)
	}
	p.shown = false
}

func (a *App) openTalk(session string) {
	if a.talkAPI != nil {
		a.talkAPI.OpenTalk(session)
		(extensionHost{a}).Show("talk")
	}
}

// propActs float in the room after each proposal shown: 放弃, then 采纳
// once it came whole, or while it comes that it does. 放弃 stays where it
// is as it comes whole: a click meant for it then is not on 采纳.
func (a *App) propActs(c *ui.Context) {
	t := a.pal.Theme
	w := &a.write
	var take, drop *prop
	for _, p := range w.props {
		if p.Chapter != w.node {
			continue
		}
		r, ok := w.cur().ed.ProposalGap(p.id())
		if !ok {
			continue
		}
		ui.Row(c.Key("提议-" + p.id())).Absolute().Left(r.X + propInset).Top(r.Y).Height(r.H).Gap(6).AlignItems(ui.Center).Children(func() {
			tip := locales.EditorDropOne()
			if p.writing {
				tip = locales.EditorStopDrop()
			}
			if a.pal.Soft(c, locales.CommonDiscard()).Tooltip(tip).KeepFocus().Clicked() {
				drop = p
			}
			if p.writing {
				ui.Text(c, locales.EditorWriting()).FontSize(kit.FsControl).TextColor(t.TextMuted).SingleLine()
			} else if a.pal.Primary(c, locales.EditorAccept()).Tooltip(locales.EditorAcceptOne()).KeepFocus().Clicked() {
				take = p
			}
		})
	}
	if drop != nil {
		a.letGo([]*prop{drop})
		w.cur().ed.Focus()
	}
	if take != nil {
		a.takeProps([]*prop{take})
	}
}

// aiBar is the line under the chapter's title for what the AI does in the
// chapter: the rewrite it is asked for, a talk opened on it while the AI
// thinks, or what it proposed there, to let go or take all at once. Its
// buttons give the keyboard back to the prose: taken, an undo takes it
// back.
func (a *App) aiBar(c *ui.Context) {
	t := a.pal.Theme
	w := &a.write
	bar := func(fn func()) {
		a.overProse(c, 10, 0, func() {
			ui.Row(c).FillWidth().Padding(6, 8, 6, 12).Gap(8).AlignItems(ui.Center).Radius(kit.RCard).Border(1, t.Border).Background(a.pal.Side).Children(fn)
		})
	}
	// 查看讨论 shows the talk of session, unless it is in view.
	see := func(session string) {
		if !(a.proposalFrame.Visible && a.proposalFrame.Session == session) && a.pal.Ghost(c, locales.EditorSeeTalk(), ui.Color{}).Clicked() {
			a.openTalk(session)
		}
	}
	if w.rewriting {
		bar(func() {
			kit.Glyph(c, kit.IcPen, kit.IconMd, t.TextMuted)
			in := ui.TextInput(c, &w.instruction).Label(locales.EditorRewriteLabel()).Placeholder(locales.EditorRewriteHint()).Grow(1).Shrink(1).AutoFocus()
			s, e := w.cur().ed.Selection()
			can := s != e && strings.TrimSpace(w.instruction) != ""
			if (a.pal.Primary(c, locales.EditorRewrite()).Disabled(!can).Clicked() || in.Submitted()) && can {
				a.rewrite(c)
				w.cur().ed.Focus()
			}
			if kit.IconButton(c, kit.IcX, locales.EditorCloseRewrite(), ui.Color{}).Clicked() || in.Shortcut(0, ui.KeyEscape) {
				w.rewriting = false
				w.cur().ed.Focus()
			}
		})
		return
	}
	var here []*prop
	for _, p := range w.props {
		if p.Chapter == w.node {
			here = append(here, p)
		}
	}
	if len(here) == 0 {
		// Asked to draft or rewrite, the AI thinks and looks things up
		// before it proposes: that shows here too, to be let go.
		if !a.proposalFrame.Active || a.proposalFrame.Chapter == "" || a.proposalFrame.Chapter != w.node {
			return
		}
		say := locales.ToastThinking()
		if a.proposalFrame.Looking {
			say = locales.EditorLooking()
		}
		bar(func() {
			ui.Spinner(c)
			ui.Text(c, say).FontSize(kit.FsControl).TextColor(t.TextMuted).Grow(1).Shrink(1).SingleLine().Ellipsis("…")
			see(a.proposalFrame.Session)
			if a.pal.Soft(c, locales.ToastCancel()).Clicked() && a.cancel != nil {
				a.cancel()
				w.cur().ed.Focus()
			}
		})
		return
	}
	writing := slices.ContainsFunc(here, func(p *prop) bool { return p.writing })
	struck := slices.ContainsFunc(here, func(p *prop) bool { return p.At == workspace.ProposeReplace || p.At == workspace.ProposeAll })
	bar(func() {
		what, color := locales.EditorSuggestionsHere(len(here)), t.Success
		if writing {
			what, color = locales.EditorWritingSuggestions(), t.TextMuted
		}
		kit.Glyph(c, kit.IcSparkle, kit.IconMd, color)
		say := ui.Text(c, what).FontSize(kit.FsControl).TextColor(t.TextMuted).Grow(1).Shrink(1).SingleLine().Ellipsis("…")
		if !writing {
			how := locales.EditorHowAppend()
			if struck {
				how = locales.EditorHowReplace()
			}
			say.Tooltip(how)
		}
		see(here[len(here)-1].session)
		if a.pal.Soft(c, locales.EditorDropAll()).Clicked() {
			a.letGo(here)
			w.cur().ed.Focus()
		}
		// Grey while one comes, so that neither moves as it is done.
		if a.pal.Primary(c, locales.EditorAcceptAll()).Disabled(writing).Clicked() {
			a.takeProps(here)
		}
	})
}

// Leave asks, as the work is about to be left with proposals the author
// has not decided on, whether to let them go; it reports whether it asks.
// then is what leaves, done once they are let go.
func (a *App) Leave(then func()) bool {
	if a.Store == nil || len(a.write.props) == 0 {
		return false
	}
	a.write.leaving = then
	return true
}

// leaveView is the question Leave asks.
func (a *App) leaveView(c *ui.Context) {
	w := &a.write
	open := w.leaving != nil
	if !open {
		return
	}
	var then func()
	a.pal.Modal(c, &open, func() {
		ui.Column(c).Width(kit.DialogAsk).Padding(kit.DialogPad).Gap(16).Children(func() {
			a.pal.DialogHead(c, locales.DialogLeaveTitle(len(w.props)), locales.DialogLeaveAbout())
			ui.Row(c).FillWidth().Gap(8).Justify(ui.End).Children(func() {
				if a.pal.Secondary(c, locales.ToastCancel()).Clicked() {
					open = false
				}
				if a.pal.Primary(c, locales.DialogLeaveDiscard()).Clicked() {
					then, open = w.leaving, false
				}
			})
		})
	})
	if !open {
		w.leaving = nil
	}
	if then != nil {
		a.dropProps(w.props, workspace.FateClosed)
		then()
	}
}

func (a *App) conversationTitle(id string) (string, bool) {
	if a.talkAPI == nil {
		return "", false
	}
	return a.talkAPI.TalkTitle(id)
}
