// Package shelf keeps the list of the author's works: where each lives,
// when it was last opened, and what it was called then, so the list shows
// without opening every work.
package shelf

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"writer/backup"
	"writer/export"
	"writer/internal/atomicfile"
	"writer/internal/strictjson"
	"writer/workspace"
)

const Schema = 1

// Book is a work on the shelf, as it was when last opened or closed.
type Book struct {
	Path   string    `json:"path"`
	Name   string    `json:"name"`
	Genre  string    `json:"genre"`
	Words  int       `json:"words"`
	Opened time.Time `json:"opened"`
	// Backup is how the work is backed up from this machine; nil until
	// the author is first asked.
	Backup *backup.Settings `json:"backup,omitempty"`
}

// Shelf is works.json.
type Shelf struct {
	Schema int `json:"schema"`
	// Library is where new works go unless the author picks elsewhere.
	Library string `json:"library"`
	// Cards shows the works as cards rather than a list.
	Cards bool `json:"cards,omitempty"`
	// Theme is the appearance chosen: "light", "dark", or empty to follow
	// the system.
	Theme string `json:"theme,omitempty"`
	// Zoom is the window's content scale on this machine, for every work.
	// Absent or zero, it is actual size.
	Zoom float32 `json:"zoom,omitempty"`
	// NoAI turns the AI off on this machine, for every work: nothing of it
	// shows and nothing is sent. Absent, the AI is on.
	NoAI bool `json:"noAI,omitempty"`
	// NoNames hides setting name links in prose on this machine. Absent,
	// names are marked, including shelves saved before this preference.
	NoNames bool `json:"noNames,omitempty"`
	// NoGPU draws the window without the GPU on this machine, from the next
	// time the software opens. Absent, the GPU draws where there is one.
	NoGPU bool `json:"noGPU,omitempty"`
	// Export is how the export dialog was last left, for every work; nil,
	// it was never used.
	Export *Export `json:"export,omitempty"`
	// MovedPhone carries one-time migration data; remove after 2026-12.
	MovedPhone json.RawMessage `json:"phone,omitempty"`
	Books      []Book          `json:"books"`
}

// Export is what the export dialog exports, of the prose, the outline and
// the setting; Text sends plain text rather than Word documents. Pen is
// the pen name last put on a manuscript, for a work without one.
type Export struct {
	export.Options
	Pen string `json:"pen,omitempty"`
}

// File is the shelf beside the model configuration at config.
func File(config string) string {
	return filepath.Join(filepath.Dir(config), "works.json")
}

// DefaultLibrary is a 写作 folder in the author's documents.
func DefaultLibrary() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "写作"
	}
	return filepath.Join(home, "Documents", "写作")
}

// Start is a first shelf: the works already in library.
func Start(library string) Shelf {
	s := Shelf{Schema: Schema, Library: library}
	for _, p := range Scan(library) {
		s.Books = append(s.Books, Book{Path: p, Name: filepath.Base(p)})
	}
	return s
}

// Load reads the shelf at path; fresh says there is none yet.
func Load(path string) (s Shelf, fresh bool, err error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Shelf{}, true, nil
	}
	if err != nil {
		return Shelf{}, false, err
	}
	if err = strictjson.Decode(b, &s); err != nil {
		return Shelf{}, false, fmt.Errorf("%s: %w", path, err)
	}
	if s.Schema != Schema {
		return Shelf{}, false, fmt.Errorf("%s: unsupported schema %d", path, s.Schema)
	}
	if s.Theme != "" && s.Theme != "light" && s.Theme != "dark" {
		return Shelf{}, false, fmt.Errorf("%s: unknown theme %q", path, s.Theme)
	}
	if s.Zoom != 0 {
		s.Zoom = NearestZoom(s.Zoom)
	}
	for _, bk := range s.Books {
		if !filepath.IsAbs(bk.Path) {
			return Shelf{}, false, fmt.Errorf("%s: work path %q is not absolute", path, bk.Path)
		}
	}
	return s, false, nil
}

// Save writes the shelf whole, through a temporary file.
func (s Shelf) Save(path string) error {
	s.Schema = Schema
	if s.Books == nil {
		s.Books = []Book{}
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return atomicfile.Write(path, append(b, '\n'), 0644)
}

// SameFolder tells whether two work paths are the same folder.
func SameFolder(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// Find is the index of the work at path, or -1.
func (s *Shelf) Find(path string) int {
	return slices.IndexFunc(s.Books, func(b Book) bool { return SameFolder(b.Path, path) })
}

// Put adds the work or updates what the shelf remembers of it.
func (s *Shelf) Put(b Book) {
	if i := s.Find(b.Path); i >= 0 {
		if b.Opened.IsZero() {
			b.Opened = s.Books[i].Opened
		}
		if b.Backup == nil {
			b.Backup = s.Books[i].Backup
		}
		s.Books[i] = b
		return
	}
	s.Books = append(s.Books, b)
}

// Backup is how the work at path is backed up, and whether it is on the
// shelf with settings of its own.
func (s *Shelf) Backup(path string) (backup.Settings, bool) {
	if i := s.Find(path); i >= 0 && s.Books[i].Backup != nil {
		return *s.Books[i].Backup, true
	}
	return backup.Settings{}, false
}

// SetBackup sets how the work at path is backed up; a work not on the
// shelf is left alone.
func (s *Shelf) SetBackup(path string, b backup.Settings) {
	if i := s.Find(path); i >= 0 {
		s.Books[i].Backup = &b
	}
}

// Remove takes the work off the shelf; its folder is left alone.
func (s *Shelf) Remove(path string) {
	if i := s.Find(path); i >= 0 {
		s.Books = slices.Delete(s.Books, i, i+1)
	}
}

// Relocate points the work at from to its new folder, keeping when it was
// last opened.
func (s *Shelf) Relocate(from, to string) {
	i := s.Find(from)
	if i < 0 {
		return
	}
	if j := s.Find(to); j >= 0 && j != i {
		s.Books = slices.Delete(s.Books, i, i+1) // the new folder is already on the shelf
		return
	}
	s.Books[i].Path = to
}

// Recent is the works by when they were last opened, newest first; works
// never opened come last, by name.
func (s *Shelf) Recent() []Book {
	out := slices.Clone(s.Books)
	slices.SortStableFunc(out, func(a, b Book) int {
		if c := b.Opened.Compare(a.Opened); c != 0 {
			return c
		}
		return strings.Compare(a.Name, b.Name)
	})
	return out
}

// Scan is the works directly inside dir.
func Scan(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if p := filepath.Join(dir, e.Name()); e.IsDir() && workspace.IsWork(p) {
			out = append(out, p)
		}
	}
	return out
}
