package editor

import (
	"slices"
	"time"
)

// typingMerge is how close keystrokes must come to undo together.
const typingMerge = time.Second

// replaceRange replaces [a, b) with s as one undoable step and puts the
// caret after it. typing merges the step into the last one while typing.
func (e *Editor) replaceRange(a, b int, s string, typing bool) {
	n := e.Doc.Len()
	a, b = clamp(min(a, b), 0, n), clamp(max(a, b), 0, n)
	ins := []rune(normalize(s))
	if a == b && len(ins) == 0 {
		return
	}
	now := time.Now()
	marksBefore := e.Doc.Marks()
	removed := slices.Clone(e.Doc.text[a:b])
	e.Doc.replace(a, b, ins)
	step := undoStep{
		at: a, removed: removed, inserted: ins,
		caretBefore: e.caret, anchorBefore: e.anchor, caretAfter: a + len(ins),
		marksBefore: marksBefore, marksAfter: e.Doc.Marks(), typing: typing, when: now,
	}
	if last := len(e.undo) - 1; typing && last >= 0 && e.undo[last].typing &&
		len(removed) == 0 && len(e.undo[last].removed) == 0 &&
		e.undo[last].caretAfter == a && now.Sub(e.undo[last].when) < typingMerge {
		u := &e.undo[last]
		u.inserted = append(u.inserted, ins...)
		u.caretAfter, u.marksAfter, u.when = step.caretAfter, step.marksAfter, now
	} else {
		e.undo = append(e.undo, step)
	}
	e.redo = nil
	e.caret, e.anchor = step.caretAfter, step.caretAfter
	e.edited()
}

// edited follows a change of the text.
func (e *Editor) edited() {
	e.hasGoal, e.reveal = false, true
	e.lastEdit = time.Now()
	e.ghost = ""
	e.cancelPredict()
}

// Insert types s at the caret, replacing the selection.
func (e *Editor) Insert(s string) {
	a, b := e.Selection()
	e.replaceRange(a, b, s, true)
	e.updateCompletion()
}

// Undo and Redo take back the last edit, and put it back; steps joined
// go together.
func (e *Editor) Undo() {
	for len(e.undo) > 0 {
		u := e.undo[len(e.undo)-1]
		e.undo = e.undo[:len(e.undo)-1]
		e.Doc.replace(u.at, u.at+len(u.inserted), u.removed)
		e.Doc.setMarks(u.marksBefore)
		e.redo = append(e.redo, u)
		e.caret, e.anchor = u.caretBefore, u.anchorBefore
		if !u.joined {
			break
		}
	}
	e.clampCaret()
	e.edited()
	e.compOpen = false
}

func (e *Editor) Redo() {
	for first := true; len(e.redo) > 0; first = false {
		u := e.redo[len(e.redo)-1]
		if !first && !u.joined {
			break
		}
		e.redo = e.redo[:len(e.redo)-1]
		e.Doc.replace(u.at, u.at+len(u.removed), u.inserted)
		e.Doc.setMarks(u.marksAfter)
		u.typing = false
		e.undo = append(e.undo, u)
		e.caret, e.anchor = u.caretAfter, u.caretAfter
	}
	e.clampCaret()
	e.edited()
	e.compOpen = false
}

// CanUndo and CanRedo report whether there is a step to take back, or to
// put back: a menu pales the row otherwise.
func (e *Editor) CanUndo() bool { return len(e.undo) > 0 }

func (e *Editor) CanRedo() bool { return len(e.redo) > 0 }

// Replace replaces runes [a, b) with s as one undoable step.
func (e *Editor) Replace(a, b int, s string) {
	e.replaceRange(a, b, s, false)
	e.compOpen = false
}

// ReplaceAll replaces every q with s as one undoable step, marks moving
// with the text around each, and returns how many it replaced.
func (e *Editor) ReplaceAll(q, s string) int {
	found := e.Doc.Find(q)
	for i := len(found) - 1; i >= 0; i-- {
		n := len(e.undo)
		e.replaceRange(found[i][0], found[i][1], s, false)
		if i < len(found)-1 && len(e.undo) > n {
			e.undo[len(e.undo)-1].joined = true
		}
	}
	e.compOpen = false
	return len(found)
}

// Select selects runes [a, b), scrolling them into view.
func (e *Editor) Select(a, b int) {
	n := e.Doc.Len()
	e.anchor, e.caret = clamp(a, 0, n), clamp(b, 0, n)
	e.hasGoal, e.reveal, e.compOpen = false, true, false
}

// SetFound shows ranges found in the text, current the one stronger; nil
// clears them.
func (e *Editor) SetFound(ranges [][2]int, current int) {
	e.found, e.current = ranges, current
}

// SetDoubts shows ranges of the text something is in doubt about, with a
// light dashed line under them; nil clears them.
func (e *Editor) SetDoubts(ranges [][2]int) { e.doubts = ranges }

// LastEdit is when the text last changed.
func (e *Editor) LastEdit() time.Time { return e.lastEdit }

func (e *Editor) clampCaret() {
	n := e.Doc.Len()
	e.caret, e.anchor = clamp(e.caret, 0, n), clamp(e.anchor, 0, n)
}

