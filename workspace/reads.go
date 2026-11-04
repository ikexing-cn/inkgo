package workspace

import (
	"errors"
	"os"
	"strings"
	"unicode"
)

// Outlines reads only the outlines of all live nodes, including folders.
// Prose and marks are not opened; missing outline files mean empty outlines.
func (s *Store) Outlines() (map[string]string, error) { return s.outlines(nil) }

// OutlinesOf reads the outlines of the live nodes ids names, as Outlines
// reads them.
func (s *Store) OutlinesOf(ids ...string) (map[string]string, error) {
	want := make(map[string]bool, len(ids))
	for _, id := range ids {
		want[id] = true
	}
	return s.outlines(func(n *Node) bool { return want[n.ID] })
}

// outlines reads the outlines of the nodes pick takes, or of all for nil.
func (s *Store) outlines(pick func(*Node) bool) (map[string]string, error) {
	unlock, err := s.lock()
	if err != nil {
		return nil, err
	}
	defer unlock()
	t, err := s.tree()
	if err != nil {
		return nil, err
	}
	out := make(map[string]string)
	t.Walk(func(n *Node, _ int) {
		if err != nil || pick != nil && !pick(n) {
			return
		}
		var p string
		p, err = s.outlinePath(n.ID)
		if err != nil {
			return
		}
		var b []byte
		b, err = os.ReadFile(p)
		if errors.Is(err, os.ErrNotExist) {
			err = nil
		}
		out[n.ID] = strings.ReplaceAll(string(b), "\r\n", "\n")
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Count measures text using the book language, with CJK counted individually.
func (l Language) Count(s string) int {
	if l == English {
		r := []rune(s)
		n := 0
		inside := false
		for i, ch := range r {
			cjk := unicode.In(ch, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul)
			if cjk {
				n++
				inside = false
				continue
			}
			if unicode.IsLetter(ch) || unicode.IsDigit(ch) {
				if !inside {
					n++
				}
				inside = true
				continue
			}
			if (ch == '\'' || ch == '-') && inside && i+1 < len(r) && (unicode.IsLetter(r[i+1]) || unicode.IsDigit(r[i+1])) {
				continue
			}
			inside = false
		}
		return n
	}
	n := 0
	for _, r := range s {
		if !unicode.IsSpace(r) {
			n++
		}
	}
	return n
}

// Counts returns the count of each document's text, by ID.
func (s *Store) Counts() (map[string]int, error) {
	texts, err := s.Texts()
	if err != nil {
		return nil, err
	}
	counts := map[string]int{}
	for id, text := range texts {
		counts[id] = s.Work().Language.Count(text)
	}
	return counts, nil
}

// Texts returns each document's text as ReadChapter gives it, by ID, with
// one reading of the tree for them all.
func (s *Store) Texts() (map[string]string, error) {
	unlock, err := s.lock()
	if err != nil {
		return nil, err
	}
	defer unlock()
	t, err := s.tree()
	if err != nil {
		return nil, err
	}
	texts := map[string]string{}
	for id, n := range t.byID {
		if n.Folder {
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
		texts[id] = CleanProse(string(b))
	}
	return texts, nil
}

// Piece is a node of the chapter tree as it is exported: its title, how
// deep in the tree it is, from 0, and its prose and outline.
type Piece struct {
	ID      string
	Title   string
	Depth   int
	Folder  bool
	Text    string
	Outline string
}

// Pieces reads every node of the tree in its order.
func (s *Store) Pieces() ([]Piece, error) {
	unlock, err := s.lock()
	if err != nil {
		return nil, err
	}
	defer unlock()
	t, err := s.tree()
	if err != nil {
		return nil, err
	}
	var pieces []Piece
	var failed error
	t.Walk(func(n *Node, depth int) {
		if failed != nil {
			return
		}
		ch, err := s.readChapter(n)
		if err != nil {
			failed = err
			return
		}
		pieces = append(pieces, Piece{ID: n.ID, Title: n.Title, Depth: depth, Folder: n.Folder, Text: ch.Text, Outline: strings.TrimSpace(ch.Outline)})
	})
	return pieces, failed
}
