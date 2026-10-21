package desk

import (
	"fmt"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"
	"writer/editor"
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
	if !a.talking || a.Store == nil {
		return
	}
	w := &a.write
	_, _, steps, _ := a.live.read()
	models := map[string]string{}
	for _, e := range steps {
		for _, call := range e.Calls {
			models[call.ID] = e.Model
		}
	}
	for _, s := range a.live.proposals() {
		a.takeProposal(s.call, s.p, true, "")
	}
	for _, e := range steps {
		switch {
		case e.Role != workspace.RoleResult:
		case e.Proposal != nil:
			a.takeProposal(e.Call, *e.Proposal, false, models[e.Call])
		default:
			if p := w.propOf(a.talkSession, e.Call); p != nil && p.writing {
				a.forget(p)
			}
		}
	}
}

// takeProposal shows what the call of the turn in flight proposes: as far as
// its text came, or whole. A new one takes the place of those undecided
// where it goes, and its chapter, when not open, opens in a tab beside
// the one shown.
func (a *App) takeProposal(call string, p workspace.Proposal, writing bool, model string) {
	w := &a.write
	session := a.talkSession
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
	if !a.talking {
		return
	}
	w := &a.write
	n := len(w.props)
	w.props = slices.DeleteFunc(w.props, func(p *prop) bool { return p.writing && p.session == a.talkSession })
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
		shown = shown || p.session == a.session
	}
	w.propVer++
	if shown {
		a.warn(a.refresh())
	}
}