// Backspace and DeleteForward delete the selection, or a rune.
func (e *Editor) Backspace() {
	a, b := e.Selection()
	if a == b {
		if a == 0 {
			return
		}
		a--
	}
	e.replaceRange(a, b, "", false)
	e.updateCompletion()
}

func (e *Editor) DeleteForward() {
	a, b := e.Selection()
	if a == b {
		if b >= e.Doc.Len() {
			return
		}
		b++
	}
	e.replaceRange(a, b, "", false)
	e.compOpen = false
}

// deleteWord deletes the selection, or else back (forward when ahead) to
// where Cmd+Left (Cmd+Right) would go. At a paragraph's edge it takes only
// the line break, as a plain Backspace (Delete) does.
func (e *Editor) deleteWord(ahead bool) {
	a, b := e.Selection()
	switch {
	case a != b:
	case ahead && b < e.Doc.Len() && e.Doc.text[b] == '\n', !ahead && a > 0 && e.Doc.text[a-1] == '\n':
		if ahead {
			b++
		} else {
			a--
		}
	case ahead:
		b = e.wordRight()
	default:
		a = e.wordLeft()
	}
	if a == b {
		return
	}
	e.replaceRange(a, b, "", false)
	e.compOpen = false
}

// MoveTo moves the caret to i, extending the selection when extend.
func (e *Editor) MoveTo(i int, extend bool) {
	e.caret = clamp(i, 0, e.Doc.Len())
	if !extend {
		e.anchor = e.caret
	}
	e.hasGoal, e.reveal, e.compOpen = false, true, false
}

// SelectAll selects the whole text.
func (e *Editor) SelectAll() {
	e.anchor, e.caret = 0, e.Doc.Len()
	e.compOpen = false
}

// SelectedText returns the text selected.
func (e *Editor) SelectedText() string {
	a, b := e.Selection()
	return e.Doc.Slice(a, b)
}

// wordAt returns the run of runes of one class around i.
func (e *Editor) wordAt(i int) (int, int) {
	r := e.Doc.text
	if len(r) == 0 {
		return 0, 0
	}
	i = clamp(i, 0, len(r)-1)
	if r[i] == '\n' && i > 0 {
		i--
	}
	cls := runeClass(r[i])
	a, b := i, i+1
	for a > 0 && r[a-1] != '\n' && runeClass(r[a-1]) == cls {
		a--
	}
	for b < len(r) && r[b] != '\n' && runeClass(r[b]) == cls {
		b++
	}
	return a, b
}

// paragraphAt returns the paragraph around i, without its line break.
func (e *Editor) paragraphAt(i int) (int, int) {
	r := e.Doc.text
	a, b := clamp(i, 0, len(r)), clamp(i, 0, len(r))
	for a > 0 && r[a-1] != '\n' {
		a--
	}
	for b < len(r) && r[b] != '\n' {
		b++
	}
	return a, b
}

// AcceptGhost inserts the prediction shown, as one step, marking it as
// accepted AI text. It reports whether there was one.
func (e *Editor) AcceptGhost() bool {
	if !e.ghostValid() {
		return false
	}
	s, at := e.ghost, e.caret
	e.replaceRange(at, at, s, false)
	n := len([]rune(s))
	note := ""
	if e.opts.GhostNote != nil {
		note = e.opts.GhostNote()
	}
	e.Doc.AddMark(MarkAIAccepted, at, at+n, note)
	e.undo[len(e.undo)-1].marksAfter = e.Doc.Marks()
	return true
}

// MarkSelection marks the selection; it reports whether there was one.
func (e *Editor) MarkSelection(kind MarkKind, note string) bool {
	a, b := e.Selection()
	if a == b {
		return false
	}
	before := e.Doc.Marks()
	e.Doc.AddMark(kind, a, b, note)
	e.undo = append(e.undo, undoStep{at: a, caretBefore: e.caret, anchorBefore: e.anchor, caretAfter: e.caret,
		marksBefore: before, marksAfter: e.Doc.Marks(), when: time.Now()})
	e.redo = nil
	return true
}

// AddMark marks m's range with all m holds, as one step to undo, and
// returns the mark's id.
func (e *Editor) AddMark(m Mark) int {
	before := e.Doc.Marks()
	id := e.Doc.Add(m)
	e.undo = append(e.undo, undoStep{at: m.Start, caretBefore: e.caret, anchorBefore: e.anchor, caretAfter: e.caret,
		marksBefore: before, marksAfter: e.Doc.Marks(), when: time.Now()})
	e.redo = nil
	return id
}

// updateCompletion offers the entities matching what was just typed.
func (e *Editor) updateCompletion() {
	e.compOpen = false
	if e.caret != e.anchor || len(e.opts.Entities) == 0 {
		return
	}
	c, k := candidates(e.Doc.text[:e.caret], e.opts.Entities)
	if len(c) == 0 {
		return
	}
	e.comp, e.compTyped, e.compSel, e.compOpen = c, k, 0, true
}

// AcceptCompletion types the rest of the completion chosen.
func (e *Editor) AcceptCompletion() bool {
	if !e.compOpen || e.compSel >= len(e.comp) {
		return false
	}
	rest := []rune(e.comp[e.compSel].Name)[e.compTyped:]
	e.compOpen = false
	e.replaceRange(e.caret, e.caret, string(rest), false)
	return true
}
