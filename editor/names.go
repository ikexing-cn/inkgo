package editor

import (
	"github.com/egoist/mygo/ui"
	"slices"
)

// NameLink is a confirmed name and the setting entry it opens. It is
// display data: never part of Doc, its marks, undo or a saved chapter.
type NameLink struct{ Name, Entry string }
type nameRange struct {
	a, b  int
	entry string
}

func (e *Editor) SetNameLinks(names []NameLink, open func(string)) {
	if !slices.Equal(e.names, names) {
		e.names = slices.Clone(names)
		e.namesVer = -1
	}
	e.openName = open
}

func (e *Editor) nameRanges() []nameRange {
	if e.namesVer != e.Doc.version {
		e.namesVer, e.nameSpans = e.Doc.version, nil
		for _, n := range e.names {
			r := []rune(n.Name)
			if len(r) == 0 {
				continue
			}
			for at := 0; at+len(r) <= e.Doc.Len(); at++ {
				if !slices.Equal(e.Doc.text[at:at+len(r)], r) {
					continue
				}
				x := nameRange{at, at + len(r), n.Entry}
				if !slices.ContainsFunc(e.nameSpans, func(k nameRange) bool { return x.a < k.b && k.a < x.b }) {
					e.nameSpans = append(e.nameSpans, x)
				}
				at += len(r) - 1
			}
		}
	}
	var out []nameRange
	for _, n := range e.nameSpans {
		overlaps := func(a, b int) bool { return a < n.b && n.a < b }
		if slices.ContainsFunc(e.Doc.marks, func(m Mark) bool { return !e.hidden(m) && overlaps(m.Start, m.End) }) ||
			slices.ContainsFunc(e.found, func(r [2]int) bool { return overlaps(r[0], r[1]) }) ||
			slices.ContainsFunc(e.doubts, func(r [2]int) bool { return overlaps(r[0], r[1]) }) ||
			slices.ContainsFunc(e.props, func(p Proposal) bool { return overlaps(p.A, p.B) }) {
			continue
		}
		a, b := e.Selection()
		if a != b && overlaps(a, b) {
			continue
		}
		out = append(out, n)
	}
	return out
}

// nameAt hits actual glyph extents, including wrapped names; empty space
// near a name must still place the caret normally.
func (e *Editor) nameAt(x, y float32) string {
	e.ensureLayout()
	l := &e.lay
	for _, n := range e.nameRanges() {
		hit := false
		l.ranges(l.toDisp(n.a, true), l.toDisp(n.b, false), func(x0, x1, top float32) {
			if x >= l.left+x0 && x < l.left+x1 && y+e.scrollY >= top && y+e.scrollY < top+l.lineH {
				hit = true
			}
		})
		if hit {
			return n.entry
		}
	}
	return ""
}

func (e *Editor) paintNames(p *ui.Painter, ox, oy, top, bottom float32) {
	l := &e.lay
	for _, n := range e.nameRanges() {
		l.ranges(l.toDisp(n.a, true), l.toDisp(n.b, false), func(x0, x1, y float32) {
			if y >= top && y <= bottom {
				dashed(p, ox+x0, ox+x1, oy+y+l.lineH-5, p.Theme().TextMuted.Alpha(0.3))
			}
		})
	}
}
