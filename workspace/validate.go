package workspace

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

var idPattern = regexp.MustCompile(`^[0-9]{8}-[0-9]{4}-[0-9a-f]{4}$`)

func validID(id string) error {
	if !idPattern.MatchString(id) {
		return fmt.Errorf("id: invalid ID %q", id)
	}
	return nil
}
func validSchema(n int) error {
	if n != Schema {
		return fmt.Errorf("schema: unsupported version %d (supported: %d)", n, Schema)
	}
	return nil
}

// validEffort mirrors model.Efforts; workspace stays free of model clients.
func validEffort(e string) bool {
	switch e {
	case "", "low", "high", "max":
		return true
	}
	return false
}

func validStatus(s Status) bool {
	switch s {
	case Unprocessed, Kept, Deferred, Rejected, Adopted, Superseded:
		return true
	}
	return false
}
func transition(from, to Status) error {
	if !validStatus(to) {
		return fmt.Errorf("status: invalid %q", to)
	}
	if from == to {
		return nil
	}
	if from == Adopted {
		return fmt.Errorf("status: adopted ideas are protected until their setting is withdrawn")
	}
	if from == Rejected && to != Deferred {
		return fmt.Errorf("status: rejected ideas must return to deferred")
	}
	return nil
}
func validateWork(w Work) error {
	if err := validSchema(w.Schema); err != nil {
		return err
	}
	if err := validID(w.ID); err != nil {
		return err
	}
	if strings.TrimSpace(w.Name) == "" {
		return fmt.Errorf("name: must not be empty")
	}
	if w.TokenBudget <= 0 {
		return fmt.Errorf("token_budget: must be positive")
	}
	seen := map[string]bool{}
	for _, c := range w.Categories {
		if strings.TrimSpace(c.Name) == "" || strings.TrimSpace(c.Target) == "" {
			return fmt.Errorf("categories: name and target must not be empty")
		}
		if seen[c.Name] {
			return fmt.Errorf("categories: duplicate %q", c.Name)
		}
		seen[c.Name] = true
	}
	if !seen[Other] {
		return fmt.Errorf("categories: must include %q", Other)
	}
	for name, sel := range map[string]Selection{"discussion": w.Discussion, "bookkeeping": w.Bookkeeping} {
		if (sel.Provider == "") != (sel.Model == "") {
			return fmt.Errorf("%s: provider and model must be specified together", name)
		}
		if !validEffort(sel.Effort) {
			return fmt.Errorf("%s.effort: invalid %q", name, sel.Effort)
		}
	}
	return nil
}
func validateIdea(i Idea, w Work) error {
	if err := validSchema(i.Schema); err != nil {
		return err
	}
	if err := validID(i.ID); err != nil {
		return err
	}
	if !validStatus(i.Status) {
		return fmt.Errorf("status: invalid %q", i.Status)
	}
	found := false
	for _, c := range w.Categories {
		if c.Name == i.Category {
			found = true
		}
	}
	if !found {
		return fmt.Errorf("category: unknown %q", i.Category)
	}
	if i.Source != SourceHuman && i.Source != SourceAI {
		return fmt.Errorf("source: must be human or ai")
	}
	if strings.TrimSpace(i.Text) == "" {
		return fmt.Errorf("text: must not be empty")
	}
	if i.Verified && (!i.Fact || strings.TrimSpace(i.FactSource) == "") {
		return fmt.Errorf("verified: requires fact and fact_source")
	}
	if i.Created.IsZero() || i.Updated.IsZero() || i.Updated.Before(i.Created) {
		return fmt.Errorf("created/updated: missing or reversed timestamps")
	}
	if i.Origin.Span.Start < 0 || i.Origin.Span.End < i.Origin.Span.Start {
		return fmt.Errorf("origin.span: invalid rune range")
	}
	if i.Origin.Session == "" && i.Origin.Message == "" {
		if i.Origin.Span != (Span{}) {
			return fmt.Errorf("origin.span: range requires a message")
		}
	} else {
		if err := validID(i.Origin.Session); err != nil {
			return fmt.Errorf("origin.session: %w", err)
		}
		if err := validID(i.Origin.Message); err != nil {
			return fmt.Errorf("origin.message: %w", err)
		}
	}
	for field, ids := range map[string][]string{"conflicts": i.Conflicts, "supersedes": i.Supersedes} {
		seen := map[string]bool{}
		for _, id := range ids {
			if err := validID(id); err != nil {
				return fmt.Errorf("%s: %w", field, err)
			}
			if id == i.ID || seen[id] {
				return fmt.Errorf("%s: self or duplicate reference %q", field, id)
			}
			seen[id] = true
		}
	}
	for _, s := range i.Similar {
		if err := validID(s.ID); err != nil {
			return fmt.Errorf("similar: %w", err)
		}
		if s.ID == i.ID || strings.TrimSpace(s.Difference) == "" {
			return fmt.Errorf("similar: requires another idea and difference")
		}
	}
	for n, h := range i.History {
		if h.At.IsZero() || !validStatus(h.From) || !validStatus(h.To) {
			return fmt.Errorf("history[%d]: invalid at/from/to", n)
		}
		if err := transition(h.From, h.To); err != nil {
			return fmt.Errorf("history[%d]: %w", n, err)
		}
		if n > 0 && (i.History[n-1].To != h.From || h.At.Before(i.History[n-1].At)) {
			return fmt.Errorf("history[%d]: discontinuous history", n)
		}
	}
	if len(i.History) > 0 && i.History[len(i.History)-1].To != i.Status {
		return fmt.Errorf("history: last to must equal status")
	}
	return nil
}
func (s *Store) validateOrigin(i Idea, sc *scan) error {
	if i.Origin.Session == "" {
		return nil
	}
	events, err := sc.events(s, i.Origin.Session)
	if err != nil {
		return fmt.Errorf("origin.session: %w", err)
	}
	for _, e := range events {
		if e.ID == i.Origin.Message {
			if e.Kind != KindMessage {
				return fmt.Errorf("origin.message: must refer to message")
			}
			if (i.Source == SourceHuman && e.Role != RoleUser) || (i.Source == SourceAI && e.Role != RoleAssistant) {
				return fmt.Errorf("origin.message: role does not match source")
			}
			if i.Origin.Span.End > utf8.RuneCountInString(e.Text) || i.Origin.Span.Start == i.Origin.Span.End {
				return fmt.Errorf("origin.span: range outside message or empty")
			}
			return nil
		}
	}
	return fmt.Errorf("origin.message: unknown %q", i.Origin.Message)
}
func validateEvent(e Event) error {
	if err := validSchema(e.Schema); err != nil {
		return err
	}
	if err := validID(e.ID); err != nil {
		return err
	}
	if e.At.IsZero() {
		return fmt.Errorf("at: missing timestamp")
	}
	switch e.Kind {
	case KindMessage:
		if e.Role != RoleUser && e.Role != RoleAssistant {
			return fmt.Errorf("role: invalid message role %q", e.Role)
		}
	case KindBookkeeping:
		if e.Role != "" {
			return fmt.Errorf("role: invalid bookkeeping role %q", e.Role)
		}
	default:
		return fmt.Errorf("kind: invalid %q", e.Kind)
	}
	if !validEffort(e.Effort) {
		return fmt.Errorf("effort: invalid %q", e.Effort)
	}
	if e.Kind == KindMessage && e.Role == RoleUser && (e.Model != "" || e.Effort != "" || e.Reasoning != "") {
		return fmt.Errorf("model: only what a model wrote records one")
	}
	if e.ReplyTo != "" {
		if err := validID(e.ReplyTo); err != nil {
			return fmt.Errorf("reply_to: %w", err)
		}
	}
	for _, id := range e.IdeaIDs {
		if err := validID(id); err != nil {
			return fmt.Errorf("idea_ids: %w", err)
		}
	}
	return nil
}
