package workspace

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

const summaryExt = ".summary.json"

// Summary is what a chapter tells, in brief, for the AI to keep the work
// consistent with chapters it does not see. It knows the prose it was made
// from, so it can tell when that changed.
type Summary struct {
	Text string `json:"text"`
	// Of is the hash of the prose summarized, and Words its length.
	Of    string `json:"of"`
	Words int    `json:"words"`
	// Read is how many of Words, from the start, the model was given,
	// when the chapter was too long to give whole; 0 when it read all.
	Read int `json:"read,omitempty"`
	// By is the model that wrote it, empty when the author did.
	By   string    `json:"by,omitempty"`
	At   time.Time `json:"at"`
	Sent *Sent     `json:"sent,omitempty"`
}

// ProseHash identifies a chapter's prose as it is stored.
func ProseHash(text string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(CleanProse(text))))
	return hex.EncodeToString(sum[:])
}

// Stale tells whether text is not the prose the summary was made from.
func (s Summary) Stale(text string) bool {
	return s.Text != "" && s.Of != ProseHash(text)
}

// ReadSummary is the summary of document id, or none.
func (s *Store) ReadSummary(id string) (Summary, error) {
	unlock, err := s.lock()
	if err != nil {
		return Summary{}, err
	}
	defer unlock()
	t, err := s.tree()
	if err != nil {
		return Summary{}, err
	}
	n, ok := t.byID[id]
	if !ok || n.Folder {
		return Summary{}, fmt.Errorf("chapters: unknown document %q", id)
	}
	return s.readSummary(n)
}

func (s *Store) readSummary(n *Node) (Summary, error) {
	p, err := s.nodePath(n, summaryExt)
	if err != nil {
		return Summary{}, err
	}
	b, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return Summary{}, nil
	}
	if err != nil {
		return Summary{}, err
	}
	var sum Summary
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err = d.Decode(&sum); err != nil {
		return Summary{}, fmt.Errorf("%s: %w", p, err)
	}
	return sum, nil
}

// Summaries are the summaries of every document that has one, by ID.
func (s *Store) Summaries() (map[string]Summary, error) {
	unlock, err := s.lock()
	if err != nil {
		return nil, err
	}
	defer unlock()
	t, err := s.tree()
	if err != nil {
		return nil, err
	}
	out := map[string]Summary{}
	for id, n := range t.byID {
		if n.Folder {
			continue
		}
		sum, err := s.readSummary(n)
		if err != nil {
			return nil, err
		}
		if sum.Text != "" {
			out[id] = sum
		}
	}
	return out, nil
}

// WriteSummary saves the summary of document id; an empty one removes it.
func (s *Store) WriteSummary(id string, sum Summary) error {
	unlock, err := s.lock()
	if err != nil {
		return err
	}
	defer unlock()
	t, err := s.tree()
	if err != nil {
		return err
	}
	n, ok := t.byID[id]
	if !ok || n.Folder {
		return fmt.Errorf("chapters: unknown document %q", id)
	}
	p, err := s.nodePath(n, summaryExt)
	if err != nil {
		return err
	}
	sum.Text = strings.TrimSpace(strings.ReplaceAll(sum.Text, "\r\n", "\n"))
	if sum.Text == "" {
		if err = os.Remove(p); errors.Is(err, os.ErrNotExist) {
			err = nil
		}
		return err
	}
	if sum.Of == "" {
		return errors.New("summary: the prose it was made from is unknown")
	}
	b, err := json.MarshalIndent(sum, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(p, append(b, '\n'), false)
}
