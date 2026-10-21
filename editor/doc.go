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
	// MarkAI is text an AI wrote and a human took: a prediction taken with
	// Tab, or a change proposed in a talk. What a human edits in it leaves
	// it.
	MarkAI
	// MarkPayoff is where a foreshadowing, of this chapter or another, is
	// paid off.
	MarkPayoff
)

func (k MarkKind) String() string {
	switch k {
	case MarkForeshadow:
		return "伏笔"
	case MarkAI:
		return "AI 写的"
	case MarkPayoff:
		return "回收"
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
	// Quote is the text the mark covered as it was made or loaded, which
	// tells what an orphaned mark was.
	Quote string
	// What the host keeps with a mark, which the editor carries along: a
	// name that outlives the ID, the name of the mark this one answers,
	// when it was made, and that the author gave it up.
	Key, Ref, At string
	Dropped      bool
}

// Doc is the text of a chapter and its marks. Offsets are rune indexes.
type Doc struct {
	text    []rune
	marks   []Mark
	nextID  int
	version int
	textVer int
}

// NewDoc returns a document holding s.
func NewDoc(s string) *Doc { return &Doc{text: []rune(normalize(s)), nextID: 1} }

func (d *Doc) Len() int              { return len(d.text) }
func (d *Doc) String() string        { return string(d.text) }
func (d *Doc) Version() int          { return d.version }
func (d *Doc) Slice(a, b int) string { return string(d.text[a:b]) }

// Marks returns a copy of the marks.
func (d *Doc) Marks() []Mark { return slices.Clone(d.marks) }

func (d *Doc) setMarks(m []Mark) { d.marks = slices.Clone(m) }

// Add marks m's range with all m holds and returns the mark's id.
func (d *Doc) Add(m Mark) int {
	a, b := d.span(m.Start, m.End)
	m.ID, m.Start, m.End, m.Orphaned, m.Quote = d.nextID, a, b, false, string(d.text[a:b])
	d.nextID++
	d.marks = append(d.marks, m)
	d.version++
	return m.ID
}

// EditMark changes the mark of id as fn does, and tells whether there is
// one. A mark fn moves quotes what it covers now.
func (d *Doc) EditMark(id int, fn func(m *Mark)) bool {
	for i := range d.marks {
		m := &d.marks[i]
		if m.ID != id {
			continue
		}
		a, b := m.Start, m.End
		fn(m)
		m.ID = id
		if m.Start != a || m.End != b {
			m.Start, m.End = d.span(m.Start, m.End)
			m.Orphaned, m.Quote = false, string(d.text[m.Start:m.End])
		}
		d.version++
		return true
	}
	return false
}

// RemoveMark removes the mark of id.
func (d *Doc) RemoveMark(id int) {
	d.marks = slices.DeleteFunc(d.marks, func(m Mark) bool { return m.ID == id })
	d.version++
}

// LoadMarks gives the document marks read back, with ids of its own.
func (d *Doc) LoadMarks(marks []Mark) {
	d.marks = nil
	for _, m := range marks {
		m.Start, m.End = clamp(m.Start, 0, len(d.text)), clamp(m.End, 0, len(d.text))
		m.End = max(m.End, m.Start)
		m.ID = d.nextID
		d.nextID++
		d.marks = append(d.marks, m)
	}
	d.version++
}

// Find returns the ranges where q occurs, without overlapping.
func (d *Doc) Find(q string) [][2]int {
	k := []rune(q)
	var out [][2]int
	if len(k) == 0 {
		return nil
	}
	for i := 0; i+len(k) <= len(d.text); {
		if slices.Equal(d.text[i:i+len(k)], k) {
			out = append(out, [2]int{i, i + len(k)})
			i += len(k)
			continue
		}
		i++
	}
	return out
}

// ReplaceAll replaces every q with s, moving the marks with the text as an
// edit would, and says how many. It is for a document not in an editor;
// an editor's own ReplaceAll can be undone.
func (d *Doc) ReplaceAll(q, s string) int {
	found := d.Find(q)
	ins := []rune(s)
	for i := len(found) - 1; i >= 0; i-- {
		d.replace(found[i][0], found[i][1], ins)
	}
	return len(found)
}

// replace replaces runes [a, b) with ins, moving the marks with the text.
func (d *Doc) replace(a, b int, ins []rune) {
	d.text = slices.Concat(d.text[:a:a], ins, d.text[b:])
	n := len(ins)
	var split []Mark
	drop := map[int]bool{}
	for i := range d.marks {
		m := &d.marks[i]
		wasEmpty := m.End <= m.Start
		if m.Kind == MarkAI && !wasEmpty && a < m.End && b > m.Start {
			// An edit inside what an AI wrote takes its words out of it:
			// what is left before and after the edit stays the AI's.
			delta := n - (b - a)
			before, after := m.Start < a, b < m.End
			switch {
			case before && after:
				split = append(split, Mark{Kind: m.Kind, Start: a + n, End: m.End + delta, Note: m.Note})
				m.End = a
			case before:
				m.End = a
			case after:
				m.Start, m.End = a+n, m.End+delta
			default:
				drop[m.ID] = true
			}
			continue
		}
		if m.Start == a && m.End == b && !wasEmpty && n > 0 {
			m.End = a + n // the words marked were replaced: the mark covers the new
			continue
		}
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
	d.marks = slices.DeleteFunc(d.marks, func(m Mark) bool { return drop[m.ID] })
	for _, m := range split {
		m.ID = d.nextID
		d.nextID++
		d.marks = append(d.marks, m)
	}
	d.version++
	if a != b || len(ins) != 0 {
		d.textVer++
	}
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

// normalize turns the line breaks of other systems into \n, and drops
// what no text holds: control characters, and the rune that holds room
// after what is proposed.
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
		if r < 0x20 && r != '\n' || r == gapRune {
			continue
		}
		out = append(out, r)
	}
	return string(out)
}

func clamp(v, lo, hi int) int { return max(lo, min(v, hi)) }

func (d *Doc) span(a, b int) (int, int) {
	return clamp(min(a, b), 0, d.Len()), clamp(max(a, b), 0, d.Len())
}
