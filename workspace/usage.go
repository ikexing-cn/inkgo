package workspace

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

// usageFile is what the work has spent on the AI, beside work.yaml, so
// that a backup takes it along.
const usageFile = "usage.json"

// Spent is what the work's requests to one model took and gave, all told,
// and what they cost by the prices built in when each was made. Cost
// counts only the requests that had a price.
type Spent struct {
	Model    string `json:"model"` // provider/model
	Requests int    `json:"requests"`
	Usage
	Cost     float64 `json:"cost,omitempty"`
	Currency string  `json:"currency,omitempty"`
}

type usageDoc struct {
	Models []Spent `json:"models"`
}

// Spend adds a request to model, what it took and gave and what it cost in
// currency, to what the work has spent. A cost in another currency than
// the model's so far is not added.
func (s *Store) Spend(model string, u Usage, cost float64, currency string) error {
	if strings.TrimSpace(model) == "" {
		return errors.New("usage: a request without its model")
	}
	unlock, err := s.lock()
	if err != nil {
		return err
	}
	defer unlock()
	all, err := s.spent()
	if err != nil {
		return err
	}
	n := 0
	for n < len(all) && all[n].Model != model {
		n++
	}
	if n == len(all) {
		all = append(all, Spent{Model: model})
	}
	m := &all[n]
	m.Requests++
	m.Input += u.Input
	m.CacheRead += u.CacheRead
	m.CacheWrite += u.CacheWrite
	m.Output += u.Output
	m.Reasoning += u.Reasoning
	if currency != "" && (m.Currency == "" || m.Currency == currency) {
		m.Cost += max(cost, 0)
		m.Currency = currency
	}
	b, err := json.MarshalIndent(usageDoc{Models: all}, "", "  ")
	if err != nil {
		return err
	}
	p, err := s.path(usageFile)
	if err != nil {
		return err
	}
	return atomicWrite(p, append(b, '\n'), false)
}

// Spent is what the work has spent on each model, in the order first used.
func (s *Store) Spent() ([]Spent, error) {
	unlock, err := s.lock()
	if err != nil {
		return nil, err
	}
	defer unlock()
	return s.spent()
}

func (s *Store) spent() ([]Spent, error) {
	p, err := s.path(usageFile)
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var doc usageDoc
	if err = decodeStrict(b, &doc); err != nil {
		return nil, fmt.Errorf("%s: %w", p, err)
	}
	seen := map[string]bool{}
	for i, m := range doc.Models {
		bad := ""
		switch {
		case strings.TrimSpace(m.Model) == "":
			bad = "model is empty"
		case seen[m.Model]:
			bad = fmt.Sprintf("model %q is listed twice", m.Model)
		case m.Requests < 0 || m.Input < 0 || m.CacheRead < 0 || m.CacheWrite < 0 || m.Output < 0 || m.Reasoning < 0 || m.Cost < 0:
			bad = "counts cannot be negative"
		case m.Cost > 0 && m.Currency == "":
			bad = "cost needs its currency"
		}
		if bad != "" {
			return nil, fmt.Errorf("%s: models[%d]: %s", p, i, bad)
		}
		seen[m.Model] = true
	}
	return doc.Models, nil
}
