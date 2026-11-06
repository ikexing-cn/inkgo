package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"writer/internal/strictjson"
)

// logFile is the AI record of the work, beside work.yaml, so that a backup
// takes it along: a line for each request made of a model.
const logFile = "ext/usage/ai-log.jsonl"

// The features a request is made for, as the AI record counts them.
const (
	FeatureTalk     = "talk"
	FeatureDraft    = "draft"
	FeatureRewrite  = "rewrite"
	FeatureContinue = "continue"
	FeatureCheck    = "check"
)

// How a request ended.
const (
	OutcomeDone      = "done"
	OutcomeCancelled = "cancelled"
	OutcomeFailed    = "failed"
)

// Record is a request the work made of a model: what it was for, the
// parts it sent and their tokens as estimated, what the provider counted
// and how it ended. The text
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
	Outcome string    `json:"outcome"`
	Error   string    `json:"error,omitempty"`
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
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}
	c, err := s.cursor(p, func(_ *jsonlCursor, raw []byte) error {
		var record Record
		if err := strictjson.Decode(raw, &record); err != nil {
			return err
		}
		return validRecord(record)
	})
	if errors.Is(err, os.ErrNotExist) {
		line, err := json.Marshal(r)
		if err != nil {
			return err
		}
		return atomicWrite(p, append(line, '\n'), true)
	}
	if err != nil {
		return err
	}
	return s.appendCursor(p, c, r)
}

// Records is the AI record, the oldest first.
func (s *Store) Records() ([]Record, error) {
	unlock, err := s.lock()
	if err != nil {
		return nil, err
	}
	defer unlock()
	all, _, err := s.records()
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
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
		if err := strictjson.Decode(raw, &r); err != nil {
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
	case r.Took < 0 || u.Input < 0 || u.CacheRead < 0 || u.CacheWrite < 0 || u.Output < 0 || u.Reasoning < 0:
		return errors.New("counts cannot be negative")
	}
	switch r.Feature {
	case FeatureTalk, FeatureDraft, FeatureRewrite, FeatureContinue, FeatureCheck:
		if r.At.IsZero() {
			return errors.New("at: a request without its time")
		}
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
