package workspace

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
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
	for _, d := range []string{"ideas", "settings", "topics", "sessions", "chapters", ".writer", ".writer/embeddings"} {
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
	for _, d := range []string{"ideas", "sessions"} {
		p, err := s.path(d)
		if err != nil {
			return nil, err
		}
		st, err := os.Stat(p)
		if err != nil {
			return nil, err
		}
		if !st.IsDir() {
			return nil, fmt.Errorf("%s: expected directory", p)
		}
	}
	if _, err = s.ideas(); err != nil {
		return nil, err
	}
	sessions, err := s.sessions()
	if err != nil {
		return nil, err
	}
	for _, id := range sessions {
		if _, _, err = s.events(id); err != nil {
			return nil, err
		}
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
	ideas, err := s.ideas()
	if err != nil {
		return err
	}
	for _, i := range ideas {
		if err = validateIdea(i, w); err != nil {
			return fmt.Errorf("ideas/%s.md: %w", i.ID, err)
		}
	}
	return s.writeWork(w)
}

// scan memoizes what checking many ideas rereads: the sessions their
// origins point into, and which idea files exist.
type scan struct {
	sessions map[string][]Event
	ids      map[string]bool // nil: look each reference up on disk
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

func readIdea(p string) (Idea, error) {
	b, err := os.ReadFile(p)
	if err != nil {
		return Idea{}, err
	}
	// Normalize delimiters only; body bytes, including CRLF, remain intact.
	start := 0
	if bytes.HasPrefix(b, []byte("---\r\n")) {
		start = 5
	} else if bytes.HasPrefix(b, []byte("---\n")) {
		start = 4
	} else {
		return Idea{}, fmt.Errorf("%s: missing YAML front matter", p)
	}
	pos := start
	for pos < len(b) {
		end := bytes.IndexByte(b[pos:], '\n')
		next := len(b)
		if end >= 0 {
			next = pos + end + 1
		}
		line := strings.TrimSuffix(strings.TrimSuffix(string(b[pos:next]), "\n"), "\r")
		if line == "---" {
			var i Idea
			if err = decodeYAML(b[start:pos], &i); err != nil {
				return i, fmt.Errorf("%s: %w", p, err)
			}
			i.Text = string(b[next:])
			return i, nil
		}
		pos = next
	}
	return Idea{}, fmt.Errorf("%s: unterminated YAML front matter", p)
}

func (s *Store) idea(id string, sc *scan) (Idea, error) {
	if err := validID(id); err != nil {
		return Idea{}, err
	}
	p, err := s.path("ideas", id+".md")
	if err != nil {
		return Idea{}, err
	}
	i, err := readIdea(p)
	if err == nil && i.ID != id {
		err = fmt.Errorf("id: does not match filename")
	}
	if err == nil {
		err = validateIdea(i, s.work)
	}
	if err == nil {
		err = s.validateOrigin(i, sc)
	}
	if err == nil {
		err = s.validateReferences(i, sc)
	}
	if err != nil {
		return Idea{}, fmt.Errorf("%s: %w", p, err)
	}
	return i, nil
}

func (s *Store) validateReferences(i Idea, sc *scan) error {
	refs := append(append([]string{}, i.Conflicts...), i.Supersedes...)
	for _, similar := range i.Similar {
		refs = append(refs, similar.ID)
	}
	for _, id := range refs {
		if sc.ids != nil {
			if !sc.ids[id] {
				return fmt.Errorf("reference %s: %w", id, os.ErrNotExist)
			}
			continue
		}
		p, err := s.path("ideas", id+".md")
		if err != nil {
			return err
		}
		st, err := os.Stat(p)
		if err != nil {
			return fmt.Errorf("reference %s: %w", id, err)
		}
		if !st.Mode().IsRegular() {
			return fmt.Errorf("reference %s: not a regular idea file", id)
		}
	}
	return nil
}

func (s *Store) Idea(id string) (Idea, error) {
	unlock, err := s.lock()
	if err != nil {
		return Idea{}, err
	}
	defer unlock()
	return s.idea(id, newScan())
}

// ideas reads every idea, oldest first; a turn's ideas keep the order its
// bookkeeping gave them, the author's before the AI's.
func (s *Store) ideas() ([]Idea, error) {
	p, err := s.path("ideas")
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(p)
	if err != nil {
		return nil, err
	}
	sc := newScan()
	sc.ids = map[string]bool{}
	var names []string
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".tmp-") {
			continue
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			return nil, fmt.Errorf("%s: unexpected canonical idea file", filepath.Join(p, entry.Name()))
		}
		id := strings.TrimSuffix(entry.Name(), ".md")
		sc.ids[id] = true
		names = append(names, id)
	}
	result := []Idea{}
	for _, id := range names {
		i, err := s.idea(id, sc)
		if err != nil {
			return nil, err
		}
		result = append(result, i)
	}
	sort.SliceStable(result, func(a, b int) bool {
		x, y := result[a], result[b]
		if !x.Created.Equal(y.Created) {
			return x.Created.Before(y.Created)
		}
		if x.Source != y.Source {
			return x.Source == SourceHuman
		}
		return x.ID < y.ID
	})
	return result, nil
}

