package workspace

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"time"
)

// Trashed is a node in the trash with what it held.
type Trashed struct {
	ID     string    `json:"id"`
	Title  string    `json:"title"`
	Folder bool      `json:"folder,omitempty"`
	File   string    `json:"file"`
	Parent string    `json:"parent,omitempty"`
	At     time.Time `json:"at"`
}

// TrashChapter moves node id, with what it holds, to the trash.
func (s *Store) TrashChapter(id string) error {
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
	if !ok {
		return fmt.Errorf("chapters: unknown node %q", id)
	}
	dir, err := s.path(trashDir, n.ID)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	info := Trashed{ID: n.ID, Title: n.Title, Folder: n.Folder, File: n.file, Parent: n.Parent.ID, At: time.Now().UTC()}
	b, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return err
	}
	if err = atomicWrite(filepath.Join(dir, trashedFile), append(b, '\n'), false); err != nil {
		return err
	}
	var from, to []string
	for _, ext := range companions(n) {
		p, err := s.nodePath(n, ext)
		if err != nil {
			return err
		}
		from, to = append(from, p), append(to, filepath.Join(dir, n.file+ext))
	}
	if err = moveAll(from, to); err != nil {
		// Still where it was: the trash has nothing of it.
		return errors.Join(err, os.RemoveAll(dir))
	}
	n.Parent.Children = slices.DeleteFunc(n.Parent.Children, func(x *Node) bool { return x == n })
	return s.writeOrder(n.Parent)
}

// trashedIDs returns the IDs in the trash, of the nodes trashed and what
// trashed folders hold.
func (s *Store) trashedIDs() (map[string]bool, error) {
	used := map[string]bool{}
	items, err := s.trash()
	if err != nil {
		return nil, err
	}
	for _, it := range items {
		used[it.ID] = true
		ids, err := s.heldIDs(it)
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			used[id] = true
		}
	}
	return used, nil
}

// heldIDs returns the IDs the .order files of a trashed folder list.
func (s *Store) heldIDs(it Trashed) ([]string, error) {
	if !it.Folder {
		return nil, nil
	}
	root, err := s.path(trashDir, it.ID, it.File)
	if err != nil {
		return nil, err
	}
	var ids []string
	err = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() != orderFile {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		var order []orderEntry
		if json.Unmarshal(b, &order) == nil {
			for _, e := range order {
				ids = append(ids, e.ID)
			}
		}
		return nil
	})
	if errors.Is(err, os.ErrNotExist) {
		err = nil
	}
	return ids, err
}

func (s *Store) trash() ([]Trashed, error) {
	p, err := s.path(trashDir)
	if err != nil {
		return nil, err
	}
	dirs, err := os.ReadDir(p)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var items []Trashed
	for _, d := range dirs {
		if !d.IsDir() || validID(d.Name()) != nil {
			continue
		}
		b, err := os.ReadFile(filepath.Join(p, d.Name(), trashedFile))
		if err != nil {
			continue // half trashed by a crash: its files are still in there
		}
		var it Trashed
		if json.Unmarshal(b, &it) != nil || it.ID != d.Name() {
			continue
		}
		// Noted as trashed and never moved here: the node is still in the
		// tree, and its ID is its own.
		ext := textExt
		if it.Folder {
			ext = ""
		}
		if _, err = os.Stat(filepath.Join(p, d.Name(), it.File+ext)); err != nil {
			continue
		}
		items = append(items, it)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].At.After(items[j].At) })
	return items, nil
}

// Trash returns what is in the trash, the latest first.
func (s *Store) Trash() ([]Trashed, error) {
	unlock, err := s.lock()
	if err != nil {
		return nil, err
	}
	defer unlock()
	return s.trash()
}

// RestoreChapter puts node id back at the end of the folder it was in, or
// at the top when that folder is gone.
func (s *Store) RestoreChapter(id string) error {
	unlock, err := s.lock()
	if err != nil {
		return err
	}
	defer unlock()
	items, err := s.trash()
	if err != nil {
		return err
	}
	i := slices.IndexFunc(items, func(it Trashed) bool { return it.ID == id })
	if i < 0 {
		return &TrashMissingError{ID: id}
	}
	it := items[i]
	t, err := s.tree()
	if err != nil {
		return err
	}
	to, err := t.folderOf(it.Parent)
	if err != nil {
		to = t.Root
	}
	dir, err := s.path(trashDir, id)
	if err != nil {
		return err
	}
	n := &Node{ID: it.ID, Title: it.Title, Folder: it.Folder, file: it.File}
	file := fileName(it.Title, takenIn(to, nil))
	moved := &Node{ID: it.ID, Title: it.Title, Folder: it.Folder, Parent: to, file: file, dir: childDir(to)}
	to.Children = append(to.Children, moved)
	if err = s.writeOrder(to); err != nil {
		return err
	}
	var from, into []string
	for _, ext := range companions(n) {
		dst, err := s.nodePath(moved, ext)
		if err != nil {
			return err
		}
		from, into = append(from, filepath.Join(dir, it.File+ext)), append(into, dst)
	}
	if err = moveAll(from, into); err != nil {
		// Still in the trash: the folder does not list it.
		to.Children = to.Children[:len(to.Children)-1]
		return errors.Join(err, s.writeOrder(to))
	}
	return os.RemoveAll(dir)
}

// PurgeChapter deletes node id from the trash for good, with the
// snapshots of it and of what it held.
func (s *Store) PurgeChapter(id string) error {
	unlock, err := s.lock()
	if err != nil {
		return err
	}
	defer unlock()
	items, err := s.trash()
	if err != nil {
		return err
	}
	i := slices.IndexFunc(items, func(it Trashed) bool { return it.ID == id })
	if i < 0 {
		return &TrashMissingError{ID: id}
	}
	held, err := s.heldIDs(items[i])
	if err != nil {
		return err
	}
	// A damaged auxiliary outline must not prevent emptying the trash.
	// Leave it untouched; dangling chapter references display in the tray.
	plan, _ := s.plan()
	changed := false
	for i := range plan.Cards {
		if plan.Cards[i].Chapter == id || slices.Contains(held, plan.Cards[i].Chapter) {
			plan.Cards[i].Chapter, changed = "", true
		}
	}
	if changed {
		if err = s.writePlan(plan); err != nil {
			return err
		}
	}
	for _, h := range append(held, id) {
		if validID(h) != nil {
			continue
		}
		p, err := s.path(historyDir, h)
		if err != nil {
			return err
		}
		if err = os.RemoveAll(p); err != nil {
			return err
		}
	}
	p, err := s.path(trashDir, id)
	if err != nil {
		return err
	}
	return os.RemoveAll(p)
}
