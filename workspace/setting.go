package workspace

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"writer/internal/strictjson"
)

// Setting is the setting document as its revisions leave it: entries in
// the order they were added, and every revision, oldest first.
type Setting struct {
	Entries   []Entry
	Revisions []Revision
}

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
		case OpChange, OpEditChange, OpDropChange:
			i := index(op.Entry)
			if i < 0 {
				return nil, fail("unknown entry %q", op.Entry)
			}
			e := &entries[i]
			e.Changes = slices.Clone(e.Changes) // the setting before stays as it was
			if err := changeOp(e, op, r.By == ByAuthor); err != nil {
				return nil, fail("%v", err)
			}
			e.Rev = n
		default:
			return nil, fail("unknown op %q", op.Op)
		}
	}
	return entries, nil
}

// changeOp applies op, one of the ops of a change, to e.
func changeOp(e *Entry, op Op, author bool) error {
	if op.Category != "" || len(op.Names) > 0 {
		return errors.New("a change changes its text, its node or its place only")
	}
	k := slices.IndexFunc(e.Changes, func(g Change) bool { return g.ID == op.Change })
	if op.Op == OpChange {
		switch {
		case e.Place != PlaceBody:
			return errors.New("a change is of a settled entry")
		case validID(op.Change) != nil || k >= 0:
			return errors.New("change needs a new change ID")
		case validID(op.From) != nil || strings.TrimSpace(op.Text) == "" || op.Place != PlaceBody && op.Place != PlaceOpen:
			return errors.New("change needs a node, text, and a place, body or open")
		}
		e.Changes = append(e.Changes, Change{ID: op.Change, From: op.From, Place: op.Place, Text: op.Text, Sources: slices.Clone(op.Sources), Author: author})
		k = len(e.Changes) - 1
	} else {
		if k < 0 {
			return fmt.Errorf("unknown change %q", op.Change)
		}
		g := &e.Changes[k]
		if g.Place == PlaceDropped {
			return errors.New("a dropped change changes no more")
		}
		if op.Op == OpDropChange {
			if op.From != "" || op.Text != "" || op.Place != "" || len(op.Sources) > 0 {
				return errors.New("drop_change drops, only")
			}
			g.Place = PlaceDropped
			return nil
		}
		switch {
		case op.From == "" && op.Text == "" && op.Place == "":
			return errors.New("edit_change changes a node, text or place")
		case op.From != "" && validID(op.From) != nil:
			return errors.New("edit_change names an invalid node")
		case op.Text != "" && strings.TrimSpace(op.Text) == "":
			return errors.New("edit_change needs text")
		case op.Place != "" && (op.Place != PlaceBody || g.Place != PlaceOpen || e.Place != PlaceBody):
			return errors.New("a change moves only as the author takes an open one of a settled entry")
		}
		if op.From != "" {
			g.From = op.From
		}
		if op.Text != "" {
			g.Text = op.Text
		}
		if op.Place != "" {
			g.Place = op.Place
		}
		g.Sources = slices.Clone(g.Sources)
		for _, src := range op.Sources {
			if !slices.Contains(g.Sources, src) {
				g.Sources = append(g.Sources, src)
			}
		}
		g.Author = g.Author || author
	}
	// Two that hold from one node would leave which of them holds to chance.
	if g := e.Changes[k]; g.Place == PlaceBody && slices.ContainsFunc(e.Changes, func(x Change) bool { return x.ID != g.ID && x.Place == PlaceBody && x.From == g.From }) {
		return errors.New("the entry changes from that node already")
	}
	return nil
}

// Order is where each node of a tree comes in reading order, folders
// before what they hold; what is not in the tree has no place.
type Order map[string]int

// Order is the tree's order of its nodes.
func (t Tree) Order() Order {
	o := Order{}
	t.Walk(func(n *Node, _ int) { o[n.ID] = len(o) })
	return o
}

// Holding are e's changes that hold, in the tree's order: those settled
// whose node is in it.
func (e Entry) Holding(o Order) []Change {
	var out []Change
	for _, g := range e.Changes {
		if _, ok := o[g.From]; ok && g.Place == PlaceBody {
			out = append(out, g)
		}
	}
	slices.SortStableFunc(out, func(a, b Change) int { return cmp.Compare(o[a.From], o[b.From]) })
	return out
}

// At is the change of e that holds at the node at: the last from it or
// before it.
func (e Entry) At(o Order, at string) (Change, bool) {
	n, ok := o[at]
	var found Change
	for _, g := range e.Holding(o) {
		if ok && o[g.From] <= n {
			found = g
		}
	}
	return found, found.ID != ""
}

