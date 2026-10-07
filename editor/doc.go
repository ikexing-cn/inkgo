// Package editor is a proof-of-concept plain-text novel editor drawn with
// mygo's native UI: Chinese line breaking, input methods, marks that follow
// edits (foreshadowing, AI provenance), entity completion and inline AI
// predictions accepted with Tab.
package editor

import "slices"

// MarkKind is what a mark records about a range of text.
type MarkKind int

const (
	// MarkForeshadow is a foreshadowing a human marked (canon).
	MarkForeshadow MarkKind = iota + 1
	// MarkAIAccepted is text an AI predicted and a human accepted with Tab.
	MarkAIAccepted
)

func (k MarkKind) String() string {
	switch k {
	case MarkForeshadow:
		return "伏笔"
	case MarkAIAccepted:
		return "AI 采纳"
	}
	return "标记"
}

// Mark is a range of the text, in runes [Start, End), that moves with edits.
type Mark struct {
	ID         int
	Kind       MarkKind
	Start, End int
	Note       string
	// Orphaned tells that all the text the mark covered was deleted.
	Orphaned bool
}

// Doc is the text of a chapter and its marks. Offsets are rune indexes.
type Doc struct {
	text    []rune
	marks   []Mark
	nextID  int
	version int
}

// NewDoc returns a document holding s.
func NewDoc(s string) *Doc { return &Doc{text: []rune(normalize(s)), nextID: 1} }

func (d *Doc) Len() int              { return len(d.text) }
func (d *Doc) String() string        { return string(d.text) }
func (d *Doc) Version() int          { return d.version }
func (d *Doc) Slice(a, b int) string { return string(d.text[a:b]) }
func (d *Doc) Runes() []rune         { return d.text }

// Marks returns a copy of the marks.
func (d *Doc) Marks() []Mark { return slices.Clone(d.marks) }

func (d *Doc) setMarks(m []Mark) { d.marks = slices.Clone(m) }

// AddMark marks [a, b) and returns the mark's id.
func (d *Doc) AddMark(kind MarkKind, a, b int, note string) int {
	a, b = clamp(min(a, b), 0, len(d.text)), clamp(max(a, b), 0, len(d.text))
	id := d.nextID
	d.nextID++
	d.marks = append(d.marks, Mark{ID: id, Kind: kind, Start: a, End: b, Note: note})
	return id
}

// RemoveMark removes the mark of id.
func (d *Doc) RemoveMark(id int) {
	d.marks = slices.DeleteFunc(d.marks, func(m Mark) bool { return m.ID == id })
}

// replace replaces runes [a, b) with ins, moving the marks with the text.
func (d *Doc) replace(a, b int, ins []rune) {
	d.text = slices.Concat(d.text[:a:a], ins, d.text[b:])
	n := len(ins)
	for i := range d.marks {
		m := &d.marks[i]
		wasEmpty := m.End <= m.Start
		// A mark's start stays after text inserted at it, and its end
		// before, so that typing next to a mark does not grow it.
		m.Start = mapPos(m.Start, a, b, n, true)
		m.End = mapPos(m.End, a, b, n, false)
		if m.End <= m.Start {
			m.End = m.Start
			if !wasEmpty {
				m.Orphaned = true
			}
		}
	}
	d.version++
}

// mapPos maps offset p across the replacement of [a, b) by n runes; right
// biases it after text inserted at it or replacing it.
func mapPos(p, a, b, n int, right bool) int {
	switch {
	case p < a:
		return p
	case p > b:
		return p + n - (b - a)
	case right:
		return a + n
	default:
		return a
	}
}

// normalize turns the line breaks of other systems into \n.
func normalize(s string) string {
	out := make([]rune, 0, len(s))
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		if r == '\r' {
			if i+1 < len(rs) && rs[i+1] == '\n' {
				continue
			}
			r = '\n'
		}
		if r == '\t' {
			r = ' '
		}
		if r < 0x20 && r != '\n' {
			continue
		}
		out = append(out, r)
	}
	return string(out)
}

func clamp(v, lo, hi int) int { return max(lo, min(v, hi)) }
