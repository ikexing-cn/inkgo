package ai

import (
	"strconv"
	"strings"

	"writer/ext/ai/aiapi"
	"writer/tokens"
)

// ServiceForm owns one editable model service. App coordinates persistence
// and requests; loading or closing a form invalidates its pending catalog.
type ServiceForm struct {
	ProviderID string
	Provider   aiapi.Provider
	Key        string
	Limits     []LimitRow
	LimitKeys  int
	Edit       string // an ID, "+" for a new service, or empty when closed
	Shown      string // edit in the last frame, for scrolling a new form into view
	Generation uint64
}

// Close runs on the frame thread.
func (f *ServiceForm) Close() {
	// Row keys must not be reused while UI state from an old form may exist.
	*f = ServiceForm{LimitKeys: f.LimitKeys, Generation: f.Generation + 1}
}

// New runs on the frame thread.
func (f *ServiceForm) New() {
	f.Close()
	f.Provider.Protocol, f.Edit = "chat", "+"
}

func (f *ServiceForm) load(p aiapi.Provider) {
	f.Close()
	f.ProviderID, f.Provider = p.ID, p
	f.setModels(p.Models)
}

func (f *ServiceForm) setModels(models []aiapi.Model) {
	f.Limits = nil
	for _, m := range models {
		f.Limits = append(f.Limits, f.LimitRow(m))
	}
}

// LimitRow runs on the frame thread.
func (f *ServiceForm) LimitRow(m aiapi.Model) LimitRow {
	f.LimitKeys++
	return LimitRow{f.LimitKeys, m.ID, tokens.Format(m.Window), tokens.Format(m.Output), strconv.Itoa(m.CompactAt)}
}

// LimitRow is a model of the service in the form, as typed; key keeps
// its fields its own as rows above it go.
type LimitRow struct {
	Key                    int
	ID, Window, Output, At string
}

// models reads the rows of the form that have an ID. It names the first
// model that breaks a rule, the rules taken in turn.
func (f *ServiceForm) models() ([]aiapi.Model, error) {
	var out []aiapi.Model
	for _, r := range f.Limits {
		id := strings.TrimSpace(r.ID)
		if id == "" {
			continue
		}
		window, _ := tokens.Parse(r.Window)
		output, _ := tokens.Parse(r.Output)
		at, err := strconv.Atoi(strings.TrimSpace(r.At))
		if err != nil {
			at = -1
		}
		out = append(out, aiapi.Model{ID: id, Window: window, Output: output, CompactAt: at})
	}
	for _, rule := range []struct {
		bad  func(m aiapi.Model) bool
		rule string
	}{
		{func(m aiapi.Model) bool { return m.Window == 0 || m.Output == 0 }, "numbers"},
		{func(m aiapi.Model) bool { return m.CompactAt < 0 }, "integer"},
		{func(m aiapi.Model) bool { return m.Output >= m.Window }, "output"},
		{func(m aiapi.Model) bool { return m.CompactAt < 50 || m.CompactAt > 95 }, "threshold"},
	} {
		for _, m := range out {
			if rule.bad(m) {
				return nil, &aiapi.SetupError{Err: &modelRuleError{id: m.ID, rule: rule.rule}}
			}
		}
	}
	return out, nil
}

// value is the service the form would save, and what keeps it
// from being saved, by field. A new service left without an ID takes one
// from its name; it may not take the ID of another, whose place it would
// take.
func (f *ServiceForm) value(settings aiapi.Settings) (aiapi.Provider, aiapi.Problems) {
	p := f.Provider
	p.ID = strings.TrimSpace(p.ID)
	if f.ProviderID == "" && p.ID == "" {
		p.ID = IdFrom(p.Name)
	}
	models, err := f.models()
	p.Models = models
	bad := settings.Check(p)

	if err != nil {
		bad.Models = err
	}
	if f.ProviderID == "" {
		for _, q := range settings.Providers() {
			if q.ID == p.ID {
				bad.ID = &providerIDTakenError{name: providerLabel(q)}
			}
		}
	}
	return p, bad
}

// IdFrom is an ID made from a name: its Latin letters in lower case, and
// the digits, - and _ after the first of them, a space as -.
func IdFrom(name string) string {
	var id []rune
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', len(id) > 0 && (r >= '0' && r <= '9' || r == '-' || r == '_'):
			id = append(id, r)
		case len(id) > 0 && r == ' ' && id[len(id)-1] != '-':
			id = append(id, '-')
		}
	}
	return strings.TrimRight(string(id), "-_")
}
