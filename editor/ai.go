package editor

import (
	"slices"
	"time"
)

// proposal is a rewrite of [a, b) shown beside the words it would replace,
// struck through, until it is accepted or dropped. It lasts while the
// text stays as it was.
type proposal struct {
	a, b int
	text string
	ver  int
	on   bool
}

// draft is text an AI is writing into the document at end, after it took
// out what was in [at, end) when it began.
type draft struct {
	on                        bool
	at, end                   int
	removed                   []rune
	marksBefore               []Mark
	caretBefore, anchorBefore int
	readOnly                  bool
}

// SetReadOnly allows, or stops, changes to the text.
func (e *Editor) SetReadOnly(on bool) { e.opts.ReadOnly = on }

// ReadOnly reports whether the text is read-only.
func (e *Editor) ReadOnly() bool { return e.opts.ReadOnly }

// RequestPrediction asks the predictor for a prediction at the caret now,
// wherever the caret is, as a shortcut does when predictions come on
// demand.
func (e *Editor) RequestPrediction() {
	if e.opts.Predictor == nil || e.opts.ReadOnly {
		return
	}
	e.cancelPredict()
	e.ghost, e.asked, e.predictVer, e.compOpen = "", true, -1, false
}

// Propose shows text as a rewrite of [a, b), with the caret after it.
func (e *Editor) Propose(a, b int, text string) {
	n := e.Doc.Len()
	a, b = clamp(min(a, b), 0, n), clamp(max(a, b), 0, n)
	e.DismissGhost()
	e.prop = proposal{a: a, b: b, text: singleParagraphs(text), ver: e.Doc.version, on: true}
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

// AcceptProposal puts the rewrite shown in place of its words, as one
// step, marked as accepted AI text with note.
func (e *Editor) AcceptProposal(note string) bool {
	if !e.proposing() {
		return false
	}
	p := e.prop
	e.prop = proposal{}
	e.replaceRange(p.a, p.b, p.text, false)
	n := len([]rune(normalize(p.text)))
	if n > 0 {
		e.Doc.AddMark(MarkAIAccepted, p.a, p.a+n, note)
		e.undo[len(e.undo)-1].marksAfter = e.Doc.Marks()
	}
	e.compOpen = false
	return true
}

// DropProposal stops showing the rewrite.
func (e *Editor) DropProposal() { e.prop = proposal{} }

// BeginDraft takes out [a, b) for an AI to write in its place, keeping the
// text read-only until EndDraft.
func (e *Editor) BeginDraft(a, b int) {
	if e.dr.on {
		return
	}
	n := e.Doc.Len()
	a, b = clamp(min(a, b), 0, n), clamp(max(a, b), 0, n)
	e.DismissGhost()
	e.prop = proposal{}
	e.dr = draft{on: true, at: a, end: a, removed: slices.Clone(e.Doc.text[a:b]), marksBefore: e.Doc.Marks(),
		caretBefore: e.caret, anchorBefore: e.anchor, readOnly: e.opts.ReadOnly}
	if b > a {
		e.Doc.replace(a, b, nil)
	}
	e.opts.ReadOnly = true
	e.caret, e.anchor = a, a
	e.edited()
}

// DraftAppend adds s to the end of the text being drafted.
func (e *Editor) DraftAppend(s string) {
	if !e.dr.on {
		return
	}
	ins := []rune(normalize(s))
	if len(ins) == 0 {
		return
	}
	e.Doc.replace(e.dr.end, e.dr.end, ins)
	e.dr.end += len(ins)
	e.caret, e.anchor = e.dr.end, e.dr.end
	e.edited()
}

// EndDraft finishes the draft: what it wrote is marked as drafted with
// note, and the whole draft, with what it took out, undoes as one step.
func (e *Editor) EndDraft(note string) {
	d := e.dr
	if !d.on {
		return
	}
	e.dr = draft{}
	e.opts.ReadOnly = d.readOnly
	if d.end == d.at && len(d.removed) == 0 {
		return
	}
	if d.end > d.at {
		e.Doc.AddMark(MarkAIDraft, d.at, d.end, note)
	}
	e.undo = append(e.undo, undoStep{at: d.at, removed: d.removed, inserted: slices.Clone(e.Doc.text[d.at:d.end]),
		caretBefore: d.caretBefore, anchorBefore: d.anchorBefore, caretAfter: d.end,
		marksBefore: d.marksBefore, marksAfter: e.Doc.Marks(), when: time.Now()})
	e.redo = nil
	e.edited()
}

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
