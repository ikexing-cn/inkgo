package workspace

import (
	"fmt"
	"slices"
	"strings"
	"time"
)

// Setting is the setting document as its revisions leave it: entries in
// the order they were added, and every revision, oldest first.
type Setting struct {
	Entries   []Entry
	Revisions []Revision
}

// Entry returns the entry id.
func (s Setting) Entry(id string) (Entry, bool) {
	for _, e := range s.Entries {
		if e.ID == id {
			return e, true
		}
	}
	return Entry{}, false
}

func validPlace(p string) bool { return p == PlaceBody || p == PlaceOpen || p == PlaceDropped }

// applyOps applies revision n, from 1, to entries, checking each op's shape
// against them.
func applyOps(entries []Entry, r Revision, n int) ([]Entry, error) {
	index := func(id string) int {
		return slices.IndexFunc(entries, func(e Entry) bool { return e.ID == id })
	}
	for k, op := range r.Ops {
		fail := func(format string, args ...any) error {
			return fmt.Errorf("ops[%d]: %s", k, fmt.Sprintf(format, args...))
		}
		for _, src := range op.Sources {
			if strings.TrimSpace(src.Quote) == "" {
				return nil, fail("source needs a quote")
			}
			if src.Chapter != "" && validID(src.Chapter) != nil {
				return nil, fail("source names an invalid chapter")
			}
			if (src.Chapter == "" || src.Session != "" || src.Message != "") && (validID(src.Session) != nil || validID(src.Message) != nil) {
				return nil, fail("source needs a session, a message and a quote, or a chapter and a quote")
			}
		}
		if err := validNames(op.Names); err != nil {
			return nil, fail("%v", err)
		}
		switch op.Op {
		case OpAdd:
			if validID(op.Entry) != nil || index(op.Entry) >= 0 {
				return nil, fail("add needs a new entry ID")
			}
			if !validPlace(op.Place) || strings.TrimSpace(op.Category) == "" || strings.TrimSpace(op.Text) == "" {
				return nil, fail("add needs a place, a category and text")
			}
			entries = append(entries, Entry{ID: op.Entry, Place: op.Place, Category: op.Category, Text: op.Text,
				Names: slices.Clone(op.Names), Sources: slices.Clone(op.Sources), Author: r.By == ByAuthor, Rev: n})
		case OpAnswer:
			i := index(op.Entry)
			if i < 0 {
				return nil, fail("unknown entry %q", op.Entry)
			}
			e := &entries[i]
			if r.By != ByAuthor || e.Place != PlaceOpen {
				return nil, fail("answer is the author's, to an open entry")
			}
			if op.Place != "" || strings.TrimSpace(op.Category) == "" || strings.TrimSpace(op.Text) == "" || len(op.Sources) > 0 {
				return nil, fail("answer needs a category and text")
			}
			e.Asked, e.Text, e.Category, e.Names, e.Place = e.Text, op.Text, op.Category, slices.Clone(op.Names), PlaceBody
			e.Author, e.Rev = true, n
		case OpEdit, OpMove, OpName:
			i := index(op.Entry)
			if i < 0 {
				return nil, fail("unknown entry %q", op.Entry)
			}
			e := &entries[i]
			switch op.Op {
			case OpEdit:
				if strings.TrimSpace(op.Text) == "" || op.Place != "" || op.Category != "" {
					return nil, fail("edit changes text, and names, and adds sources, only")
				}
				e.Text = op.Text
				if len(op.Names) > 0 {
					e.Names = slices.Clone(op.Names)
				}
			case OpName:
				if op.Text != "" || op.Place != "" || op.Category != "" || len(op.Sources) > 0 {
					return nil, fail("name changes names only")
				}
				e.Names = slices.Clone(op.Names)
			default:
				if (op.Place == "" && op.Category == "") || (op.Place != "" && !validPlace(op.Place)) || op.Text != "" || len(op.Names) > 0 {
					return nil, fail("move changes place or category, and adds sources, only")
				}
				if op.Place != "" {
					e.Place = op.Place
				}
				if op.Category != "" {
					e.Category = op.Category
				}
			}
			// What made the change, as the author's words, joins the sources.
			for _, src := range op.Sources {
				if !slices.Contains(e.Sources, src) {
					e.Sources = append(e.Sources, src)
				}
			}
			e.Author = e.Author || r.By == ByAuthor
			e.Rev = n
		default:
			return nil, fail("unknown op %q", op.Op)
		}
	}
	return entries, nil
}

