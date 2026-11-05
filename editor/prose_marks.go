package editor

import "slices"

// ProseMark is a display-only decoration supplied by the marks prerequisite.
// Open runs on the frame thread; it must already be guarded by its source.
type ProseMark struct {
	Kind       MarkKind
	Start, End int
	Dropped    bool
	Open       func()
}

// MarkCheck is a dashed consistency-check underline.
const MarkCheck MarkKind = 4

// SetProseMarks makes the host's marks the sole persistent decorations.
// Document marks remain available for editing, anchoring and undo.
func (e *Editor) SetProseMarks(marks []ProseMark) { e.proseMarks = marks }

func (e *Editor) decorations() []ProseMark { return e.visibleProseMarks(e.proseMarks) }

// visibleProseMarks applies the same first-overlap-wins rule as the prototype.
func (e *Editor) visibleProseMarks(marks []ProseMark) []ProseMark {
	type span struct{ a, b int }
	var occupied []span
	a, b := e.Selection()
	if a != b {
		occupied = append(occupied, span{a, b})
	}
	for _, p := range e.props {
		if !p.Keep {
			occupied = append(occupied, span{p.A, p.B})
		}
	}
	for _, r := range e.found {
		occupied = append(occupied, span{r[0], r[1]})
	}
	var out []ProseMark
	for _, m := range marks {
		if m.End <= m.Start || slices.ContainsFunc(occupied, func(s span) bool { return s.a < m.End && m.Start < s.b }) {
			continue
		}
		occupied = append(occupied, span{m.Start, m.End})
		out = append(out, m)
	}
	return out
}
func (e *Editor) proseMarkAt(x, y float32) *ProseMark {
	r := e.markedRangeAt(x, y)
	a, b := r[0], r[1]
	if a == b {
		return nil
	}
	for _, m := range e.decorations() {
		if m.Start == a && m.End == b {
			return &m
		}
	}
	return nil
}
