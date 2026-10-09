package workspace

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"go.yaml.in/yaml/v3"
)

// Store is one open work. Only one application instance may write a work at
// a time; within it, the mutex serializes every operation.
type Store struct {
	mu     sync.Mutex
	root   string
	work   Work
	closed bool
}

func (s *Store) Root() string { return s.root }

func cloneWork(w Work) Work {
	w.Categories = append([]Category(nil), w.Categories...)
	return w
}

func (s *Store) Work() Work {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneWork(s.work)
}

func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}

// lock serializes an operation and rereads work.yaml first, so a category
// list edited by hand applies to it.
func (s *Store) lock() (unlock func(), err error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil, errors.New("workspace is closed")
	}
	if err := s.reloadWork(); err != nil {
		s.mu.Unlock()
		return nil, err
	}
	return s.mu.Unlock, nil
}

// Hold runs fn with the work held still: nothing of it is written until
// fn returns.
func (s *Store) Hold(fn func(root string) error) error {
	unlock, err := s.lock()
	if err != nil {
		return err
	}
	defer unlock()
	return fn(s.root)
}

// Create accepts a missing or empty directory. It never replaces an existing
// workspace or adopts unrelated files in a nonempty directory.
func Create(dir, name, genre string) (*Store, error) {
	root, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(root)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if len(entries) > 0 {
		return nil, fmt.Errorf("%s: directory is not empty", root)
	}
	w := Work{Schema: Schema, ID: NewID(), Name: name, Genre: genre, Categories: Categories(genre), TokenBudget: 16000}
	if err = validateWork(w); err != nil {
		return nil, err
	}
	if err = os.MkdirAll(root, 0755); err != nil {
		return nil, err
	}
	for _, d := range []string{"sessions", "chapters", ".writer"} {
		if err = os.Mkdir(filepath.Join(root, d), 0755); err != nil {
			return nil, err
		}
	}
	if err = atomicWrite(filepath.Join(root, ".gitignore"), []byte(".writer/\n"), true); err != nil {
		return nil, err
	}
	data, err := yaml.Marshal(w)
	if err != nil {
		return nil, err
	}
	if err = atomicWrite(filepath.Join(root, "work.yaml"), data, true); err != nil {
		return nil, err
	}
	if err = atomicWrite(filepath.Join(root, "setting.jsonl"), nil, true); err != nil {
		return nil, err
	}
	return Open(root)
}

// Open validates every canonical file, so a hand edit gone wrong is reported
// with its file and field before anything is written.
func Open(dir string) (*Store, error) {
	root, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	s := &Store{root: root}
	if err = s.reloadWork(); err != nil {
		return nil, err
	}
	p, err := s.path("sessions")
	if err != nil {
		return nil, err
	}
	if st, err := os.Stat(p); err != nil {
		return nil, err
	} else if !st.IsDir() {
		return nil, fmt.Errorf("%s: expected directory", p)
	}
	sessions, err := s.sessions()
	if err != nil {
		return nil, err
	}
	sc := newScan()
	for _, id := range sessions {
		if _, err = sc.events(s, id); err != nil {
			return nil, err
		}
	}
	if _, _, err = s.setting(sc); err != nil {
		return nil, err
	}
	return s, nil
}

// path refuses symlink components so a hand-edited workspace cannot redirect
// reads or writes outside its root. All IDs are validated before reaching it.
func (s *Store) path(parts ...string) (string, error) {
	p := s.root
	for _, part := range parts {
		p = filepath.Join(p, part)
		st, err := os.Lstat(p)
		if err != nil && !os.IsNotExist(err) {
			return "", err
		}
		if err == nil && st.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("%s: symlinks are not allowed", p)
		}
	}
	return p, nil
}

func decodeYAML(data []byte, out any) error {
	d := yaml.NewDecoder(bytes.NewReader(data))
	d.KnownFields(true)
	if err := d.Decode(out); err != nil {
		return err
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		if err != nil {
			return err
		}
		return fmt.Errorf("unexpected extra YAML document")
	}
	return nil
}

