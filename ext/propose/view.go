package propose

import (
	"slices"
	"strings"
	"writer/ext/propose/proposetext"

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

// Proposal is a change an AI proposed in a talk that the author has not
// decided on.
type Proposal struct {
	workspace.Proposal
	session, call string
	model         string // that proposed it, which the words taken are marked with
	base          string // the chapter's text as it was proposed, which one of all replaces
	near          int    // where its words were last found in the chapter
	// Writing reports that the tool input is still streaming.
	Writing bool
	fresh   bool // came whole since it was last placed
	placed  bool // shows in the editor of its chapter
	shown   bool // was brought into view
}

// ID names p to the editor.
func (p *Proposal) ID() string { return p.session + "/" + p.call }

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
	ed            *editor.Editor
	ver, Proposal int
}

// propOf is the proposal the call of the talk session made, if it is
// still undecided.
func (w *View) propOf(session, call string) *Proposal {
	for _, p := range w.Props {
		if p.session == session && p.call == call {
			return p
		}
	}
	return nil
}

// followProps takes in what the reply in flight proposes in the prose, as
// its text comes and once it came whole; one whose call failed goes, the
// AI being told why.
func (a *View) followProps() {
	f := a.Frame
	if !f.Active || a.book.Get().Store() == nil {
		return
	}
	for _, u := range f.Updates {
		a.TakeProposal(u.Call, u.Proposal, u.Writing, u.Model)
	}
	for _, call := range f.Failed {
		if p := a.propOf(f.Session, call); p != nil && p.Writing {
			a.forget(p)
		}
	}
}