func validateRevision(r Revision) error {
	if err := validSchema(r.Schema); err != nil {
		return err
	}
	if err := validID(r.ID); err != nil {
		return err
	}
	if r.At.IsZero() {
		return fmt.Errorf("at: missing timestamp")
	}
	switch r.By {
	case ByAuthor:
		if r.Session != "" || r.Turn != "" || r.Model != "" || r.Effort != "" {
			return fmt.Errorf("by: the author's revisions record only ops")
		}
	case ByTalk:
		for field, id := range map[string]string{"session": r.Session, "turn": r.Turn} {
			if err := validID(id); err != nil {
				return fmt.Errorf("%s: %w", field, err)
			}
		}
		if !validEffort(r.Effort) {
			return fmt.Errorf("effort: invalid %q", r.Effort)
		}
	case ByBookkeeper:
		if err := validID(r.Session); err != nil {
			return fmt.Errorf("session: %w", err)
		}
		if r.Turn != "" {
			return fmt.Errorf("turn: the bookkeeper answered no words")
		}
		return nil // one that failed changed nothing
	default:
		return fmt.Errorf("by: must be %s or %s", ByAuthor, ByTalk)
	}
	if len(r.Ops) == 0 {
		return fmt.Errorf("ops: a revision changes something")
	}
	return nil
}

// kept is a revision as the bookkeeper wrote it: what it recorded beside
// its ops, which batch, its reasoning and how it failed, is read and
// dropped.
type kept struct {
	Revision
	Base      int    `json:"base"`
	Skipped   []Op   `json:"skipped"`
	From      string `json:"from"`
	Through   string `json:"through"`
	Redo      string `json:"redo"`
	Failed    string `json:"failed"`
	Reasoning string `json:"reasoning"`
}

// setting reads setting.jsonl and replays it, returning the length of its
// recoverable prefix. With sc, it also checks every source still quotes its
// message, as opening a work does.
func (s *Store) setting(sc *scan) (Setting, int64, error) {
	p, err := s.path("setting.jsonl")
	if err != nil {
		return Setting{}, 0, err
	}
	var set Setting
	seen := map[string]bool{}
	prefix, err := readJSONL(p, func(raw []byte) error {
		var r Revision
		if err := decodeStrict(raw, &r); err != nil {
			var k kept
			if decodeStrict(raw, &k) != nil || k.By != ByBookkeeper {
				return err
			}
			r = k.Revision
		}
		if err := validateRevision(r); err != nil {
			return err
		}
		if seen[r.ID] {
			return fmt.Errorf("id: duplicate %q", r.ID)
		}
		entries, err := applyOps(set.Entries, r, len(set.Revisions)+1)
		if err != nil {
			return err
		}
		if sc != nil {
			if err := s.checkRevision(r, sc); err != nil {
				return err
			}
		}
		seen[r.ID] = true
		set.Entries = entries
		set.Revisions = append(set.Revisions, r)
		return nil
	})
	if err != nil {
		return Setting{}, 0, err
	}
	return set, prefix, nil
}

