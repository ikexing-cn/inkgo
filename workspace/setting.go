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

// Processed returns the index after the last message of session that a
// bookkeeping revision covered, failed ones included, in its events.
func (s Setting) Processed(session string, events []Event) int {
	at := map[string]int{}
	for n, e := range events {
		at[e.ID] = n + 1
	}
	done := 0
	for _, r := range s.Revisions {
		if r.By == ByBookkeeper && r.Session == session {
			done = max(done, at[r.Through])
		}
	}
	return done
}

// Redone reports whether a later revision redid the failed revision id
// successfully.
func (s Setting) Redone(id string) bool {
	for _, r := range s.Revisions {
		if r.Redo == id && r.Failed == "" {
			return true
		}
	}
	return false
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
			if src.Chapter != "" {
				if validID(src.Chapter) != nil || src.Session != "" || src.Message != "" {
					return nil, fail("a chapter's source names the chapter only")
				}
			} else if validID(src.Session) != nil || validID(src.Message) != nil {
				return nil, fail("source needs a session, a message and a quote")
			}
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
				Sources: slices.Clone(op.Sources), Author: r.By == ByAuthor, Rev: n})
		case OpEdit, OpMove:
			i := index(op.Entry)
			if i < 0 {
				return nil, fail("unknown entry %q", op.Entry)
			}
			e := &entries[i]
			if op.Op == OpEdit {
				if strings.TrimSpace(op.Text) == "" || op.Place != "" || op.Category != "" {
					return nil, fail("edit changes text, and adds sources, only")
				}
				e.Text = op.Text
			} else {
				if (op.Place == "" && op.Category == "") || (op.Place != "" && !validPlace(op.Place)) || op.Text != "" {
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
		if r.Session != "" || r.From != "" || r.Through != "" || r.Redo != "" || r.Failed != "" || r.Base != 0 ||
			len(r.Skipped) > 0 || r.Model != "" || r.Effort != "" || r.Reasoning != "" {
			return fmt.Errorf("by: the author's revisions record only ops")
		}
		if len(r.Ops) == 0 {
			return fmt.Errorf("ops: an author's revision changes something")
		}
	case ByBookkeeper:
		for field, id := range map[string]string{"session": r.Session, "from": r.From, "through": r.Through} {
			if err := validID(id); err != nil {
				return fmt.Errorf("%s: %w", field, err)
			}
		}
		if r.Redo != "" {
			if err := validID(r.Redo); err != nil {
				return fmt.Errorf("redo: %w", err)
			}
		}
		if r.Failed != "" && len(r.Ops) > 0 {
			return fmt.Errorf("failed: a failed revision has no ops")
		}
		if !validEffort(r.Effort) {
			return fmt.Errorf("effort: invalid %q", r.Effort)
		}
	default:
		return fmt.Errorf("by: must be %s or %s", ByAuthor, ByBookkeeper)
	}
	return nil
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
			return err
		}
		if err := validateRevision(r); err != nil {
			return err
		}
		if seen[r.ID] {
			return fmt.Errorf("id: duplicate %q", r.ID)
		}
		if r.Redo != "" && !seen[r.Redo] {
			return fmt.Errorf("redo: unknown earlier revision %q", r.Redo)
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

// checkRevision checks what r points to in the sessions: its messages, and
// that each source quotes its message verbatim.
func (s *Store) checkRevision(r Revision, sc *scan) error {
	if r.By == ByBookkeeper {
		events, err := sc.events(s, r.Session)
		if err != nil {
			return fmt.Errorf("session: %w", err)
		}
		for field, id := range map[string]string{"from": r.From, "through": r.Through} {
			if !slices.ContainsFunc(events, func(e Event) bool { return e.ID == id }) {
				return fmt.Errorf("%s: unknown message %q", field, id)
			}
		}
	}
	for k, op := range r.Ops {
		for _, src := range op.Sources {
			if src.Chapter != "" {
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
			if !strings.Contains(events[i].Text, src.Quote) {
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
//
// The bookkeeper works from the first r.Base revisions. Its ops on entries
// a later revision changed are skipped, the author's change standing, and
// each becomes an open entry saying what it would have done.
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
	if r.ID == "" {
		r.ID = fresh()
	}
	if r.Base > len(set.Revisions) {
		return Revision{}, fmt.Errorf("base: only %d revisions", len(set.Revisions))
	}
	var ops []Op
	for _, op := range r.Ops {
		if op.Op == OpAdd && op.Entry == "" {
			op.Entry = fresh()
		}
		if e, ok := set.Entry(op.Entry); ok && r.By == ByBookkeeper && e.Rev > r.Base {
			r.Skipped = append(r.Skipped, op)
			ops = append(ops, Op{Op: OpAdd, Entry: fresh(), Place: PlaceOpen, Category: e.Category, Text: stale(e, op)})
			continue
		}
		ops = append(ops, op)
	}
	r.Ops = ops
	if err = validateRevision(r); err != nil {
		return Revision{}, err
	}
	if r.Redo != "" && !slices.ContainsFunc(set.Revisions, func(x Revision) bool { return x.ID == r.Redo && x.Failed != "" }) {
		return Revision{}, fmt.Errorf("redo: no failed revision %q", r.Redo)
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

// stale says what the bookkeeper would have done to e, which the author
// changed while it worked.
func stale(e Entry, op Op) string {
	switch {
	case op.Op == OpEdit:
		return fmt.Sprintf("「%s」：整理时想改成「%s」，但你刚改过这一条。", e.Text, op.Text)
	case op.Place == PlaceDropped:
		return fmt.Sprintf("「%s」：整理时想放弃这一条，但你刚改过它。", e.Text)
	default:
		return fmt.Sprintf("「%s」：整理时想移动这一条，但你刚改过它。", e.Text)
	}
}

// checkChapters checks that each source in a chapter quotes its text as it
// is now; later edits of the chapter leave the source as it was.
func (s *Store) checkChapters(r Revision) error {
	var t *Tree
	for k, op := range r.Ops {
		for _, src := range op.Sources {
			if src.Chapter == "" {
				continue
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
