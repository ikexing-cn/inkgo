package workspace

import "strings"

// lateDefault is how many chapters may follow a foreshadowing before it is
// listed as buried long, until the author sets another number.
const lateDefault = 30

// ForeshadowStatus is derived from the source mark and its payoffs.
type ForeshadowStatus int

const (
	ForeOpen    ForeshadowStatus = iota // buried, not paid off
	ForeDone                            // paid off somewhere
	ForeDropped                         // given up
	ForeLost                            // its sentence was deleted
)

// Payoff is a place that pays a foreshadowing off.
type Payoff struct {
	ID, Node, Title, Quote, At string
	Chapter, Start, End        int // Chapter counts the chapters from 1
	Here                       bool
}

// Foreshadow is a foreshadowing of the book with what pays it off, in the
// order of the chapters.
type Foreshadow struct {
	ID, Node, Title, Note, Quote, At string
	Chapter, Start, End              int
	Here, Lost, Dropped              bool
	Payoffs                          []Payoff
	// Gap is how many chapters lie between its chapter and the first that
	// pays it off, or the last of the book while none does.
	Gap int
}

// Status gives dropped and lost source marks priority over payoffs.
func (f Foreshadow) Status() ForeshadowStatus {
	switch {
	case f.Dropped:
		return ForeDropped
	case f.Lost:
		return ForeLost
	case len(f.Payoffs) > 0:
		return ForeDone
	}
	return ForeOpen
}

// LateThreshold is how many chapters may follow a foreshadowing before it counts as
// buried long; a nonpositive preference uses the default of 30.
func LateThreshold(after int) int {
	if after > 0 {
		return after
	}
	return lateDefault
}

// Foreshadows gathers the book's foreshadowings and what pays them off:
// the open chapter's as it is being written, the others' as of the disk.
// current is empty without a live chapter; live replaces its stored marks.
func (t Tree) Foreshadows(stored map[string][]ChapterMark, current string, live []ChapterMark) []Foreshadow {
	var out []Foreshadow
	pays := map[string][]Payoff{}
	n := 0
	t.Walk(func(nd *Node, _ int) {
		if nd.Folder {
			return
		}
		n++
		marks := stored[nd.ID]
		here := nd.ID == current
		if here {
			marks = live
		}
		for _, m := range marks {
			switch {
			case m.Kind == MarkForeshadow:
				out = append(out, Foreshadow{ID: m.ID, Node: nd.ID, Title: nd.Title, Note: m.Note, Quote: m.Quote, At: m.At,
					Chapter: n, Start: m.Start, End: m.End, Here: here, Lost: m.Orphaned, Dropped: m.Dropped})
			case m.Kind == MarkPayoff && !m.Orphaned && m.Of != "":
				pays[m.Of] = append(pays[m.Of], Payoff{ID: m.ID, Node: nd.ID, Title: nd.Title, Quote: m.Quote, At: m.At,
					Chapter: n, Start: m.Start, End: m.End, Here: here})
			}
		}
	})
	for i := range out {
		f := &out[i]
		f.Payoffs = pays[f.ID]
		f.Gap = n - f.Chapter
		if len(f.Payoffs) > 0 {
			f.Gap = max(f.Payoffs[0].Chapter-f.Chapter, 0)
		}
	}
	return out
}

// ForeshadowCandidates are the foreshadowings a passage of the chapter open may pay
// off, those matching the search: all buried in it or before it but the
// lost and the dropped. here counts the chapters up to the open one.
func (t Tree) ForeshadowCandidates(all []Foreshadow, current, query string) (out []Foreshadow, here int) {
	q := strings.TrimSpace(query)
	n := 0
	t.Walk(func(nd *Node, _ int) {
		if !nd.Folder {
			n++
			if nd.ID == current {
				here = n
			}
		}
	})
	for _, f := range all {
		if st := f.Status(); st == ForeLost || st == ForeDropped || f.Chapter > here {
			continue
		}
		if q == "" || strings.Contains(f.Note, q) || strings.Contains(f.Quote, q) {
			out = append(out, f)
		}
	}
	return out, here
}

// Late reports whether an open foreshadowing has reached the preferred chapter gap.
func (f Foreshadow) Late(after int) bool {
	return f.Status() == ForeOpen && f.Gap >= LateThreshold(after)
}
