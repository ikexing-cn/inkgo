package workspace

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"writer/internal/strictjson"
)

// Plan keeps the author's story order, independently of the chapter tree.
// Both slices retain their on-disk order. Times are words, never dates.
type Plan struct {
	Schema int        `json:"schema"`
	Lines  []PlotLine `json:"lines"`
	Cards  []Card     `json:"cards"`
}

type PlotLine struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Card struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	When    string `json:"when"`
	About   string `json:"about"`
	Chapter string `json:"chapter"`
	Line    string `json:"line"`
}

// TimeColumn groups adjacent cards with the same nonempty time.
type TimeColumn struct {
	When  string
	Cards []Card
}

func (p Plan) Columns() []TimeColumn {
	var cols []TimeColumn
	for _, c := range p.Cards {
		if n := len(cols); n > 0 && c.When != "" && cols[n-1].When == c.When {
			cols[n-1].Cards = append(cols[n-1].Cards, c)
		} else {
			cols = append(cols, TimeColumn{c.When, []Card{c}})
		}
	}
	return cols
}

func (p Plan) Card(id string) (Card, bool) {
	i := slices.IndexFunc(p.Cards, func(c Card) bool { return c.ID == id })
	if i < 0 {
		return Card{}, false
	}
	return p.Cards[i], true
}

func (p Plan) validate() error {
	if err := validSchema(p.Schema); err != nil {
		return err
	}
	ids, lines := map[string]bool{}, map[string]bool{}
	for _, l := range p.Lines {
		if validID(l.ID) != nil || ids[l.ID] || strings.TrimSpace(l.Name) == "" {
			return fmt.Errorf("lines: invalid or duplicate id/name %q", l.ID)
		}
		ids[l.ID], lines[l.ID] = true, true
	}
	for _, c := range p.Cards {
		if validID(c.ID) != nil || ids[c.ID] || strings.TrimSpace(c.Title) == "" {
			return fmt.Errorf("cards: invalid or duplicate id/title %q", c.ID)
		}
		if c.Chapter != "" && validID(c.Chapter) != nil {
			return fmt.Errorf("cards[%s].chapter: invalid ID", c.ID)
		}
		if c.Line != "" && !lines[c.Line] {
			return fmt.Errorf("cards[%s].line: unknown line %q", c.ID, c.Line)
		}
		ids[c.ID] = true
	}
	return nil
}

func (s *Store) plan() (Plan, error) {
	p, err := s.path("outline.json")
	if err != nil {
		return Plan{}, err
	}
	b, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return Plan{Schema: Schema}, nil
	}
	if err != nil {
		return Plan{}, err
	}
	var out Plan
	if err = strictjson.Decode(b, &out); err == nil {
		err = out.validate()
	}
	if err != nil {
		return Plan{}, &PlanError{Path: p, Line: lineOf(b, err), Err: err}
	}
	return out, nil
}

// PlanError is an outline.json that does not read. Line is the line, from
// 1, where the JSON itself breaks, and 0 when it reads as JSON but not as an
// outline.
type PlanError struct {
	Path string
	Line int
	Err  error
}

func (e *PlanError) Error() string { return e.Path + ": " + e.Err.Error() }
func (e *PlanError) Unwrap() error { return e.Err }

// lineOf finds the line of b where decoding it failed with err.
func lineOf(b []byte, err error) int {
	var syntax *json.SyntaxError
	var kind *json.UnmarshalTypeError
	at := int64(-1)
	switch {
	case errors.As(err, &syntax):
		at = syntax.Offset
	case errors.As(err, &kind):
		at = kind.Offset
	}
	if at < 0 {
		return 0
	}
	return 1 + bytes.Count(b[:min(int(at), len(b))], []byte("\n"))
}

func (s *Store) writePlan(p Plan) error {
	if err := p.validate(); err != nil {
		return err
	}
	path, err := s.path("outline.json")
	if err != nil {
		return err
	}
	if p.Cards == nil {
		p.Cards = []Card{}
	}
	if p.Lines == nil {
		p.Lines = []PlotLine{}
	}
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(path, append(b, '\n'), false)
}

func (s *Store) Plan() (Plan, error) {
	unlock, err := s.lock()
	if err != nil {
		return Plan{}, err
	}
	defer unlock()
	return s.plan()
}

// editPlan reads under the store's lock, so a failed hand edit is never
// overwritten by an action in an already-open window.
func (s *Store) editPlan(fn func(*Plan) error) error {
	unlock, err := s.lock()
	if err != nil {
		return err
	}
	defer unlock()
	p, err := s.plan()
	if err != nil {
		return err
	}
	if err = fn(&p); err != nil {
		return err
	}
	return s.writePlan(p)
}

func placeAt(cards []Card, order Order, chapter, after string) int {
	if after != "" {
		if i := slices.IndexFunc(cards, func(c Card) bool { return c.ID == after }); i >= 0 {
			return i + 1
		}
	}
	if chapter == "" {
		return len(cards)
	}
	mine, ok := order[chapter]
	if !ok {
		return len(cards)
	}
	at := 0
	for i, c := range cards {
		if n, ok := order[c.Chapter]; ok && n <= mine {
			at = i + 1
		}
	}
	return at
}