// At is the setting as it stands at the node at, as the prose there is
// written to it: each entry reads as its change that holds there, or as
// itself, and has no changes; and how many changes of the body that hold
// come after it.
func (s Setting) At(o Order, at string) (Setting, int) {
	out := Setting{Entries: make([]Entry, len(s.Entries)), Revisions: s.Revisions}
	n, ahead := o[at], 0
	for i, e := range s.Entries {
		if g, ok := e.At(o, at); ok {
			e.Text = g.Text
		}
		if e.Place == PlaceBody {
			for _, g := range e.Holding(o) {
				if o[g.From] > n {
					ahead++
				}
			}
		}
		e.Changes = nil
		out.Entries[i] = e
	}
	return out, ahead
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
	p, err := s.extensionPath("lore", "setting.jsonl")
	if err != nil {
		return Setting{}, 0, err
	}
	if _, err := os.Stat(p); errors.Is(err, os.ErrNotExist) {
		return Setting{}, 0, nil
	}
	var set Setting
	seen := map[string]bool{}
	prefix, err := readJSONL(p, func(raw []byte) error {
		var r Revision
		if err := strictjson.Decode(raw, &r); err != nil {
			var k kept
			if strictjson.Decode(raw, &k) != nil || k.By != ByBookkeeper {
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
		for _, g := range e.Changes {
			used[g.ID] = true
		}
	}
	for _, rev := range set.Revisions {
		used[rev.ID] = true
	}
	if r.By == ByBookkeeper {
		return Revision{}, fmt.Errorf("by: the bookkeeper writes no more")
	}
	if r.ID == "" {
		r.ID = freshID(used)
	}
	r.Ops = slices.Clone(r.Ops)
	for k, op := range r.Ops {
		if op.Op == OpAdd && op.Entry == "" {
			r.Ops[k].Entry = freshID(used)
		}
		if op.Op == OpChange && op.Change == "" {
			r.Ops[k].Change = freshID(used)
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
	p, err := s.extensionPath("lore", "setting.jsonl")
	if err != nil {
		return Revision{}, err
	}
	if err := s.prepareExtension("lore"); err != nil {
		return Revision{}, err
	}
	if _, err := os.Stat(p); errors.Is(err, os.ErrNotExist) {
		if err := atomicWrite(p, nil, true); err != nil {
			return Revision{}, err
		}
	}
	if err := appendJSONL(p, prefix, r); err != nil {
		return r, err
	}
	s.lastRevision, s.revisionPrefix = r.ID, prefix
	return r, nil
}

// Unrevise removes id only while it is still the last revision. Earlier
// lines keep their bytes, including formatting supplied by other editors.
func (s *Store) Unrevise(id string) error {
	unlock, err := s.lock()
	if err != nil {
		return err
	}
	defer unlock()
	set, prefix, err := s.setting(nil)
	if err != nil {
		return err
	}
	if id == "" || len(set.Revisions) == 0 || set.Revisions[len(set.Revisions)-1].ID != id {
		return ErrSettingChanged
	}
	p, err := s.extensionPath("lore", "setting.jsonl")
	if err != nil {
		return err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return err
	}
	if int64(len(data)) != prefix {
		return ErrSettingChanged
	}
	end := len(data)
	if end > 0 && data[end-1] == '\n' {
		end--
	}
	start := int64(bytes.LastIndexByte(data[:end], '\n') + 1)
	if s.lastRevision == id && s.revisionPrefix <= start && bytes.Equal(data[s.revisionPrefix:start], []byte{'\n'}) {
		start = s.revisionPrefix
	}
	if err := atomicWrite(p, data[:start], false); err != nil {
		return err
	}
	s.lastRevision, s.revisionPrefix = "", 0
	return nil
}

// checkChapters checks that each source in a chapter quotes its text as it
// is now, and that a change starts at a node in the tree; later edits of
// the chapter leave the source as it was, and the node may go to the
// trash.
func (s *Store) checkChapters(r Revision) error {
	var t *Tree
	read := func() error {
		if t != nil {
			return nil
		}
		tree, err := s.tree()
		t = &tree
		return err
	}
	for k, op := range r.Ops {
		if op.From != "" {
			if err := read(); err != nil {
				return err
			}
			if _, ok := t.Node(op.From); !ok {
				return fmt.Errorf("ops[%d]: unknown node %q", k, op.From)
			}
		}
		for _, src := range op.Sources {
			if src.Chapter == "" || src.Message != "" {
				continue // a message's quote of a chapter is checked in the message
			}
			if err := read(); err != nil {
				return err
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
