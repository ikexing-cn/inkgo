package workspace

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var idPattern = regexp.MustCompile(`^[0-9]{8}-[0-9]{4}-[0-9a-f]{4}$`)

func validID(id string) error {
	if !idPattern.MatchString(id) {
		return fmt.Errorf("id: invalid ID %q", id)
	}
	return nil
}
func validSchema(n int) error {
	if n == 1 {
		return fmt.Errorf("schema: 这是旧版本的作品（schema 1，思路卡片），没有迁移，请新建作品")
	}
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
	for name, sel := range map[string]Selection{"discussion": w.Discussion, "writing": w.Writing} {
		if (sel.Provider == "") != (sel.Model == "") {
			return fmt.Errorf("%s: provider and model must be specified together", name)
		}
		if !validEffort(sel.Effort) {
			return fmt.Errorf("%s.effort: invalid %q", name, sel.Effort)
		}
	}
	return nil
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
	switch e.Role {
	case RoleUser, RoleAssistant:
		if len(e.Calls) > 0 || e.Call != "" || e.Failed || e.Revision != "" {
			return fmt.Errorf("calls: only a step of a reply calls tools")
		}
	case RoleCall:
		if e.ReplyTo == "" || len(e.Calls) == 0 || e.Call != "" || e.Failed || e.Revision != "" {
			return fmt.Errorf("calls: a step answers the author's words and calls tools")
		}
		for _, c := range e.Calls {
			if c.ID == "" || c.Name == "" || !json.Valid(c.Input) {
				return fmt.Errorf("calls: a call has an ID, a name and its input as JSON")
			}
		}
	case RoleResult:
		if e.ReplyTo == "" || e.Call == "" || len(e.Calls) > 0 {
			return fmt.Errorf("call: a result answers the author's words and names its call")
		}
		if e.Revision != "" {
			if err := validID(e.Revision); err != nil {
				return fmt.Errorf("revision: %w", err)
			}
		}
	default:
		return fmt.Errorf("role: invalid message role %q", e.Role)
	}
	if e.Capped && e.Role != RoleAssistant {
		return fmt.Errorf("capped: only a reply is capped")
	}
	if !validEffort(e.Effort) {
		return fmt.Errorf("effort: invalid %q", e.Effort)
	}
	if e.Role == RoleUser && (e.Model != "" || e.Effort != "" || e.Reasoning != "" || len(e.Context) > 0 || len(e.Omitted) > 0) {
		return fmt.Errorf("model: only what a model wrote records one")
	}
	if e.Role != RoleUser && (e.Chapter != "" || len(e.Quotes) > 0) {
		return fmt.Errorf("quotes: only the author quotes the prose")
	}
	if e.Chapter != "" {
		if err := validID(e.Chapter); err != nil {
			return fmt.Errorf("chapter: %w", err)
		}
	}
	for _, q := range e.Quotes {
		if err := validID(q.Chapter); err != nil {
			return fmt.Errorf("quotes: %w", err)
		}
		if strings.TrimSpace(q.Text) == "" {
			return fmt.Errorf("quotes: empty")
		}
	}
	if e.ReplyTo != "" {
		if err := validID(e.ReplyTo); err != nil {
			return fmt.Errorf("reply_to: %w", err)
		}
	}
	return nil
}
