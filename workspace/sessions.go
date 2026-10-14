package workspace

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

func (s *Store) sessions() ([]string, error) {
	p, err := s.path("sessions")
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(p)
	if err != nil {
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
		if err := decodeStrict(raw, &e); err != nil {
			return err
		}
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
	events, prefix, err := s.events(session)
	if err != nil {
		return Event{}, err
	}
	if event.Schema == 0 {
		event.Schema = Schema
	}
	if event.At.IsZero() {
		event.At = time.Now().UTC()
	}
	ids := map[string]bool{}
	for _, e := range events {
		ids[e.ID] = true
	}
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
	p, err := s.path("sessions", session+".jsonl")
	if err != nil {
		return Event{}, err
	}
	return event, appendJSONL(p, prefix, event)
}