// checkRevision checks what r points to in the sessions: the author's
// words a talk's answered, and that each source quotes its message
// verbatim.
func (s *Store) checkRevision(r Revision, sc *scan) error {
	if r.By == ByTalk {
		events, err := sc.events(s, r.Session)
		if err != nil {
			return fmt.Errorf("session: %w", err)
		}
		if !slices.ContainsFunc(events, func(e Event) bool { return e.ID == r.Turn && e.Role == RoleUser }) {
			return fmt.Errorf("turn: unknown words of the author %q", r.Turn)
		}
	}
	for k, op := range r.Ops {
		for _, src := range op.Sources {
			if src.Message == "" {
				continue // the chapter changes as the author writes on
			}
			events, err := sc.events(s, src.Session)
			if err != nil {
				return fmt.Errorf("ops[%d].sources: %w", k, err)
			}
			i := slices.IndexFunc(events, func(e Event) bool { return e.ID == src.Message })
			if i < 0 {
				return fmt.Errorf("ops[%d].sources: unknown message %q", k, src.Message)
			}
			if !strings.Contains(events[i].Said(), src.Quote) {
				return fmt.Errorf("ops[%d].sources: the quote is not in message %q", k, src.Message)
			}
		}
	}
	return nil
}

// Setting returns the setting document.
func (s *Store) Setting() (Setting, error) {
	unlock, err := s.lock()
	if err != nil {
		return Setting{}, err
	}
	defer unlock()
	set, _, err := s.setting(nil)
	return set, err
}

// Revise appends revision r and returns it as written: its ID, time and
// schema filled in, and an ID for each entry it adds.
func (s *Store) Revise(r Revision) (Revision, error) {
	unlock, err := s.lock()
	if err != nil {
		return Revision{}, err
	}
	defer unlock()
	set, prefix, err := s.setting(nil)
	if err != nil {
		return Revision{}, err
	}
	if r.Schema == 0 {
		r.Schema = Schema
	}
	if r.At.IsZero() {
		r.At = time.Now().UTC()
	}
	used := map[string]bool{}
	for _, e := range set.Entries {
		used[e.ID] = true
	}
	for _, rev := range set.Revisions {
		used[rev.ID] = true
	}
	fresh := func() string {
		id := NewID()
		for used[id] {
			id = NewID()
		}
		used[id] = true
		return id
	}
	if r.By == ByBookkeeper {
		return Revision{}, fmt.Errorf("by: the bookkeeper writes no more")
	}
	if r.ID == "" {
		r.ID = fresh()
	}
	r.Ops = slices.Clone(r.Ops)
	for k, op := range r.Ops {
		if op.Op == OpAdd && op.Entry == "" {
			r.Ops[k].Entry = fresh()
		}
	}
	if err = validateRevision(r); err != nil {
		return Revision{}, err
	}
	categories := map[string]bool{}
	for _, c := range s.work.Categories {
		categories[c.Name] = true
	}
	for k, op := range r.Ops {
		if op.Category != "" && !categories[op.Category] {
			return Revision{}, fmt.Errorf("ops[%d]: unknown category %q", k, op.Category)
		}
	}
	if _, err = applyOps(set.Entries, r, len(set.Revisions)+1); err != nil {
		return Revision{}, err
	}
	if err = s.checkRevision(r, newScan()); err != nil {
		return Revision{}, err
	}
	if err = s.checkChapters(r); err != nil {
		return Revision{}, err
	}
	p, err := s.path("setting.jsonl")
	if err != nil {
		return Revision{}, err
	}
	return r, appendJSONL(p, prefix, r)
}

// checkChapters checks that each source in a chapter quotes its text as it
// is now; later edits of the chapter leave the source as it was.
func (s *Store) checkChapters(r Revision) error {
	var t *Tree
	for k, op := range r.Ops {
		for _, src := range op.Sources {
			if src.Chapter == "" || src.Message != "" {
				continue // a message's quote of a chapter is checked in the message
			}
			if t == nil {
				tree, err := s.tree()
				if err != nil {
					return err
				}
				t = &tree
			}
			n, ok := t.Node(src.Chapter)
			if !ok || n.Folder {
				return fmt.Errorf("ops[%d].sources: unknown chapter %q", k, src.Chapter)
			}
			ch, err := s.readChapter(n)
			if err != nil {
				return err
			}
			if !strings.Contains(ch.Text, src.Quote) {
				return fmt.Errorf("ops[%d].sources: the quote is not in chapter %q", k, n.Title)
			}
		}
	}
	return nil
}
