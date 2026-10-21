package workspace

import (
	"slices"
	"strings"
)

// Idea is a thing the AI suggests that the author has not decided: an
// entry waiting, with the settled entry it is about, if one; or a change of
// a settled entry from a chapter on.
type Idea struct {
	Entry  Entry  // the entry waiting, or the one the change is of
	Change Change // with an ID, the change
	About  string // the settled entry it is about
}

// ID identifies the suggestion or the proposed change.
func (i Idea) ID() string {
	if i.Change.ID != "" {
		return i.Change.ID
	}
	return i.Entry.ID
}

// Ideas are what the AI suggests and the author has not decided: the
// entries waiting, each about the settled one that shares a name with it,
// or whose name it says; then the changes of settled entries, each about
// its own, while its chapter is in the tree.
func (s Setting) Ideas(o Order) []Idea {
	var out, changes []Idea
	for _, e := range s.Entries {
		if e.Place == PlaceBody {
			for _, g := range e.Changes {
				if _, ok := o[g.From]; ok && g.Place == PlaceOpen {
					changes = append(changes, Idea{Entry: e, Change: g, About: e.ID})
				}
			}
		}
		if e.Place != PlaceOpen {
			continue
		}
		it := Idea{Entry: e}
		best := 0
		for _, b := range s.Entries {
			if b.Place != PlaceBody {
				continue
			}
			for _, name := range b.Names {
				n := len(name)
				if slices.Contains(e.Names, name) {
					n += len(e.Text) // a shared name tells more than a mention
				} else if !strings.Contains(e.Text, name) {
					continue
				}
				if n > best {
					best, it.About = n, b.ID
				}
			}
		}
		out = append(out, it)
	}
	return append(out, changes...)
}