func (s *Store) reloadWork() error {
	p, err := s.path("work.yaml")
	if err != nil {
		return err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return err
	}
	var w Work
	if err = decodeYAML(b, &w); err == nil {
		err = validateWork(w)
	}
	if err != nil {
		return fmt.Errorf("%s: %w", p, err)
	}
	s.work = w
	return nil
}

func (s *Store) writeWork(w Work) error {
	data, err := yaml.Marshal(w)
	if err != nil {
		return err
	}
	p, err := s.path("work.yaml")
	if err != nil {
		return err
	}
	if err = atomicWrite(p, data, false); err != nil {
		return err
	}
	s.work = cloneWork(w)
	return nil
}

func (s *Store) SaveWork(w Work) error {
	unlock, err := s.lock()
	if err != nil {
		return err
	}
	defer unlock()
	if err := validateWork(w); err != nil {
		return fmt.Errorf("work.yaml: %w", err)
	}
	if w.ID != s.work.ID {
		return fmt.Errorf("work.yaml: id cannot change")
	}
	set, _, err := s.setting(nil)
	if err != nil {
		return err
	}
	for _, e := range set.Entries {
		if !slices.ContainsFunc(w.Categories, func(c Category) bool { return c.Name == e.Category }) {
			return fmt.Errorf("categories: %q is still used by the setting", e.Category)
		}
	}
	return s.writeWork(w)
}

// scan memoizes the sessions that checking many sources rereads.
type scan struct {
	sessions map[string][]Event
}

func newScan() *scan { return &scan{sessions: map[string][]Event{}} }

func (sc *scan) events(s *Store, session string) ([]Event, error) {
	if events, ok := sc.sessions[session]; ok {
		return events, nil
	}
	events, _, err := s.events(session)
	if err != nil {
		return nil, err
	}
	sc.sessions[session] = events
	return events, nil
}

func (s *Store) RemoveCategory(name string) error {
	unlock, err := s.lock()
	if err != nil {
		return err
	}
	defer unlock()
	if name == Other {
		return fmt.Errorf("categories: cannot remove %q", Other)
	}
	w := cloneWork(s.work)
	found := -1
	for n, c := range w.Categories {
		if c.Name == name {
			found = n
		}
	}
	if found < 0 {
		return fmt.Errorf("categories: unknown %q", name)
	}
	set, prefix, err := s.setting(nil)
	if err != nil {
		return err
	}
	// Reclassify first: if interrupted, the setting still holds under the
	// old category list, and removing again completes the operation.
	var ops []Op
	for _, e := range set.Entries {
		if e.Category == name {
			ops = append(ops, Op{Op: OpMove, Entry: e.ID, Category: Other})
		}
	}
	if len(ops) > 0 {
		r := Revision{Schema: Schema, ID: NewID(), At: time.Now().UTC(), By: ByAuthor, Ops: ops}
		p, err := s.path("setting.jsonl")
		if err != nil {
			return err
		}
		if err = appendJSONL(p, prefix, r); err != nil {
			return err
		}
	}
	w.Categories = append(w.Categories[:found], w.Categories[found+1:]...)
	return s.writeWork(w)
}

// atomicWrite writes a temporary file and renames it over path, so a crash
// never leaves half a file. exclusive refuses an existing path; checking
// first suffices because one application instance writes a work, and works
// on file systems without hard links, such as exFAT drives.
func atomicWrite(path string, data []byte, exclusive bool) error {
	if exclusive {
		if _, err := os.Lstat(path); err == nil {
			return &os.PathError{Op: "create", Path: path, Err: os.ErrExist}
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".tmp-")
	if err != nil {
		return err
	}
	temp := f.Name()
	defer os.Remove(temp)
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(temp, path)
}