// TakeProposal shows what the call of the turn in flight proposes: as far as
// its text came, or whole. A new one takes the place of those undecided
// where it goes, and its chapter, when not open, opens in a tab beside
// the one shown.
func (a *View) TakeProposal(call string, p workspace.Proposal, writing bool, model string) {
	w := a
	session := a.Frame.Session
	key := session + "/" + call
	if w.seen[key] {
		pr := w.propOf(session, call)
		switch {
		case pr == nil || !pr.Writing: // decided, or whole
		case writing:
			if pr.Text != p.Text {
				pr.Text = p.Text
				w.propVer++
			}
		default:
			pr.Proposal, pr.Writing, pr.fresh, pr.model = p, false, true, model
			w.propVer++
		}
		return
	}
	if w.seen == nil {
		w.seen = map[string]bool{}
	}
	w.seen[key] = true
	pr := &Proposal{Proposal: p, session: session, call: call, model: model, Writing: writing, base: a.chapterText(p.Chapter)}
	var over []*Proposal
	if at, ok := pr.forEditor(pr.base); ok {
		for _, q := range w.Props {
			if q.Chapter != p.Chapter {
				continue
			}
			if qat, ok := q.forEditor(pr.base); ok && samePlace(at, qat, p.At == workspace.ProposeAll || q.At == workspace.ProposeAll) {
				over = append(over, q)
			}
		}
	}
	a.dropProps(over, workspace.FateReplaced)
	w.Props = append(w.Props, pr)
	w.propVer++
	if a.node() != p.Chapter {
		a.surface.Get().EnsureTab(p.Chapter)
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
func (a *View) unfinishedProps() {
	if !a.Frame.Active {
		return
	}
	w := a
	n := len(w.Props)
	w.Props = slices.DeleteFunc(w.Props, func(p *Proposal) bool { return p.Writing && p.session == a.Frame.Session })
	if len(w.Props) != n {
		w.propVer++
	}
}

// forget lets p go with nothing to tell of it.
func (a *View) forget(p *Proposal) {
	w := a
	w.Props = slices.DeleteFunc(w.Props, func(q *Proposal) bool { return q == p })
	w.propVer++
}

// dropProps lets the proposals ps go, writing in their talks what came of
// them.
func (a *View) dropProps(ps []*Proposal, fate string) {
	if len(ps) == 0 {
		return
	}
	w := a
	ps = slices.Clone(ps)
	shown := false
	for _, p := range ps {
		w.Props = slices.DeleteFunc(w.Props, func(q *Proposal) bool { return q == p })
		_, err := a.book.Get().Store().Append(p.session, workspace.Event{Role: workspace.RoleFate, Call: p.call, Fate: fate})
		a.warn(err)
		shown = shown || true
	}
	w.propVer++
	if shown {
		a.warn(a.surface.Get().Refresh())
	}
}

// letGo is the author letting the proposals ps go: one still coming stops
// the reply that writes it.
func (a *View) letGo(ps []*Proposal) {
	if slices.ContainsFunc(ps, func(p *Proposal) bool { return p.Writing }) && a.Frame.Stop != nil {
		a.Frame.Stop()
	}
	a.dropProps(ps, workspace.FateDropped)
}

// TakeProps puts the proposals ps of the chapter shown in its text, as one
// step to undo, after a snapshot of it as it was that names the talks they
// came from; their words are marked
// as the AI's, and the caret goes after the last. One still coming is
// left.
func (a *View) TakeProps(ps []*Proposal) {
	text := a.ed().Doc.String()
	var list []*Proposal
	var places []editor.Proposal
	var talks []string
	for _, p := range ps {
		if p.Chapter != a.node() || p.Writing {
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
	if err := a.book.Get().Store().SnapshotForTalks(a.node(), text, storedMarks(a.ed().Doc.Marks()), talks); err != nil {
		a.warn(err)
		return
	}
	a.ed().Take(places, list[0].model)
	a.warn(a.book.Get().SaveText())
	a.dropProps(list, workspace.FateTaken)
	a.notices.Get().Notices().Did(proposetext.EditorProposalAccepted())
	a.ed().Focus()
}

// chapterText is the text of the chapter of ID as the page has it: in
// its tab's editor, or else on the disk.
func (a *View) chapterText(ID string) string { return a.surface.Get().ChapterText(ID) }

func (p *Proposal) forEditor(text string) (editor.Proposal, bool) {
	at, near, ok := p.Proposal.Place(text, p.base, p.near)
	p.near = near
	return editor.Proposal{ID: p.ID(), A: at.A, B: at.B, Keep: at.Keep, Apart: at.Apart, Text: at.Text}, ok
}

// placeProps shows the proposals of the chapter shown where they go in its
// text now. One whose words the author changed is void, once it came
// whole. One shown for the first time, or asked for, comes into view.
func (a *View) placeProps() {
	w := a
	if a.ed() == nil {
		return
	}
	if key := (placedKey{a.ed(), a.ed().Doc.Version(), w.propVer}); key != w.placed {
		text := a.ed().Doc.String()
		var shown []editor.Proposal
		var void []*Proposal
		late := false
		for _, p := range w.Props {
			if p.Chapter != a.node() {
				continue
			}
			at, ok := p.forEditor(text)
			p.placed = ok
			switch {
			case ok:
				shown = append(shown, at)
				p.fresh = false
			case !p.Writing:
				void = append(void, p)
				late = late || p.fresh
			}
		}
		a.ed().SetProposals(shown, propRoom)
		w.placed = placedKey{a.ed(), a.ed().Doc.Version(), w.propVer}
		switch {
		case len(void) == 1 && late:
			a.notices.Get().Notices().Hint(proposetext.EditorProposalVoid())
		case len(void) > 0:
			a.notices.Get().Notices().Hint(proposetext.EditorProposalsVoid(len(void)))
		}
		a.dropProps(void, workspace.FateVoid)
	}
	for _, p := range w.Props {
		if p.Chapter == a.node() && p.placed && !p.shown {
			p.shown = true
			a.ed().RevealProposal(p.ID())
		}
	}
}

// pruneProps lets go of what was proposed in a chapter that is no longer
// in the book.
func (a *View) pruneProps() {
	w := a
	if a.book.Get().Content().Tree().Root == nil {
		return
	}
	var gone []*Proposal
	for _, p := range w.Props {
		if _, ok := a.book.Get().Content().Tree().Node(p.Chapter); !ok {
			gone = append(gone, p)
		}
	}
	a.dropProps(gone, workspace.FateVoid)
}

// showProp shows the chapter of p on the writing page, with p in view.
func (a *View) showProp(p *Proposal) {

	if a.ed() == nil || a.node() != p.Chapter {
		a.desktop.Get().OpenWriting(p.Chapter)
	}
	p.shown = false
}

func (a *View) openTalk(ID string) {
	for _, i := range a.starterItems() {
		_ = a.starters.Guard(i.Owner, i.ID, func() { i.Value.Open(ID) })
	}
}

// propActs float in the room after each proposal shown: 放弃, then 采纳
// once it came whole, or while it comes that it does. 放弃 stays where it
// is as it comes whole: a click meant for it then is not on 采纳.
func (a *View) propActs(c *ui.Context) {
	t := a.palette.Get().Colors().Theme
	w := a
	var take, drop *Proposal
	for _, p := range w.Props {
		if p.Chapter != a.node() {
			continue
		}
		r, ok := a.ed().ProposalGap(p.ID())
		if !ok {
			continue
		}
		ui.Row(c.Key("提议-" + p.ID())).Absolute().Left(r.X + propInset).Top(r.Y).Height(r.H).Gap(6).AlignItems(ui.Center).Children(func() {
			tip := proposetext.EditorDropOne()
			if p.Writing {
				tip = proposetext.EditorStopDrop()
			}
			if a.palette.Get().Colors().Soft(c, proposetext.CommonDiscard()).Tooltip(tip).KeepFocus().Clicked() {
				drop = p
			}
			if p.Writing {
				ui.Text(c, locales.EditorWriting()).FontSize(kit.FsControl).TextColor(t.TextMuted).SingleLine()
			} else if a.palette.Get().Colors().Primary(c, locales.EditorAccept()).Tooltip(proposetext.EditorAcceptOne()).KeepFocus().Clicked() {
				take = p
			}
		})
	}
	if drop != nil {
		a.letGo([]*Proposal{drop})
		a.ed().Focus()
	}
	if take != nil {
		a.TakeProps([]*Proposal{take})
	}
}

// aiBar is the line under the chapter's title for what the AI does in the
// chapter: the Rewrite it is asked for, a talk opened on it while the AI
// thinks, or what it proposed there, to let go or take all at once. Its
// buttons give the keyboard back to the prose: taken, an undo takes it
// back.
func (a *View) aiBar(c *ui.Context) {
	t := a.palette.Get().Colors().Theme
	w := a
	bar := func(fn func()) {
		a.surface.Get().Band(c, 10, 0, func() {
			ui.Row(c).FillWidth().Padding(6, 8, 6, 12).Gap(8).AlignItems(ui.Center).Radius(kit.RCard).Border(1, t.Border).Background(a.palette.Get().Colors().Side).Children(fn)
		})
	}
	// 查看讨论 shows the talk of session, unless it is in view.
	see := func(session string) {
		if !(a.Frame.Visible && a.Frame.Session == session) && a.palette.Get().Colors().Ghost(c, proposetext.EditorSeeTalk(), ui.Color{}).Clicked() {
			a.openTalk(session)
		}
	}
	if w.Rewriting {
		bar(func() {
			kit.Glyph(c, kit.IcPen, kit.IconMd, t.TextMuted)
			in := ui.TextInput(c, &w.Instruction).Label(proposetext.EditorRewriteLabel()).Placeholder(proposetext.EditorRewriteHint()).Grow(1).Shrink(1).AutoFocus()
			s, e := a.ed().Selection()
			can := s != e && strings.TrimSpace(w.Instruction) != ""
			if (a.palette.Get().Colors().Primary(c, proposetext.EditorRewrite()).Disabled(!can).Clicked() || in.Submitted()) && can {
				a.Rewrite(c)
				a.ed().Focus()
			}
			if kit.IconButton(c, kit.IcX, proposetext.EditorCloseRewrite(), ui.Color{}).Clicked() || in.Shortcut(0, ui.KeyEscape) {
				w.Rewriting = false
				a.ed().Focus()
			}
		})
		return
	}
	var here []*Proposal
	for _, p := range w.Props {
		if p.Chapter == a.node() {
			here = append(here, p)
		}
	}
	if len(here) == 0 {
		// Asked to draft or Rewrite, the AI thinks and looks things up
		// before it proposes: that shows here too, to be let go.
		if !a.Frame.Active || a.Frame.Chapter == "" || a.Frame.Chapter != a.node() {
			return
		}
		say := locales.ToastThinking()
		if a.Frame.Looking {
			say = proposetext.EditorLooking()
		}
		bar(func() {
			ui.Spinner(c)
			ui.Text(c, say).FontSize(kit.FsControl).TextColor(t.TextMuted).Grow(1).Shrink(1).SingleLine().Ellipsis("…")
			see(a.Frame.Session)
			if a.palette.Get().Colors().Soft(c, locales.ToastCancel()).Clicked() && a.Frame.Stop != nil {
				a.Frame.Stop()
				a.ed().Focus()
			}
		})
		return
	}
	writing := slices.ContainsFunc(here, func(p *Proposal) bool { return p.Writing })
	struck := slices.ContainsFunc(here, func(p *Proposal) bool { return p.At == workspace.ProposeReplace || p.At == workspace.ProposeAll })
	bar(func() {
		what, color := proposetext.EditorSuggestionsHere(len(here)), t.Success
		if writing {
			what, color = proposetext.EditorWritingSuggestions(), t.TextMuted
		}
		kit.Glyph(c, kit.IcSparkle, kit.IconMd, color)
		say := ui.Text(c, what).FontSize(kit.FsControl).TextColor(t.TextMuted).Grow(1).Shrink(1).SingleLine().Ellipsis("…")
		if !writing {
			how := proposetext.EditorHowAppend()
			if struck {
				how = proposetext.EditorHowReplace()
			}
			say.Tooltip(how)
		}
		see(here[len(here)-1].session)
		if a.palette.Get().Colors().Soft(c, proposetext.EditorDropAll()).Clicked() {
			a.letGo(here)
			a.ed().Focus()
		}
		// Grey while one comes, so that neither moves as it is done.
		if a.palette.Get().Colors().Primary(c, proposetext.EditorAcceptAll()).Disabled(writing).Clicked() {
			a.TakeProps(here)
		}
	})
}

// Leave asks, as the work is about to be left with proposals the author
// has not decided on, whether to let them go; it reports whether it asks.
// then is what leaves, done once they are let go.
func (a *View) Leave(then func()) bool {
	if a.book.Get().Store() == nil || len(a.Props) == 0 {
		return false
	}
	a.Leaving = then
	return true
}

// leaveView is the question Leave asks.
func (a *View) leaveView(c *ui.Context) {
	w := a
	open := w.Leaving != nil
	if !open {
		return
	}
	var then func()
	a.palette.Get().Colors().Modal(c, &open, func() {
		ui.Column(c).Width(kit.DialogAsk).Padding(kit.DialogPad).Gap(16).Children(func() {
			a.palette.Get().Colors().DialogHead(c, proposetext.DialogLeaveTitle(len(w.Props)), proposetext.DialogLeaveAbout())
			ui.Row(c).FillWidth().Gap(8).Justify(ui.End).Children(func() {
				if a.palette.Get().Colors().Secondary(c, locales.ToastCancel()).Clicked() {
					open = false
				}
				if a.palette.Get().Colors().Primary(c, proposetext.DialogLeaveDiscard()).Clicked() {
					then, open = w.Leaving, false
				}
			})
		})
	})
	if !open {
		w.Leaving = nil
	}
	if then != nil {
		a.dropProps(w.Props, workspace.FateClosed)
		then()
	}
}

// OpenRewrite asks how to Rewrite the words selected.
func (a *View) OpenRewrite() {
	w := a
	if s, e := a.ed().Selection(); s == e {
		return
	}
	a.surface.Get().CloseSelectionTools()
	w.Rewriting, w.Instruction = true, ""
}

// Rewrite opens a talk to Rewrite the words selected as the author asked:
// its first words quote them. The AI proposes the change in the prose.
func (a *View) Rewrite(c *ui.Context) {
	w := a
	s, e := a.ed().Selection()
	wish, quote := strings.TrimSpace(w.Instruction), strings.TrimSpace(a.ed().Doc.Slice(s, e))
	if wish == "" || quote == "" {
		return
	}
	q := workspace.Quote{Chapter: a.node(), Title: a.title(), Text: quote}
	if a.beginTalk(c, workspace.Event{Origin: workspace.AIRewrite, Text: wish, Quotes: []workspace.Quote{q}}) {
		w.Rewriting, w.Instruction = false, ""
		a.ed().MoveTo(e, false) // the words went to the talk: the caret after them
	}
}