// letGo is the author letting the proposals ps go: one still coming stops
// the reply that writes it.
func (a *App) letGo(ps []*prop) {
	if slices.ContainsFunc(ps, func(p *prop) bool { return p.writing }) && a.cancel != nil {
		a.cancel()
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
	text := w.ed.Doc.String()
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
	if err := a.Store.SnapshotForTalks(w.node, text, storedMarks(w.ed.Doc.Marks()), talks); err != nil {
		a.warn(err)
		return
	}
	w.ed.Take(places, list[0].model)
	a.warn(a.saveText())
	a.dropProps(list, workspace.FateTaken)
	a.notice = "已采纳 AI 提议；可撤销恢复原文，原版本已存入快照"
	w.ed.Focus()
}

// chapterText is the text of the chapter of id as the page has it: in the
// editor shown or kept for its tab, or else on the disk.
func (a *App) chapterText(id string) string {
	w := &a.write
	if id == w.node && w.ed != nil {
		return w.ed.Doc.String()
	}
	if s := w.parked[id]; s != nil {
		return s.ed.Doc.String()
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
	if w.ed == nil {
		return
	}
	if key := (placedKey{w.ed, w.ed.Doc.Version(), w.propVer}); key != w.placed {
		text := w.ed.Doc.String()
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
		w.ed.SetProposals(shown, propRoom)
		w.placed = placedKey{w.ed, w.ed.Doc.Version(), w.propVer}
		switch {
		case len(void) == 1 && late:
			a.hint = "AI 提议已作废；写的时候，它要换掉的原文改动了"
		case len(void) > 0:
			a.hint = fmt.Sprintf("%d 处 AI 提议已作废；它要换掉的原文改动了", len(void))
		}
		a.dropProps(void, workspace.FateVoid)
	}
	for _, p := range w.props {
		if p.Chapter == w.node && p.placed && !p.shown {
			p.shown = true
			w.ed.RevealProposal(p.id())
		}
	}
}

// pruneProps lets go of what was proposed in a chapter that is no longer
// in the book.
func (a *App) pruneProps() {
	w := &a.write
	if w.tree.Root == nil {
		return
	}
	var gone []*prop
	for _, p := range w.props {
		if _, ok := w.tree.Node(p.Chapter); !ok {
			gone = append(gone, p)
		}
	}
	a.dropProps(gone, workspace.FateVoid)
}

// showProp shows the chapter of p on the writing page, with p in view.
func (a *App) showProp(p *prop) {
	w := &a.write
	if !w.entered {
		w.entered = true
		a.enterWriting()
	}
	a.page = pageWrite
	if w.node != p.Chapter {
		a.openNode(p.Chapter)
	}
	p.shown = false
}

// openTalk shows the talk session beside the prose.
func (a *App) openTalk(session string) {
	d := &a.doc
	d.hidden, d.tab, d.talks = false, paneTalk, false
	a.write.focus = false
	a.savePrefs()
	a.switchSession(session)
}

// propActs float in the room after each proposal shown: 放弃, then 采纳
// once it came whole, or while it comes that it does. 放弃 stays where it
// is as it comes whole: a click meant for it then is not on 采纳.
func (a *App) propActs(c *ui.Context) {
	t := a.pal.theme
	w := &a.write
	var take, drop *prop
	for _, p := range w.props {
		if p.Chapter != w.node {
			continue
		}
		r, ok := w.ed.ProposalGap(p.id())
		if !ok {
			continue
		}
		ui.Row(c.Key("提议-" + p.id())).Absolute().Left(r.X + propInset).Top(r.Y).Height(r.H).Gap(6).AlignItems(ui.Center).Children(func() {
			tip := "放弃这一处提议"
			if p.writing {
				tip = "让 AI 停下，放弃这一处"
			}
			if a.pal.soft(c, "放弃").Tooltip(tip).KeepFocus().Clicked() {
				drop = p
			}
			if p.writing {
				ui.Text(c, "正在写…").FontSize(fsControl).TextColor(t.TextMuted).SingleLine()
			} else if a.pal.primary(c, "采纳").Tooltip("采纳这一处提议").KeepFocus().Clicked() {
				take = p
			}
		})
	}
	if drop != nil {
		a.letGo([]*prop{drop})
		w.ed.Focus()
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
	t := a.pal.theme
	w := &a.write
	bar := func(fn func()) {
		overProse(c, 10, 0, func() {
			ui.Row(c).FillWidth().Padding(6, 8, 6, 12).Gap(8).AlignItems(ui.Center).Radius(rCard).Border(1, t.Border).Background(a.pal.side).Children(fn)
		})
	}
	// 查看讨论 shows the talk of session, unless it is in view.
	see := func(session string) {
		if !(a.talkShown() && a.session == session) && a.pal.ghost(c, "查看讨论", ui.Color{}).Clicked() {
			a.openTalk(session)
		}
	}
	if w.rewriting {
		bar(func() {
			glyph(c, icPen, iconMd, t.TextMuted)
			in := ui.TextInput(c, &w.instruction).Label("改写要求").Placeholder("改写要求，例如：放慢节奏，多写她的犹豫").Grow(1).Shrink(1).AutoFocus()
			s, e := w.ed.Selection()
			can := s != e && strings.TrimSpace(w.instruction) != ""
			if (a.pal.primary(c, "改写").Disabled(!can).Clicked() || in.Submitted()) && can {
				a.rewrite(c)
				w.ed.Focus()
			}
			if iconButton(c, icX, "关闭改写", ui.Color{}).Clicked() || in.Shortcut(0, ui.KeyEscape) {
				w.rewriting = false
				w.ed.Focus()
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
		if !a.talking || a.talkChapter == "" || a.talkChapter != w.node {
			return
		}
		_, _, steps, _ := a.live.read()
		say := "AI 正在思考…"
		if len(looks(steps, "")) > 0 {
			say = "AI 正在查阅…"
		}
		bar(func() {
			ui.Spinner(c)
			ui.Text(c, say).FontSize(fsControl).TextColor(t.TextMuted).Grow(1).Shrink(1).SingleLine().Ellipsis("…")
			see(a.talkSession)
			if a.pal.soft(c, "取消").Clicked() && a.cancel != nil {
				a.cancel()
				w.ed.Focus()
			}
		})
		return
	}
	writing := slices.ContainsFunc(here, func(p *prop) bool { return p.writing })
	struck := slices.ContainsFunc(here, func(p *prop) bool { return p.At == workspace.ProposeReplace || p.At == workspace.ProposeAll })
	bar(func() {
		what, color := fmt.Sprintf("本章有 %d 处 AI 提议", len(here)), t.Success
		if writing {
			what, color = "AI 正在写提议…", t.TextMuted
		}
		glyph(c, icSparkle, iconMd, color)
		say := ui.Text(c, what).FontSize(fsControl).TextColor(t.TextMuted).Grow(1).Shrink(1).SingleLine().Ellipsis("…")
		if !writing {
			how := "绿字是 AI 新写的"
			if struck {
				how += "，划掉的是会被换掉的原文"
			}
			say.Tooltip(how + "；采纳后才写入正文")
		}
		see(here[len(here)-1].session)
		if a.pal.soft(c, "全部放弃").Clicked() {
			a.letGo(here)
			w.ed.Focus()
		}
		// Grey while one comes, so that neither moves as it is done.
		if a.pal.primary(c, "全部采纳").Disabled(writing).Clicked() {
			a.takeProps(here)
		}
	})
}

// fateNames say what came of a proposal, in the card of its talk.
var fateNames = map[string]string{
	workspace.FateTaken:    "已采纳",
	workspace.FateDropped:  "已放弃",
	workspace.FateReplaced: "已被新的提议替换",
	workspace.FateVoid:     "正文已改动，提议已作废",
	workspace.FateClosed:   "作品关闭时已丢弃",
	workspace.FateOff:      "关闭 AI 时已丢弃",
}

// whereNames say where a proposal goes, in its card.
var whereNames = map[string]string{
	workspace.ProposeReplace: "换掉一句",
	workspace.ProposeAfter:   "接在一句之后",
	workspace.ProposeEnd:     "接在章末",
	workspace.ProposeAll:     "换掉全章",
}

// fateOf is what came of the proposal of call, as events from from on
// tell, or "" while it is undecided.
func fateOf(events []workspace.Event, from int, call string) string {
	for _, e := range events[from:] {
		if e.Role == workspace.RoleFate && e.Call == call {
			return e.Fate
		}
	}
	return ""
}

// propCard shows, in the reply of the talk session, a change it proposed
// in the prose, by the result e of the call that proposed it.
func (a *App) propCard(c *ui.Context, session string, e workspace.Event) {
	from := slices.IndexFunc(a.events, func(u workspace.Event) bool { return u.ID == e.ReplyTo })
	a.propCardOf(c, *e.Proposal, a.write.propOf(session, e.Call), fateOf(a.events, max(from, 0), e.Call))
}

// turnCards show what the turn in flight proposed: as it comes, and as
// it came whole, in the order it began.
func (a *App) turnCards(c *ui.Context, steps []workspace.Event) {
	w := &a.write
	var calls []string
	ps := map[string]workspace.Proposal{}
	for _, s := range a.live.proposals() {
		calls, ps[s.call] = append(calls, s.call), s.p
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
		a.propCardOf(c, ps[call], w.propOf(a.talkSession, call), fateOf(a.events, 0, call))
	}
}

// writingProps tells whether the turn in flight is writing a proposal.
func (a *App) writingProps() bool {
	return slices.ContainsFunc(a.write.props, func(p *prop) bool { return p.writing && p.session == a.talkSession })
}

// propCardOf shows the proposal p: which chapter, where in it, how long,
// and what came of it, by fate or by live, the proposal while undecided.
// Undecided, it links to where it shows.
func (a *App) propCardOf(c *ui.Context, p workspace.Proposal, live *prop, fate string) {
	t := a.pal.theme
	state := fateNames[fate]
	switch {
	case state != "":
	case live != nil && live.writing:
		state = "正在写…"
	case live != nil:
		state = "等你决定"
	default:
		state = "已丢弃"
	}
	title := a.nodeTitle(p.Chapter)
	if title == "" {
		title = p.Title
	}
	ui.Column(c).FillWidth().Gap(2).Padding(8, 10).Radius(rCard).Border(1, a.pal.line).Background(a.pal.side).Children(func() {
		ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
			color := t.TextMuted
			if live != nil {
				color = t.Success
			}
			glyph(c, icPen, iconSm, color)
			ui.Text(c, "提议修改「"+title+"」").FontSize(fsControl).SingleLine().Ellipsis("…").Grow(1).Shrink(1)
			if live != nil && a.pal.ghost(c, "在正文中查看", ui.Color{}).Clicked() {
				a.showProp(live)
			}
		})
		ui.Textf(c, "%s · %d 字 · %s", whereNames[p.At], workspace.Count(p.Text), state).FontSize(fsNote).TextColor(t.TextMuted)
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
	a.modal(c, &open, func() {
		ui.Column(c).Width(dialogAsk).Padding(dialogPad).Gap(16).Children(func() {
			a.pal.dialogHead(c, fmt.Sprintf("还有 %d 处 AI 提议没有处理", len(w.props)), "关闭后将丢弃。")
			ui.Row(c).FillWidth().Gap(8).Justify(ui.End).Children(func() {
				if a.pal.secondary(c, "取消").Clicked() {
					open = false
				}
				if a.pal.primary(c, "丢弃并关闭").Clicked() {
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
