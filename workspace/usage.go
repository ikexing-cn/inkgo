package workspace

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// usageFile is what versions before the AI record counted the work spent,
// by model; the record takes it over once and leaves it where it is.
const usageFile = "usage.json"

// spent is what the work's requests to one model took and gave, all told,
// as usage.json kept it.
type spent struct {
	Model    string `json:"model"` // provider/model
	Requests int    `json:"requests"`
	Usage
	Cost     float64 `json:"cost,omitempty"`
	Currency string  `json:"currency,omitempty"`
}

type usageDoc struct {
	Models []spent `json:"models"`
}

// before is what usage.json counted, a line of FeatureBefore for each
// model in the order first used, or nothing without the file.
func (s *Store) before() ([]Record, error) {
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
	var out []Record
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
		out = append(out, Record{Schema: Schema, Feature: FeatureBefore, Model: m.Model, Usage: m.Usage, Requests: m.Requests,
			Cost: m.Cost, Currency: m.Currency, Outcome: OutcomeDone})
	}
	return out, nil
}
