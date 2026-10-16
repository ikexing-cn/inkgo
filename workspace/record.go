package workspace

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

// logFile is the AI record of the work, beside work.yaml, so that a backup
// takes it along: a line for each request made of a model.
const logFile = "ai-log.jsonl"

// The features a request is made for, as the AI record counts them.
const (
	FeatureTalk     = "talk"
	FeatureDraft    = "draft"
	FeatureRewrite  = "rewrite"
	FeatureContinue = "continue"
	FeatureCheck    = "check"
	// FeatureBefore is what versions before the record counted of a
	// model, all its requests in one line, with no time, chapter or talk.
	FeatureBefore = "before"
)

// How a request ended.
const (
	OutcomeDone      = "done"
	OutcomeCancelled = "cancelled"
	OutcomeFailed    = "failed"
)

// Record is a request the work made of a model: what it was for, the
// parts it sent and their tokens as estimated, what the provider counted
// and what that cost by the prices built in, and how it ended. The text
// sent is not kept: it is in the work.
type Record struct {
	Schema  int       `json:"schema"`
	At      time.Time `json:"at,omitzero"`
	Took    int64     `json:"took_ms,omitempty"` // milliseconds
	Feature string    `json:"feature"`
	Chapter string    `json:"chapter,omitempty"`
	Talk    string    `json:"talk,omitempty"`
	Model   string    `json:"model"` // provider/model
	Effort  string    `json:"effort,omitempty"`
	Parts   []Part    `json:"parts,omitempty"`
	Usage   Usage     `json:"usage"`
	// Requests is how many requests a line of FeatureBefore sums.
	Requests int     `json:"requests,omitempty"`
	Cost     float64 `json:"cost,omitempty"`
	Currency string  `json:"currency,omitempty"`
	Outcome  string  `json:"outcome"`
	Error    string  `json:"error,omitempty"`
}

// Count is how many requests r stands for.
func (r Record) Count() int {
	if r.Feature == FeatureBefore {
		return r.Requests
	}
	return 1
}

// Purpose is what a request is made for, as the code making it knows:
// the AI record writes it with the request.
type Purpose struct {
	Feature, Chapter, Talk string
	Parts                  []Part
}

// ErrEnough is the cause the maker of a request cancels it with once it
// has all it wants: the record counts the request done.
var ErrEnough = errors.New("enough")

type purposeKey struct{}

// WithPurpose is ctx carrying p to the requests made with it.
func WithPurpose(ctx context.Context, p Purpose) context.Context {
	return context.WithValue(ctx, purposeKey{}, p)
}

// PurposeOf is the purpose ctx carries, or none.
func PurposeOf(ctx context.Context) Purpose {
	p, _ := ctx.Value(purposeKey{}).(Purpose)
	return p
}

// Log adds r to the AI record. The first time, what an older version
// counted in usage.json goes before it, a line for each model; that file
// is not read again.
func (s *Store) Log(r Record) error {
	if r.Schema == 0 {
		r.Schema = Schema
	}
	if err := validRecord(r); err != nil {
		return err
	}
	unlock, err := s.lock()
	if err != nil {
		return err
	}
	defer unlock()
	p, err := s.path(logFile)
	if err != nil {
		return err
	}
	_, prefix, err := s.records()
	if errors.Is(err, os.ErrNotExist) {
		before, err := s.before()
		if err != nil {
			return err
		}
		var lines bytes.Buffer
		for _, x := range append(before, r) {
			line, err := json.Marshal(x)
			if err != nil {
				return err
			}
			lines.Write(append(line, '\n'))
		}
		return atomicWrite(p, lines.Bytes(), true)
	}
	if err != nil {
		return err
	}
	return appendJSONL(p, prefix, r)
}

// Records is the AI record, the oldest first: with none kept yet, what an
// older version counted in usage.json.
func (s *Store) Records() ([]Record, error) {
	unlock, err := s.lock()
	if err != nil {
		return nil, err
	}
	defer unlock()
	all, _, err := s.records()
	if errors.Is(err, os.ErrNotExist) {
		return s.before()
	}
	return all, err
}

// records reads the AI record and the length of its whole lines.
func (s *Store) records() ([]Record, int64, error) {
	p, err := s.path(logFile)
	if err != nil {
		return nil, 0, err
	}
	var all []Record
	prefix, err := readJSONL(p, func(raw []byte) error {
		var r Record
		if err := decodeStrict(raw, &r); err != nil {
			return err
		}
		if err := validRecord(r); err != nil {
			return err
		}
		all = append(all, r)
		return nil
	})
	return all, prefix, err
}

func validRecord(r Record) error {
	if err := validSchema(r.Schema); err != nil {
		return err
	}
	u := r.Usage
	switch {
	case strings.TrimSpace(r.Model) == "":
		return errors.New("model: a request without its model")
	case r.Took < 0 || r.Requests < 0 || u.Input < 0 || u.CacheRead < 0 || u.CacheWrite < 0 || u.Output < 0 || u.Reasoning < 0 || r.Cost < 0:
		return errors.New("counts cannot be negative")
	case r.Cost > 0 && r.Currency == "":
		return errors.New("cost needs its currency")
	}
	switch r.Feature {
	case FeatureTalk, FeatureDraft, FeatureRewrite, FeatureContinue, FeatureCheck:
		if r.At.IsZero() {
			return errors.New("at: a request without its time")
		}
	case FeatureBefore:
	default:
		return fmt.Errorf("feature: unknown %q", r.Feature)
	}
	switch r.Outcome {
	case OutcomeDone, OutcomeCancelled:
	case OutcomeFailed:
		if strings.TrimSpace(r.Error) == "" {
			return errors.New("error: a failure without its reason")
		}
	default:
		return fmt.Errorf("outcome: unknown %q", r.Outcome)
	}
	return nil
}
