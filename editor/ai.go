package editor

import (
	"slices"

	"github.com/egoist/mygo/ui"
)

// Proposal is a change of the text an AI proposes, shown in the text until
// the host takes it or lets it go: Text after the words [A, B), which it
// replaces, struck through, or with Keep follows. Apart shows Text below
// those words, as paragraphs of its own, though taken it goes in where
// they were. ID names it to the host.
type Proposal struct {
	ID    string
	A, B  int
	Keep  bool
	Apart bool
	Text  string
}

// gapRune holds the room after what is proposed, for what the host shows
// there: as wide as the gap, and drawn as nothing.
const gapRune = '￼'

func (e *Editor) ReadOnly() bool { return e.opts.ReadOnly }

// SetProposals shows ps in place of those shown, each with gap DIPs of
// room after its text. One whose text grew while its end was in view keeps
// its end in view as it grows.
func (e *Editor) SetProposals(ps []Proposal, gap float32) {
	ps = slices.Clone(ps)
	for i := range ps {
		p := &ps[i]
		*p = e.normalizeProposal(*p)
	}
	if gap != e.gap {
		e.gap, e.shapes = gap, nil // shaped with the old room
		e.propVer++
	}
	if slices.Equal(ps, e.props) {
		return
	}
	for _, p := range ps {
		if old := slices.IndexFunc(e.props, func(q Proposal) bool { return q.ID == p.ID }); old >= 0 && len(p.Text) > len(e.props[old].Text) {
			if _, seen := e.ProposalGap(p.ID); seen {
				e.follow = p.ID
			}
		}
	}
	e.props = ps
	e.propVer++
}

func (e *Editor) Proposals() []Proposal { return slices.Clone(e.props) }

// ProposalGap returns where the room after proposal id's text lies,
// relative to the element. ok is false before the editor was painted, for
// a proposal not shown, and while the room is not wholly in view.
func (e *Editor) ProposalGap(id string) (r ui.Rect, ok bool) {
	l, ready := e.geometry()
	if !ready {
		return ui.Rect{}, false
	}
	d, found := l.gaps[id]
	if !found {
		return ui.Rect{}, false
	}
	ln := &l.lines[l.lineOf(d)]
	y := ln.y - e.scrollY
	if y < 0 || y+l.lineH > e.height {
		return ui.Rect{}, false
	}
	return ui.Rect{X: l.left + l.x(ln, d), Y: y, W: e.gap, H: l.lineH}, true
}

// RevealProposal scrolls the start of proposal id's text into view.
func (e *Editor) RevealProposal(id string) { e.show = id }

// Take puts the proposals ps in place of their words, as one step to undo,
// their words marked as an AI's with note, and puts the caret after the
// last of them. Proposals whose words overlap one before them are left
// out.
func (e *Editor) Take(ps []Proposal, note string) {
	ps = slices.Clone(ps)
	slices.SortStableFunc(ps, func(p, q Proposal) int { return p.A - q.A })
	var took []Proposal
	end := -1
	for _, p := range ps {
		p = e.normalizeProposal(p)
		if p.Keep {
			p.A = p.B // what it follows stays
		}
		if p.A < end {
			continue
		}
		took = append(took, p)
		end = p.B
	}
	if len(took) == 0 {
		return
	}
	e.DismissGhost()
	steps := len(e.undo)
	for i := len(took) - 1; i >= 0; i-- {
		p := took[i]
		e.replaceRange(p.A, p.B, p.Text, false)
		if len(e.undo) > steps+1 {
			e.undo[len(e.undo)-1].joined = true
		}
	}
	if len(e.undo) == steps {
		return
	}
	shift, caret := 0, 0
	for _, p := range took {
		at := p.A + shift
		from, to := words(normalize(p.Text))
		if to > from {
			e.Doc.Add(Mark{Kind: MarkAI, Start: at + from, End: at + to, Note: note})
		}
		caret = at + to
		shift += len([]rune(normalize(p.Text))) - (p.B - p.A)
	}
	// Undone, the caret is where the first went in, not wherever it was
	// before; redone, it is after the last, as now.
	e.undo[steps].caretBefore, e.undo[steps].anchorBefore = took[0].A, took[0].A
	e.undo[len(e.undo)-1].marksAfter, e.undo[len(e.undo)-1].caretAfter = e.Doc.Marks(), caret
	e.caret, e.anchor, e.reveal = caret, caret, true
	e.compOpen = false
}

// words is where the words of s begin and end, in runes: the line breaks
// it begins or ends with are no words of it, and marked they would show at
// the end of the paragraph before, or after its own.
func words(s string) (from, to int) {
	r := []rune(s)
	from, to = 0, len(r)
	for from < to && r[from] == '\n' {
		from++
	}
	for to > from && r[to-1] == '\n' {
		to--
	}
	return from, to
}

// singleParagraphs keeps the paragraphs of s, without blank lines between
// or after, keeping an initial line break to start a paragraph of its own.
func singleParagraphs(s string) string {
	var out []rune
	r := []rune(normalize(s))
	if len(r) > 0 && r[0] == '\n' {
		out = append(out, '\n')
	}
	for _, c := range r {
		if c == '\n' && (len(out) == 0 || out[len(out)-1] == '\n') {
			continue
		}
		out = append(out, c)
	}
	for len(out) > 1 && out[len(out)-1] == '\n' {
		out = out[:len(out)-1]
	}
	return string(out)
}

// SetPredictOnDemand makes predictions come only when asked, or also as
// typing pauses.
func (e *Editor) SetPredictOnDemand(on bool) { e.opts.PredictOnDemand = on }

// SetNoAI turns the AI off in the editor, dropping the prediction shown
// or asked for, or back on.
func (e *Editor) SetNoAI(on bool) {
	e.opts.NoAI = on
	if on {
		e.DismissGhost()
		e.predict.asked = false
	}
}

// hidden tells whether m is not shown: what an AI wrote, while it is off.
func (e *Editor) hidden(m Mark) bool {
	return e.opts.NoAI && m.Kind == MarkAI
}

func (e *Editor) normalizeProposal(p Proposal) Proposal {
	p.A, p.B = e.Doc.span(p.A, p.B)
	p.Text = singleParagraphs(p.Text)
	return p
}