// SaveCard adds a new card, or edits an existing one without moving it.
// A card newly assigned to a live chapter follows that chapter and all
// earlier chapters. after, for a new card, overrides that position.
func (s *Store) SaveCard(c Card, after string) (Card, error) {
	err := s.editPlan(func(p *Plan) error {
		t, err := s.tree()
		if err != nil {
			return err
		}
		order := t.Order()
		c.Title, c.When = strings.TrimSpace(c.Title), strings.TrimSpace(c.When)
		at := placeAt(p.Cards, order, c.Chapter, after)
		if c.ID == "" {
			c.ID = NewID()
		} else {
			i := slices.IndexFunc(p.Cards, func(x Card) bool { return x.ID == c.ID })
			if i < 0 {
				return fmt.Errorf("unknown card %q", c.ID)
			}
			old := p.Cards[i]
			p.Cards = slices.Delete(p.Cards, i, i+1)
			at = i
			_, liveTarget := order[c.Chapter]
			if _, assigned := order[old.Chapter]; !assigned && liveTarget {
				at = placeAt(p.Cards, order, c.Chapter, "")
			}
		}
		p.Cards = slices.Insert(p.Cards, at, c)
		return nil
	})
	return c, err
}

func (s *Store) DeleteCard(id string) error {
	return s.editPlan(func(p *Plan) error {
		p.Cards = slices.DeleteFunc(p.Cards, func(c Card) bool { return c.ID == id })
		return nil
	})
}

// PlaceCard moves a card to a chapter cell or the tray. A nil line keeps
// its plot line when dropped in the tray.
func (s *Store) PlaceCard(id, chapter string, line *string) error {
	return s.editPlan(func(p *Plan) error {
		i := slices.IndexFunc(p.Cards, func(c Card) bool { return c.ID == id })
		if i < 0 {
			return fmt.Errorf("unknown card %q", id)
		}
		t, err := s.tree()
		if err != nil {
			return err
		}
		order := t.Order()
		c := p.Cards[i]
		_, was := order[c.Chapter]
		c.Chapter = chapter
		if line != nil {
			c.Line = *line
		}
		p.Cards = slices.Delete(p.Cards, i, i+1)
		at := i
		if !was && chapter != "" {
			at = placeAt(p.Cards, order, chapter, "")
		}
		p.Cards = slices.Insert(p.Cards, at, c)
		return nil
	})
}

// DropCard puts a card last in column, taking its time and lane; -1 is
// the "after" column, which keeps the card's time.
func (s *Store) DropCard(id string, column int, line string) error {
	return s.editPlan(func(p *Plan) error {
		i := slices.IndexFunc(p.Cards, func(c Card) bool { return c.ID == id })
		if i < 0 {
			return fmt.Errorf("unknown card %q", id)
		}
		c := p.Cards[i]
		cols := p.Columns()
		after, stay := "", false
		if column != -1 {
			if column < 0 || column >= len(cols) {
				return fmt.Errorf("unknown time column %d", column)
			}
			col := cols[column]
			c.When = col.When
			stay = len(col.Cards) == 1 && col.Cards[0].ID == id
			for _, x := range col.Cards {
				if x.ID != id {
					after = x.ID
				}
			}
		}
		c.Line = line
		p.Cards = slices.Delete(p.Cards, i, i+1)
		at := len(p.Cards)
		if stay {
			at = i
		} else if after != "" {
			at = slices.IndexFunc(p.Cards, func(c Card) bool { return c.ID == after }) + 1
		}
		p.Cards = slices.Insert(p.Cards, at, c)
		return nil
	})
}

func (s *Store) SaveTime(column int, when string) error {
	return s.editPlan(func(p *Plan) error {
		cols := p.Columns()
		if column < 0 || column >= len(cols) {
			return fmt.Errorf("unknown time column %d", column)
		}
		for i := range p.Cards {
			if slices.ContainsFunc(cols[column].Cards, func(c Card) bool { return c.ID == p.Cards[i].ID }) {
				p.Cards[i].When = strings.TrimSpace(when)
			}
		}
		return nil
	})
}

func (s *Store) SaveLine(l PlotLine) (PlotLine, error) {
	err := s.editPlan(func(p *Plan) error {
		l.Name = strings.TrimSpace(l.Name)
		if l.ID == "" {
			l.ID = NewID()
			p.Lines = append(p.Lines, l)
		} else {
			i := slices.IndexFunc(p.Lines, func(x PlotLine) bool { return x.ID == l.ID })
			if i < 0 {
				return fmt.Errorf("unknown plot line %q", l.ID)
			}
			p.Lines[i] = l
		}
		return nil
	})
	return l, err
}

func (s *Store) MoveLine(id string, delta int) error {
	return s.editPlan(func(p *Plan) error {
		i := slices.IndexFunc(p.Lines, func(l PlotLine) bool { return l.ID == id })
		j := i + delta
		if i < 0 || j < 0 || j >= len(p.Lines) {
			return fmt.Errorf("invalid plot line move")
		}
		p.Lines[i], p.Lines[j] = p.Lines[j], p.Lines[i]
		return nil
	})
}

func (s *Store) DeleteLine(id string) error {
	return s.editPlan(func(p *Plan) error {
		p.Lines = slices.DeleteFunc(p.Lines, func(l PlotLine) bool { return l.ID == id })
		for i := range p.Cards {
			if p.Cards[i].Line == id {
				p.Cards[i].Line = ""
			}
		}
		return nil
	})
}
