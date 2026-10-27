package desk

import (
	"errors"
	"strconv"
	"strings"

	"writer/aiport"
	"writer/tokens"
)

// serviceForm owns one editable model service. App coordinates persistence
// and requests; loading or closing a form invalidates its pending catalog.
type serviceForm struct {
	providerID string
	provider   aiport.Provider
	key        string
	limits     []limitRow
	limitKeys  int
	edit       string // an ID, "+" for a new service, or empty when closed
	shown      string // edit in the last frame, for scrolling a new form into view
	generation uint64
}

func (f *serviceForm) close() {
	// Row keys must not be reused while UI state from an old form may exist.
	*f = serviceForm{limitKeys: f.limitKeys, generation: f.generation + 1}
}

func (f *serviceForm) new() {
	f.close()
	f.provider.Protocol, f.edit = "chat", "+"
}

func (f *serviceForm) load(p aiport.Provider) {
	f.close()
	f.providerID, f.provider = p.ID, p
	f.setModels(p.Models)
}

func (f *serviceForm) setModels(models []aiport.Model) {
	f.limits = nil
	for _, m := range models {
		f.limits = append(f.limits, f.limitRow(m))
	}
}

func (f *serviceForm) limitRow(m aiport.Model) limitRow {
	f.limitKeys++
	return limitRow{f.limitKeys, m.ID, tokens.Format(m.Window), tokens.Format(m.Output), strconv.Itoa(m.CompactAt)}
}

// limitRow is a model of the service in the form, as typed; key keeps
// its fields its own as rows above it go.
type limitRow struct {
	key                    int
	id, window, output, at string
}

// models reads the rows of the form that have an ID. It names the first
// model that breaks a rule, the rules taken in turn.
func (f *serviceForm) models() ([]aiport.Model, error) {
	var out []aiport.Model
	for _, r := range f.limits {
		id := strings.TrimSpace(r.id)
		if id == "" {
			continue
		}
		window, _ := tokens.Parse(r.window)
		output, _ := tokens.Parse(r.output)
		at, err := strconv.Atoi(strings.TrimSpace(r.at))
		if err != nil {
			at = -1
		}
		out = append(out, aiport.Model{ID: id, Window: window, Output: output, CompactAt: at})
	}
	for _, rule := range []struct {
		bad func(m aiport.Model) bool
		why string
	}{
		{func(m aiport.Model) bool { return m.Window == 0 || m.Output == 0 }, "的上下文上限和最大输出须为正整数，可写成 128K、1M"},
		{func(m aiport.Model) bool { return m.CompactAt < 0 }, "的压缩阈值须为整数"},
		{func(m aiport.Model) bool { return m.Output >= m.Window }, "的最大输出须小于上下文上限"},
		{func(m aiport.Model) bool { return m.CompactAt < 50 || m.CompactAt > 95 }, "的压缩阈值须在 50% 到 95% 之间"},
	} {
		for _, m := range out {
			if rule.bad(m) {
				return nil, &aiport.SetupError{Err: errors.New(m.ID + " " + rule.why)}
			}
		}
	}
	return out, nil
}

// value is the service the form would save, and what keeps it
// from being saved, by field. A new service left without an ID takes one
// from its name; it may not take the ID of another, whose place it would
// take.
func (f *serviceForm) value(settings aiport.Settings) (aiport.Provider, aiport.Problems) {
	p := f.provider
	p.ID = strings.TrimSpace(p.ID)
	if f.providerID == "" && p.ID == "" {
		p.ID = idFrom(p.Name)
	}
	models, err := f.models()
	p.Models = models
	bad := settings.Check(p)
	if err != nil {
		bad.Models = err.Error()
	}
	if f.providerID == "" {
		for _, q := range settings.Providers() {
			if q.ID == p.ID {
				bad.ID = "该 ID 已被「" + providerLabel(q) + "」使用，请换一个"
			}
		}
	}
	return p, bad
}

// idFrom is an ID made from a name: its Latin letters in lower case, and
// the digits, - and _ after the first of them, a space as -.
func idFrom(name string) string {
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
