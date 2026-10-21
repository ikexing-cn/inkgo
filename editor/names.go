package editor

import (
	"slices"
	"sort"

	"github.com/egoist/mygo/ui"
)

// NameLink is a confirmed name and the setting entry it opens. It is
// display data: never part of Doc, its marks, undo or a saved chapter.
type NameLink struct{ Name, Entry string }
type nameRange struct {
	a, b  int
	entry string
}

type nameBox struct {
	span      nameRange
	x0, x1, y float32
}

func (e *Editor) SetNameLinks(names []NameLink, open func(string)) {
	if !slices.Equal(e.names, names) {
		e.names = slices.Clone(names)
		e.namesVer = -1
		e.nameBoxes = nil
	}
	e.openName = open
}

func (e *Editor) rebuildNames() {
	if e.namesVer != e.Doc.textVer {
		e.namesVer, e.nameSpans = e.Doc.textVer, e.nameSpans[:0]
		e.nameBoxes = nil
		scratch := make([]nameRange, 0, cap(e.nameSpans))
		var added []nameRange
		for _, n := range e.names {
			r := []rune(n.Name)
			if len(r) == 0 {
				continue
			}
			added = added[:0]
			j := 0
			for at := 0; at+len(r) <= e.Doc.Len(); at++ {
				if !slices.Equal(e.Doc.text[at:at+len(r)], r) {
					continue
				}
				x := nameRange{at, at + len(r), n.Entry}
				for j < len(e.nameSpans) && e.nameSpans[j].b <= x.a {
					j++
				}
				if j == len(e.nameSpans) || e.nameSpans[j].a >= x.b {
					added = append(added, x)
				}
				at += len(r) - 1
			}
			// Earlier names win overlaps; merge only after sweeping this name.
			merged := scratch[:0]
			i, j := 0, 0
			for i < len(e.nameSpans) && j < len(added) {
				if e.nameSpans[i].a < added[j].a {
					merged = append(merged, e.nameSpans[i])
					i++
				} else {
					merged = append(merged, added[j])
					j++
				}
			}
			merged = append(merged, e.nameSpans[i:]...)
			merged = append(merged, added[j:]...)
			e.nameSpans, scratch = merged, e.nameSpans[:0]
		}
	}
}

func (e *Editor) nameRanges() []nameRange {
	e.rebuildNames()
	if len(e.nameSpans) == 0 {
		return nil
	}
	out := make([]nameRange, 0, len(e.nameSpans))
	for _, n := range e.nameSpans {
		if e.nameVisible(n) {
			out = append(out, n)
		}
	}
	return out
}

func (e *Editor) nameVisible(n nameRange) bool {
	overlaps := func(a, b int) bool { return a < n.b && n.a < b }
	if slices.ContainsFunc(e.Doc.marks, func(m Mark) bool { return !e.hidden(m) && overlaps(m.Start, m.End) }) ||
		slices.ContainsFunc(e.found, func(r [2]int) bool { return overlaps(r[0], r[1]) }) ||
		slices.ContainsFunc(e.doubts, func(r [2]int) bool { return overlaps(r[0], r[1]) }) ||
		slices.ContainsFunc(e.props, func(p Proposal) bool { return overlaps(p.A, p.B) }) {
		return false
	}
	a, b := e.Selection()
	if a != b && overlaps(a, b) {
		return false
	}
	return true
}

func (e *Editor) visibleNames(top, bottom float32, fn func(nameBox)) {
	e.rebuildNames()
	if e.nameBoxes == nil || e.nameLayout != e.layoutVer {
		e.nameBoxes = make([]nameBox, 0, len(e.nameSpans))
		e.nameLayout = e.layoutVer
		l := &e.lay
		for _, n := range e.nameSpans {
			l.eachVisible(l.toDisp(n.a, true), l.toDisp(n.b, false), -1, l.height, func(x0, x1, y float32) {
				e.nameBoxes = append(e.nameBoxes, nameBox{n, x0, x1, y})
			})
		}
	}
	i := sort.Search(len(e.nameBoxes), func(i int) bool { return e.nameBoxes[i].y >= top })
	for ; i < len(e.nameBoxes) && e.nameBoxes[i].y <= bottom; i++ {
		box := e.nameBoxes[i]
		if e.nameVisible(box.span) {
			fn(box)
		}
	}
}

// nameAt hits actual glyph extents, including wrapped names; empty space
// near a name must still place the caret normally.
func (e *Editor) nameAt(x, y float32) string {
	e.ensureLayout()
	l := &e.lay
	var entry string
	y += e.scrollY
	e.visibleNames(y-l.lineH, y, func(box nameBox) {
		if entry == "" && x >= l.left+box.x0 && x < l.left+box.x1 && y < box.y+l.lineH {
			entry = box.span.entry
		}
	})
	return entry
}

func (e *Editor) paintNames(p *ui.Painter, ox, oy, top, bottom float32) {
	l := &e.lay
	e.visibleNames(top, bottom, func(box nameBox) {
		dashed(p, ox+box.x0, ox+box.x1, oy+box.y+l.lineH-5, p.Theme().TextMuted.Alpha(0.3))
	})
}
