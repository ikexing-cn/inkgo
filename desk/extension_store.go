package desk

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"writer/ext"
	"writer/internal/atomicfile"
	"writer/locales"
)

// extensionFile is ext.json: off lists disabled local IDs, switches contains
// book overrides, settings maps qualified IDs to raw JSON, and injections maps
// hook IDs to order/choice (local only). Reads never write; only a Save replaces
// a damaged file. Unreadable files are reported and treated as empty.
type extensionFile struct {
	Off        []string                           `json:"off,omitempty"`
	Switches   map[string]bool                    `json:"switches,omitempty"`
	Settings   map[string]json.RawMessage         `json:"settings,omitempty"`
	Injections map[string]ext.InjectionPreference `json:"injections,omitempty"`
}
type extensionStore struct {
	mu     sync.Mutex
	root   string
	files  map[string]*extensionFile
	report func(error)
}

func newExtensionStore(root string, report func(error)) *extensionStore {
	return &extensionStore{root: root, files: map[string]*extensionFile{}, report: report}
}

func (s *extensionStore) path(b ext.Book) string {
	if b.Dir != "" {
		return filepath.Join(b.Dir, ".writer", "ext.json")
	}
	if s.root == "" {
		return ""
	}
	return filepath.Join(s.root, "ext.json")
}

func (s *extensionStore) file(b ext.Book) *extensionFile {
	path := s.path(b)
	if f := s.files[path]; f != nil {
		return f
	}
	f := &extensionFile{}
	if path != "" {
		data, err := os.ReadFile(path)
		if err == nil {
			err = json.Unmarshal(data, f)
		}
		if err != nil && !os.IsNotExist(err) {
			s.report(fmt.Errorf("%s", locales.ExtFileUnreadable(fmt.Sprintf("%s: %v", path, err))))
			f = &extensionFile{}
		}
	}
	if f.Settings == nil {
		f.Settings = map[string]json.RawMessage{}
	}
	s.files[path] = f
	return f
}

func (s *extensionStore) save(b ext.Book) error {
	path := s.path(b)
	if path == "" {
		return nil
	}
	data, err := json.MarshalIndent(s.file(b), "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return atomicfile.Write(path, append(data, '\n'), 0644)
}

func (s *extensionStore) Load(b ext.Book, id string) ([]byte, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, ok := s.file(b).Settings[id]
	return append([]byte(nil), data...), ok
}

func (s *extensionStore) Save(b ext.Book, id string, data []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.file(b).Settings[id] = append([]byte(nil), data...)
	if err := s.save(b); err != nil {
		s.report(err)
	}
}

type localExtensions struct{ s *extensionStore }

func (l localExtensions) Load() ([]string, error) {
	l.s.mu.Lock()
	defer l.s.mu.Unlock()
	return append([]string(nil), l.s.file(ext.Book{}).Off...), nil
}

func (l localExtensions) Save(off []string) error {
	l.s.mu.Lock()
	defer l.s.mu.Unlock()
	f := l.s.file(ext.Book{})
	old := f.Off
	f.Off = append([]string(nil), off...)
	if err := l.s.save(ext.Book{}); err != nil {
		f.Off = old
		return err
	}
	return nil
}

type bookExtensions struct{ s *extensionStore }

func (l bookExtensions) Load(b ext.Book) (map[string]bool, error) {
	l.s.mu.Lock()
	defer l.s.mu.Unlock()
	return copyBookSwitches(l.s.file(b).Switches), nil
}

func (l bookExtensions) Save(b ext.Book, v map[string]bool) error {
	l.s.mu.Lock()
	defer l.s.mu.Unlock()
	f := l.s.file(b)
	old := f.Switches
	f.Switches = copyBookSwitches(v)
	if err := l.s.save(b); err != nil {
		f.Switches = old
		return err
	}
	return nil
}

func copyBookSwitches(v map[string]bool) map[string]bool {
	out := map[string]bool{}
	for k, b := range v {
		out[k] = b
	}
	return out
}

type injectionExtensions struct{ s *extensionStore }

func (l injectionExtensions) Load() (map[string]ext.InjectionPreference, error) {
	l.s.mu.Lock()
	defer l.s.mu.Unlock()
	return copyInjections(l.s.file(ext.Book{}).Injections), nil
}

func (l injectionExtensions) Save(v map[string]ext.InjectionPreference) error {
	l.s.mu.Lock()
	defer l.s.mu.Unlock()
	f := l.s.file(ext.Book{})
	old := f.Injections
	f.Injections = copyInjections(v)
	if err := l.s.save(ext.Book{}); err != nil {
		f.Injections = old
		return err
	}
	return nil
}

func copyInjections(v map[string]ext.InjectionPreference) map[string]ext.InjectionPreference {
	out := map[string]ext.InjectionPreference{}
	for k, p := range v {
		p.Order = append([]string(nil), p.Order...)
		out[k] = p
	}
	return out
}

// migrate copies one-time shelf data; remove after 2026-12. It never replaces
// existing settings and leaves the shelf data intact if persistence fails.
func (s *extensionStore) migrate(data json.RawMessage) error {
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	f := s.file(ext.Book{})
	if _, ok := f.Settings["phone.typesetting"]; ok {
		return nil
	}
	f.Settings["phone.typesetting"] = append([]byte(nil), data...)
	if err := s.save(ext.Book{}); err != nil {
		delete(f.Settings, "phone.typesetting")
		return err
	}
	return nil
}