func (s *Store) Ideas() ([]Idea, error) {
	unlock, err := s.lock()
	if err != nil {
		return nil, err
	}
	defer unlock()
	return s.ideas()
}

func (s *Store) saveIdea(i Idea, exclusive bool) error {
	if err := validateIdea(i, s.work); err != nil {
		return fmt.Errorf("ideas/%s.md: %w", i.ID, err)
	}
	sc := newScan()
	if err := s.validateOrigin(i, sc); err != nil {
		return fmt.Errorf("ideas/%s.md: %w", i.ID, err)
	}
	refs := append(append([]string{}, i.Conflicts...), i.Supersedes...)
	for _, similar := range i.Similar {
		refs = append(refs, similar.ID)
	}
	for _, id := range refs {
		if _, err := s.idea(id, sc); err != nil {
			return fmt.Errorf("ideas/%s.md: reference %s: %w", i.ID, id, err)
		}
	}
	p, err := s.path("ideas", i.ID+".md")
	if err != nil {
		return err
	}
	if !exclusive {
		old, readErr := s.idea(i.ID, sc)
		switch {
		case readErr == nil:
			if err = checkUpdate(old, i); err != nil {
				return err
			}
		case errors.Is(readErr, os.ErrNotExist):
			exclusive = true
		default:
			return readErr
		}
	}
	b, err := yaml.Marshal(i)
	if err != nil {
		return err
	}
	data := append([]byte("---\n"), b...)
	data = append(data, []byte("---\n"+i.Text)...)
	return atomicWrite(p, data, exclusive)
}

// checkUpdate keeps an idea's past intact: its history only grows, and its
// status only moves along that history.
func checkUpdate(old, i Idea) error {
	if err := transition(old.Status, i.Status); err != nil {
		return err
	}
	if old.Status != i.Status && (len(i.History) != len(old.History)+1 || i.History[len(i.History)-1].From != old.Status) {
		return fmt.Errorf("history: status changes require an appended transition")
	}
	if !i.Created.Equal(old.Created) {
		return fmt.Errorf("created: cannot change")
	}
	if i.Updated.Before(old.Updated) {
		return fmt.Errorf("updated: cannot move backwards")
	}
	if len(i.History) < len(old.History) {
		return fmt.Errorf("history: cannot remove records")
	}
	for n, h := range old.History {
		next := i.History[n]
		if !h.At.Equal(next.At) || h.From != next.From || h.To != next.To || h.Reason != next.Reason {
			return fmt.Errorf("history: cannot rewrite records")
		}
	}
	return nil
}

func (s *Store) SaveIdea(i Idea) error {
	unlock, err := s.lock()
	if err != nil {
		return err
	}
	defer unlock()
	return s.saveIdea(i, false)
}

// NewIdea fills in defaults and creates the idea; it never overwrites an
// existing file, and retries a generated ID that collides.
func (s *Store) NewIdea(i Idea) (Idea, error) {
	unlock, err := s.lock()
	if err != nil {
		return Idea{}, err
	}
	defer unlock()
	if i.Schema == 0 {
		i.Schema = Schema
	}
	if i.Status == "" {
		i.Status = Unprocessed
	}
	if i.Category == "" {
		i.Category = Other
	}
	if i.Source == "" {
		i.Source = SourceHuman
	}
	if i.Created.IsZero() {
		i.Created = time.Now().UTC()
	}
	if i.Updated.IsZero() {
		i.Updated = i.Created
	}
	generated := i.ID == ""
	for tries := 0; tries < 128; tries++ {
		if generated {
			i.ID = NewID()
		}
		err := s.saveIdea(i, true)
		if generated && errors.Is(err, os.ErrExist) {
			continue
		}
		return i, err
	}
	return Idea{}, fmt.Errorf("id: could not allocate unique idea ID")
}

func (s *Store) ChangeStatus(id string, status Status, reason string) (Idea, error) {
	unlock, err := s.lock()
	if err != nil {
		return Idea{}, err
	}
	defer unlock()
	i, err := s.idea(id, newScan())
	if err != nil {
		return Idea{}, err
	}
	if err = transition(i.Status, status); err != nil {
		return Idea{}, err
	}
	if i.Status == status {
		return i, nil
	}
	now := time.Now().UTC()
	i.History = append(i.History, History{At: now, From: i.Status, To: status, Reason: reason})
	i.Status = status
	i.Updated = now
	return i, s.saveIdea(i, false)
}

func (s *Store) SetCategory(id, category string) error {
	unlock, err := s.lock()
	if err != nil {
		return err
	}
	defer unlock()
	i, err := s.idea(id, newScan())
	if err != nil {
		return err
	}
	i.Category = category
	i.Updated = time.Now().UTC()
	return s.saveIdea(i, false)
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
	ideas, err := s.ideas()
	if err != nil {
		return err
	}
	// Reclassify first: even if interrupted, every intermediate file remains valid
	// under the old category list. A retry completes the operation safely.
	for _, i := range ideas {
		if i.Category == name {
			i.Category = Other
			i.Updated = time.Now().UTC()
			if err = s.saveIdea(i, false); err != nil {
				return err
			}
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
