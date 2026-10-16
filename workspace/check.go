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

const checkExt = ".check.json"

// Kinds of finding.
const (
	FindConflict = "conflict" // the prose is at odds with an entry of the setting
	FindName     = "name"     // a name of the setting, written another way
	FindNew      = "new"      // someone or something the setting does not have
)

// Finding is one place where a chapter and the setting do not agree.
type Finding struct {
	ID    string `json:"id"`
	Kind  string `json:"kind"`
	Title string `json:"title"`
	// Quote is the prose's own words.
	Quote string `json:"quote"`
	// Entry is the entry a conflict is with, and Says its words.
	Entry string `json:"entry,omitempty"`
	Says  string `json:"says,omitempty"`
	// Name is the name meant where one is written another way, or the new
	// name; Category and Text, what the prose says the new one is.
	Name     string `json:"name,omitempty"`
	Category string `json:"category,omitempty"`
	Text     string `json:"text,omitempty"`
	// Meant is set once the author said the prose is as meant: the
	// finding is not brought up again.
	Meant bool `json:"meant,omitempty"`
}

// Check is what reading a chapter against the setting found. It knows the
// prose it read, so it can tell when that changed.
type Check struct {
	Of    string `json:"of"`
	Words int    `json:"words"`
	// Read is how many of Words, from the start, the check was given,
	// when the chapter was too long to give whole; 0 when it read all.
	Read     int       `json:"read,omitempty"`
	By       string    `json:"by"`
	At       time.Time `json:"at"`
	Findings []Finding `json:"findings,omitempty"`
	Sent     *Sent     `json:"sent,omitempty"`
}

// ProseHash identifies a chapter's prose as it is stored.
func ProseHash(text string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(CleanProse(text))))
	return hex.EncodeToString(sum[:])
}

// Stale tells whether text is not the prose the check read.
func (k Check) Stale(text string) bool { return k.Of != ProseHash(text) }

// ReadCheck is the check of document id, or none.
func (s *Store) ReadCheck(id string) (Check, error) {
	unlock, err := s.lock()
	if err != nil {
		return Check{}, err
	}
	defer unlock()
	t, err := s.tree()
	if err != nil {
		return Check{}, err
	}
	n, ok := t.byID[id]
	if !ok || n.Folder {
		return Check{}, fmt.Errorf("chapters: unknown document %q", id)
	}
	return s.readCheck(n)
}

func (s *Store) readCheck(n *Node) (Check, error) {
	p, err := s.nodePath(n, checkExt)
	if err != nil {
		return Check{}, err
	}
	b, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return Check{}, nil
	}
	if err != nil {
		return Check{}, err
	}
	var k Check
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err = d.Decode(&k); err != nil {
		return Check{}, fmt.Errorf("%s: %w", p, err)
	}
	return k, nil
}

// Checks are the checks of every document that has one, by ID.
func (s *Store) Checks() (map[string]Check, error) {
	unlock, err := s.lock()
	if err != nil {
		return nil, err
	}
	defer unlock()
	t, err := s.tree()
	if err != nil {
		return nil, err
	}
	out := map[string]Check{}
	for id, n := range t.byID {
		if n.Folder {
			continue
		}
		k, err := s.readCheck(n)
		if err != nil {
			return nil, err
		}
		if k.Of != "" {
			out[id] = k
		}
	}
	return out, nil
}

// WriteCheck saves the check of document id; one of no prose removes it.
func (s *Store) WriteCheck(id string, k Check) error {
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
	p, err := s.nodePath(n, checkExt)
	if err != nil {
		return err
	}
	if k.Of == "" {
		if err = os.Remove(p); errors.Is(err, os.ErrNotExist) {
			err = nil
		}
		return err
	}
	for i, f := range k.Findings {
		switch {
		case f.ID == "":
			return fmt.Errorf("check: finding %d has no id", i)
		case f.Kind != FindConflict && f.Kind != FindName && f.Kind != FindNew:
			return fmt.Errorf("check: finding %s is of unknown kind %q", f.ID, f.Kind)
		case strings.TrimSpace(f.Quote) == "":
			return fmt.Errorf("check: finding %s quotes nothing", f.ID)
		}
	}
	b, err := json.MarshalIndent(k, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(p, append(b, '\n'), false)
}
