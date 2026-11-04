package editor

import "github.com/egoist/mygo/ui"

// SetDoubtColor sets the host's color for finding underlines.
func (e *Editor) SetDoubtColor(color ui.Color) { e.doubtColor = color }

// SetNoteHighlight shades a display-only range. An empty range clears it.
func (e *Editor) SetNoteHighlight(a, b int) { e.noteHighlight = [2]int{a, b} }

// HoveredLine is the underlined finding or foreshadowing under the pointer.
// It is empty over unmarked text, names and space outside the glyphs.
func (e *Editor) HoveredLine() (a, b int) { return e.lineHover[0], e.lineHover[1] }

func (e *Editor) markedRangeAt(x, y float32) [2]int {
	l, ok := e.geometry()
	if !ok {
		return [2]int{}
	}
	y += e.scrollY
	ln := &l.lines[l.lineAt(y)]
	hit := func(a, b int) bool {
		from, to := max(l.toDisp(a, true), ln.start), min(l.toDisp(b, false), ln.end)
		if from >= to || y < ln.y || y >= ln.y+l.lineH {
			return false
		}
		if ln.sh.first != nil {
			return x >= l.left+l.x(ln, from) && x < l.left+l.x(ln, to)
		}
		hit := false
		l.ranges(from, to, func(x0, x1, row float32) {
			if row == ln.y && x >= l.left+x0 && x < l.left+x1 {
				hit = true
			}
		})
		return hit
	}
	for _, r := range e.doubts {
		if hit(r[0], r[1]) {
			return r
		}
	}
	for _, m := range e.Doc.marks {
		if !e.hidden(m) && m.End > m.Start && (m.Kind == MarkForeshadow || m.Kind == MarkPayoff) && hit(m.Start, m.End) {
			return [2]int{m.Start, m.End}
		}
	}
	return [2]int{}
}
