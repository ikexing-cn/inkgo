package workspace

import (
	"fmt"
	"os"
)

// Kinds of a chapter mark.
const (
	MarkForeshadow = "foreshadow" // a foreshadowing the author marked
	MarkPayoff     = "payoff"     // where a foreshadowing is paid off
	MarkAI         = "ai"         // what an AI wrote and the author took
)

// What an AI wrote was marked two ways while drafting was apart from the
// rest; either is MarkAI as read.
const (
	markAIAccepted = "ai-accepted" // a sentence predicted, or a rewrite
	markAIDraft    = "ai-draft"    // a chapter drafted from its outline
)

// ChapterMark is a mark of a document's text, in runes [Start, End). Quote
// is the text it covered when saved, which finds it again after the file
// changed outside; one that cannot be found is Orphaned.
//
// A foreshadowing and a payoff have an ID, by which a payoff, in any
// chapter, names the foreshadowing it pays off (Of). A foreshadowing the
// author gave up is Dropped. At is when the mark was made, in RFC 3339.
type ChapterMark struct {
	Kind     string `json:"kind"`
	Start    int    `json:"start"`
	End      int    `json:"end"`
	Quote    string `json:"quote"`
	Note     string `json:"note,omitempty"`
	Orphaned bool   `json:"orphaned,omitempty"`
	ID       string `json:"id,omitempty"`
	Of       string `json:"of,omitempty"`
	Dropped  bool   `json:"dropped,omitempty"`
	At       string `json:"at,omitempty"`
}

// identify gives the foreshadowings of node id saved before they had IDs
// one each, the same at every read until the chapter is saved with it,
// and the marks of what an AI wrote, of either old kind, the one kind.
func identify(id string, marks []ChapterMark) {
	for i := range marks {
		if marks[i].Kind == MarkForeshadow && marks[i].ID == "" {
			marks[i].ID = fmt.Sprintf("old-%s-%d", id, i)
		}
		if k := marks[i].Kind; k == markAIAccepted || k == markAIDraft {
			marks[i].Kind = MarkAI
		}
	}
}

// Marks returns the marks of each chapter that has any, by ID, found in
// its text again as ReadChapter finds them.
func (s *Store) Marks() (map[string][]ChapterMark, error) {
	unlock, err := s.lock()
	if err != nil {
		return nil, err
	}
	defer unlock()
	t, err := s.tree()
	if err != nil {
		return nil, err
	}
	out := map[string][]ChapterMark{}
	for id, n := range t.byID {
		if n.Folder {
			continue
		}
		marks, err := s.readMarks(n)
		if err != nil {
			return nil, err
		}
		if len(marks) == 0 {
			continue
		}
		p, err := s.nodePath(n, textExt)
		if err != nil {
			return nil, err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		text := []rune(CleanProse(string(b)))
		for i, m := range marks {
			marks[i] = anchor(text, m)
		}
		identify(id, marks)
		out[id] = marks
	}
	return out, nil
}

// anchor finds m in text again: where it was if its quote is still there,
// else at the occurrence of its quote nearest to that, else nowhere.
func anchor(text []rune, m ChapterMark) ChapterMark {
	m.Start, m.End = clampIndex(m.Start, len(text)), clampIndex(m.End, len(text))
	if m.End < m.Start {
		m.End = m.Start
	}
	if m.Orphaned || string(text[m.Start:m.End]) == m.Quote {
		return m
	}
	q := []rune(m.Quote)
	best := -1
	for i := 0; len(q) > 0 && i+len(q) <= len(text); i++ {
		if string(text[i:i+len(q)]) == m.Quote && (best < 0 || abs(i-m.Start) < abs(best-m.Start)) {
			best = i
		}
	}
	if best < 0 {
		m.Orphaned, m.End = true, m.Start
		return m
	}
	m.Start, m.End = best, best+len(q)
	return m
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
