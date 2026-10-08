package workspace

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
		if strings.HasPrefix(e.Name(), ".tmp-") {
			continue
		}
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			return nil, fmt.Errorf("%s: unexpected session file", filepath.Join(p, e.Name()))
		}
		id := strings.TrimSuffix(e.Name(), ".jsonl")
		if err = validID(id); err != nil {
			return nil, fmt.Errorf("%s: %w", filepath.Join(p, e.Name()), err)
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
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.check(); err != nil {
		return nil, err
	}
	return s.sessions()
}
func (s *Store) NewSession() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.check(); err != nil {
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
func decodeEvent(b []byte) (Event, error) {
	var e Event
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(&e); err != nil {
		return e, err
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		if err != nil {
			return e, err
		}
		return e, fmt.Errorf("multiple JSON values on one line")
	}
	return e, nil
}

// events returns the length of the recoverable prefix. Only a syntactically
// incomplete, unterminated last line is recoverable. Complete corrupt lines,
// unknown fields and unsupported schemas always fail with filename and line.
func (s *Store) events(session string) ([]Event, int64, error) {
	if err := validID(session); err != nil {
		return nil, 0, err
	}
	p, err := s.path("sessions", session+".jsonl")
	if err != nil {
		return nil, 0, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, 0, err
	}
	events := []Event{}
	seen := map[string]Event{}
	offset := 0
	line := 0
	for offset < len(data) {
		line++
		end := bytes.IndexByte(data[offset:], '\n')
		terminated := end >= 0
		next := len(data)
		if terminated {
			next = offset + end + 1
		}
		raw := bytes.TrimSpace(data[offset:next])
		e, err := decodeEvent(raw)
		if !terminated && (errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF)) {
			return events, int64(offset), nil
		}
		if err == nil {
			err = validateEvent(e)
		}
		if err == nil {
			if _, exists := seen[e.ID]; exists {
				err = fmt.Errorf("id: duplicate %q", e.ID)
			}
		}
		if err == nil && e.ReplyTo != "" {
			if _, exists := seen[e.ReplyTo]; !exists {
				err = fmt.Errorf("reply_to: unknown earlier event %q", e.ReplyTo)
			}
		}
		if err != nil {
			return nil, 0, fmt.Errorf("%s:%d: %w", p, line, err)
		}
		seen[e.ID] = e
		events = append(events, e)
		offset = next
	}
	return events, int64(len(data)), nil
}
func (s *Store) Events(session string) ([]Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.check(); err != nil {
		return nil, err
	}
	events, _, err := s.events(session)
	return events, err
}
func (s *Store) Append(session string, event Event) (Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.check(); err != nil {
		return Event{}, err
	}
	events, prefix, err := s.events(session)
	if err != nil {
		return Event{}, err
	}
	if event.Schema == 0 {
		event.Schema = Schema
	}
	if event.Kind == "" {
		event.Kind = "message"
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
	// Bookkeeping events journal reserved idea IDs before materialization so a
	// crashed bookkeeping operation can replay the same IDs without duplicates.
	if event.Kind != "bookkeeping" {
		if err = s.reloadWork(); err != nil {
			return Event{}, err
		}
		for _, id := range event.IdeaIDs {
			if _, err = s.idea(id); err != nil {
				return Event{}, fmt.Errorf("idea_ids: %w", err)
			}
		}
	}
	data, err := json.Marshal(event)
	if err != nil {
		return Event{}, err
	}
	data = append(data, '\n')
	p, err := s.path("sessions", session+".jsonl")
	if err != nil {
		return Event{}, err
	}
	f, err := os.OpenFile(p, os.O_RDWR, 0600)
	if err != nil {
		return Event{}, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return Event{}, err
	}
	if st.Size() != prefix {
		if err = f.Truncate(prefix); err != nil {
			return Event{}, err
		}
	}
	// A valid final event without a newline is preserved, then delimited.
	if prefix > 0 {
		var last [1]byte
		if _, err = f.ReadAt(last[:], prefix-1); err != nil {
			return Event{}, err
		}
		if last[0] != '\n' {
			data = append([]byte{'\n'}, data...)
		}
	}
	if _, err = f.Seek(prefix, io.SeekStart); err != nil {
		return Event{}, err
	}
	if _, err = f.Write(data); err != nil {
		return Event{}, err
	}
	if err = f.Sync(); err != nil {
		return Event{}, err
	}
	return event, nil
}
