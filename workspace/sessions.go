package workspace

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"writer/internal/strictjson"
)

func (s *Store) sessions() ([]string, error) {
	p, err := s.path("sessions")
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(p)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	ids := []string{}
	for _, e := range entries {
		// What is not a session is not ours, and is left alone: the file
		// a system keeps in every folder, the copy a syncing program made
		// of a session it could not merge.
		id, ok := strings.CutSuffix(e.Name(), ".jsonl")
		if e.IsDir() || !ok || validID(id) != nil {
			continue
		}
		if _, err = s.path("sessions", e.Name()); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids, nil
}
func (s *Store) Sessions() ([]string, error) {
	unlock, err := s.lock()
	if err != nil {
		return nil, err
	}
	defer unlock()
	return s.sessions()
}
func (s *Store) NewSession() (string, error) {
	unlock, err := s.lock()
	if err != nil {
		return "", err
	}
	defer unlock()
	dir, err := s.path("sessions")
	if err != nil {
		return "", err
	}
	if err = os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	for tries := 0; tries < 128; tries++ {
		id := NewID()
		p, err := s.path("sessions", id+".jsonl")
		if err != nil {
			return "", err
		}
		err = atomicWrite(p, nil, true)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		return id, err
	}
	return "", fmt.Errorf("id: could not allocate unique session ID")
}

// events reads a session and returns the length of its recoverable prefix.
func (s *Store) events(session string) ([]Event, int64, error) {
	if err := validID(session); err != nil {
		return nil, 0, err
	}
	p, err := s.path("sessions", session+".jsonl")
	if err != nil {
		return nil, 0, err
	}
	events := []Event{}
	seen := map[string]bool{}
	prefix, err := readJSONL(p, func(raw []byte) error {
		var e Event
		if err := strictjson.Decode(raw, &e); err != nil {
			return err
		}
		if err := checkEvent(e, seen); err != nil {
			return err
		}
		events = append(events, e)
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	return events, prefix, nil
}

func (s *Store) Events(session string) ([]Event, error) {
	unlock, err := s.lock()
	if err != nil {
		return nil, err
	}
	defer unlock()
	events, _, err := s.events(session)
	return events, err
}

func (s *Store) Append(session string, event Event) (Event, error) {
	unlock, err := s.lock()
	if err != nil {
		return Event{}, err
	}
	defer unlock()
	if err := validID(session); err != nil {
		return Event{}, err
	}
	p, err := s.path("sessions", session+".jsonl")
	if err != nil {
		return Event{}, err
	}
	c, err := s.cursor(p, func(c *jsonlCursor, raw []byte) error {
		var e Event
		if err := strictjson.Decode(raw, &e); err != nil {
			return err
		}
		return checkEvent(e, c.ids)
	})
	if err != nil {
		return Event{}, err
	}
	if event.Schema == 0 {
		event.Schema = Schema
	}
	if event.At.IsZero() {
		event.At = time.Now().UTC()
	}
	ids := c.ids
	if event.ID == "" {
		for n := 0; n < 128; n++ {
			event.ID = NewID()
			if !ids[event.ID] {
				break
			}
		}
	}
	if ids[event.ID] {
		return Event{}, fmt.Errorf("id: duplicate event %q", event.ID)
	}
	if err = validateEvent(event); err != nil {
		return Event{}, err
	}
	if event.ReplyTo != "" && !ids[event.ReplyTo] {
		return Event{}, fmt.Errorf("reply_to: unknown earlier event %q", event.ReplyTo)
	}
	if err := s.appendCursor(p, c, event); err != nil {
		return event, err
	}
	ids[event.ID] = true
	return event, nil
}

func checkEvent(e Event, seen map[string]bool) error {
	if err := validateEvent(e); err != nil {
		return err
	}
	if seen[e.ID] {
		return fmt.Errorf("id: duplicate %q", e.ID)
	}
	if e.ReplyTo != "" && !seen[e.ReplyTo] {
		return fmt.Errorf("reply_to: unknown earlier event %q", e.ReplyTo)
	}
	seen[e.ID] = true
	return nil
}
