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
func (e *Editor) SetProseMarks(marks []ProseMark) {
	e.proseMarks = marks
	e.marksVer++
}

func (e *Editor) decorations() []ProseMark { return e.proseLayers().marks }

// proseLayers resolves whole spans in priority order. The live selection is
// independent of rewrite/proposal/find layers; only marks and names yield to it.
type proseLayers struct {
	rewrite, selection bool
	proposals          []Proposal
	found              [][2]int
	marks              []ProseMark
	names              []nameRange
}

type proseKey struct {
	text, props, found, marks, names int
	a, b                             int
	rewriting                        bool
}

type proseCache struct {
	key    proseKey
	valid  bool
	layers proseLayers
}

func (e *Editor) proseLayers() proseLayers {
	a, b := e.Selection()
	key := proseKey{e.Doc.textVer, e.propVer, e.foundVer, e.marksVer, e.namesRevision, a, b, e.rewriting}
	if e.proseCache.valid && e.proseCache.key == key {
		return e.proseCache.layers
	}
	out := proseLayers{selection: a != b}
	occupied := newSpanIndex(e.Doc.Len())
	if e.rewriting {
		out.rewrite = occupied.keep(a, b)
	}
	for _, p := range e.props {
		if p.Keep || occupied.keep(p.A, p.B) {
			out.proposals = append(out.proposals, p)
		}
	}
	for _, r := range e.found {
		if occupied.keep(r[0], r[1]) {
			out.found = append(out.found, r)
		}
	}
	// Live selection paints independently, but reserves space for lower layers.
	occupied.add(a, b)
	for _, m := range e.proseMarks {
		if occupied.keep(m.Start, m.End) {
			out.marks = append(out.marks, m)
		}
	}
	e.rebuildNames()
	for _, n := range e.nameSpans {
		if occupied.keep(n.a, n.b) {
			out.names = append(out.names, n)
		}
	}
	slices.SortStableFunc(out.names, func(a, b nameRange) int { return a.a - b.a })
	e.proseCache = proseCache{key: key, valid: true, layers: out}
	e.nameBoxes = nil
	return out
}

// spanIndex is an occupancy tree over rune positions. Priority insertion and
// whole-span intersection take O(log n); adjacent intervals do not overlap.
// Node states are empty (0), mixed (1), and wholly occupied (2).
type spanIndex struct {
	n    int
	tree []uint8
}

func newSpanIndex(n int) spanIndex { return spanIndex{n: n, tree: make([]uint8, 4*n+4)} }
func (s spanIndex) keep(a, b int) bool {
	a, b = max(0, a), min(s.n, b)
	if b <= a || s.overlaps(1, 0, s.n, a, b) {
		return false
	}
	s.fill(1, 0, s.n, a, b)
	return true
}
func (s spanIndex) add(a, b int) {
	a, b = max(0, a), min(s.n, b)
	if b > a {
		s.fill(1, 0, s.n, a, b)
	}
}
func (s spanIndex) overlaps(node, lo, hi, a, b int) bool {
	if b <= lo || hi <= a || s.tree[node] == 0 {
		return false
	}
	if s.tree[node] == 2 || (a <= lo && hi <= b) {
		return true
	}
	mid := (lo + hi) / 2
	return s.overlaps(node*2, lo, mid, a, b) || s.overlaps(node*2+1, mid, hi, a, b)
}
func (s spanIndex) fill(node, lo, hi, a, b int) {
	if b <= lo || hi <= a || s.tree[node] == 2 {
		return
	}
	if a <= lo && hi <= b {
		s.tree[node] = 2
		return
	}
	mid := (lo + hi) / 2
	s.fill(node*2, lo, mid, a, b)
	s.fill(node*2+1, mid, hi, a, b)
	s.tree[node] = 1
	if s.tree[node*2] == 2 && s.tree[node*2+1] == 2 {
		s.tree[node] = 2
	}
}

// SetRewriting gives the rewrite request's selection first display priority.
func (e *Editor) SetRewriting(on bool) {
	if e.rewriting != on {
		e.rewriting = on
		e.propVer++
	}
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
