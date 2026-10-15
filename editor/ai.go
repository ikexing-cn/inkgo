package editor

// proposal is what an AI wrote in place of [a, b), a rewrite or a draft,
// shown after the words it would replace, which are struck through, until
// it is accepted or dropped. It lasts while the text stays as it was.
type proposal struct {
	a, b int
	text string
	kind MarkKind // the mark its words take when accepted
	ver  int
	on   bool
}

// SetReadOnly allows, or stops, changes to the text.
func (e *Editor) SetReadOnly(on bool) { e.opts.ReadOnly = on }

// ReadOnly reports whether the text is read-only.
func (e *Editor) ReadOnly() bool { return e.opts.ReadOnly }

// RequestPrediction asks the predictor for a prediction at the caret now,
// wherever the caret is, as a shortcut does when predictions come on
// demand.
func (e *Editor) RequestPrediction() {
	if e.opts.Predictor == nil || e.opts.ReadOnly || e.opts.NoAI {
		return
	}
	e.cancelPredict()
	e.ghost, e.asked, e.predictVer, e.compOpen = "", true, -1, false
}

// Propose shows text as a rewrite of [a, b), with the caret after it.
func (e *Editor) Propose(a, b int, text string) {
	e.propose(a, b, singleParagraphs(text), MarkAIAccepted)
}

// ProposeDraft shows a draft to take the place of [a, b), empty until
// ProposeMore adds what comes of it.
func (e *Editor) ProposeDraft(a, b int) { e.propose(a, b, "", MarkAIDraft) }

// ProposeMore adds s to the end of the draft shown, and keeps that end in
// view.
func (e *Editor) ProposeMore(s string) {
	if !e.proposing() || e.prop.kind != MarkAIDraft {
		return
	}
	e.prop.text += normalize(s)
	e.reveal = true
}

func (e *Editor) propose(a, b int, text string, kind MarkKind) {
	n := e.Doc.Len()
	a, b = clamp(min(a, b), 0, n), clamp(max(a, b), 0, n)
	e.DismissGhost()
	e.prop = proposal{a: a, b: b, text: text, kind: kind, ver: e.Doc.version, on: true}
	e.caret, e.anchor, e.reveal = b, b, true
}

// Proposal returns the rewrite shown, if one is.
func (e *Editor) Proposal() (a, b int, text string, ok bool) {
	if !e.proposing() {
		return 0, 0, "", false
	}
	return e.prop.a, e.prop.b, e.prop.text, true
}

func (e *Editor) proposing() bool { return e.prop.on && e.prop.ver == e.Doc.version }

// AcceptProposal puts what is proposed in place of its words, as one
// step, marked with note as accepted AI text, or as drafted.
func (e *Editor) AcceptProposal(note string) bool {
	if !e.proposing() {
		return false
	}
	p := e.prop
	e.prop = proposal{}
	e.replaceRange(p.a, p.b, p.text, false)
	n := len([]rune(normalize(p.text)))
	if n > 0 {
		e.Doc.AddMark(p.kind, p.a, p.a+n, note)
		e.undo[len(e.undo)-1].marksAfter = e.Doc.Marks()
	}
	e.compOpen = false
	return true
}

// DropProposal stops showing what is proposed.
func (e *Editor) DropProposal() { e.prop = proposal{} }

// singleParagraphs keeps the paragraphs of s, without blank lines between.
func singleParagraphs(s string) string {
	var out []rune
	for _, r := range normalize(s) {
		if r == '\n' && (len(out) == 0 || out[len(out)-1] == '\n') {
			continue
		}
		out = append(out, r)
	}
	for len(out) > 0 && out[len(out)-1] == '\n' {
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
		e.asked = false
	}
}

// hidden tells whether m is not shown: what an AI wrote, while it is off.
func (e *Editor) hidden(m Mark) bool {
	return e.opts.NoAI && (m.Kind == MarkAIAccepted || m.Kind == MarkAIDraft)
}
