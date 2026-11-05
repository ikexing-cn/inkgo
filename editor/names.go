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
		e.namesRevision++
		e.namesVer = -1
		e.nameBoxes = nil
	}
	e.openName = open
}

func (e *Editor) rebuildNames() {
	if e.namesVer == e.Doc.textVer {
		return
	}
	e.namesVer, e.nameSpans = e.Doc.textVer, e.nameSpans[:0]
	e.nameBoxes = nil
	for _, n := range e.names {
		r := []rune(n.Name)
		if len(r) == 0 {
			continue
		}
		for at := 0; at+len(r) <= e.Doc.Len(); at++ {
			if !slices.Equal(e.Doc.text[at:at+len(r)], r) {
				continue
			}
			e.nameSpans = append(e.nameSpans, nameRange{at, at + len(r), n.Entry})
			at += len(r) - 1
		}
	}
}

func (e *Editor) nameRanges() []nameRange { return e.proseLayers().names }

func (e *Editor) visibleNames(top, bottom float32, fn func(nameBox)) {
	visible := e.nameRanges()
	if e.nameBoxes == nil || e.nameLayout != e.layoutVer {
		e.nameBoxes = make([]nameBox, 0, len(visible))
		e.nameLayout = e.layoutVer
		l := &e.lay
		for _, n := range visible {
			l.eachVisible(l.toDisp(n.a, true), l.toDisp(n.b, false), -1, l.height, func(x0, x1, y float32) {
				e.nameBoxes = append(e.nameBoxes, nameBox{n, x0, x1, y})
			})
		}
		slices.SortStableFunc(e.nameBoxes, func(a, b nameBox) int {
			if a.y < b.y {
				return -1
			}
			if a.y > b.y {
				return 1
			}
			return a.span.a - b.span.a
		})
	}
	i := sort.Search(len(e.nameBoxes), func(i int) bool { return e.nameBoxes[i].y >= top })
	for ; i < len(e.nameBoxes) && e.nameBoxes[i].y <= bottom; i++ {
		box := e.nameBoxes[i]
		fn(box)
	}
}

// nameAt hits actual glyph extents, including wrapped names; empty space
// near a name must still place the caret normally.
func (e *Editor) nameAt(x, y float32) string {
	return e.nameRangeAt(x, y).entry
}

func (e *Editor) nameRangeAt(x, y float32) nameRange {
	e.ensureLayout()
	l := &e.lay
	var hit nameRange
	y += e.scrollY
	e.visibleNames(y-l.lineH, y, func(box nameBox) {
		if hit.entry == "" && x >= l.left+box.x0 && x < l.left+box.x1 && y < box.y+l.lineH {
			hit = box.span
		}
	})
	return hit
}

func (e *Editor) paintNames(p *ui.Painter, ox, oy, top, bottom float32) {
	l := &e.lay
	e.visibleNames(top, bottom, func(box nameBox) {
		if box.span == e.nameHover {
			dashed(p, ox+box.x0, ox+box.x1, oy+box.y+l.lineH-5, p.Theme().TextMuted.Alpha(0.3))
		}
	})
}
