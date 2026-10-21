package workspace

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"writer/internal/strictjson"
)

// orderEntry is a node as its directory's .order records it.
type orderEntry struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	File   string `json:"file"`
	Folder bool   `json:"folder,omitempty"`
}

// tree reads the chapter tree, giving files without an entry nodes of
// their own and dropping entries without files, and saves each .order
// that changed.
func (s *Store) tree() (Tree, error) {
	root, err := s.diskPath()
	if err != nil {
		return Tree{}, err
	}
	if err = os.MkdirAll(root, 0755); err != nil {
		return Tree{}, err
	}
	t := Tree{Root: &Node{Folder: true}, byID: map[string]*Node{}}
	used, err := s.trashedIDs()
	if err != nil {
		return Tree{}, err
	}
	if err = s.readDir(t, t.Root, used); err != nil {
		return Tree{}, err
	}
	return t, nil
}

func (s *Store) readDir(t Tree, folder *Node, used map[string]bool) error {
	dir := childDir(folder)
	p, err := s.diskPath(dir...)
	if err != nil {
		return err
	}
	files, err := os.ReadDir(p)
	if err != nil {
		return err
	}
	folders, docs := map[string]bool{}, map[string]bool{}
	for _, f := range files {
		name := f.Name()
		switch {
		case strings.HasPrefix(name, "."):
		case f.Type()&os.ModeSymlink != 0:
			return fmt.Errorf("%s: symlinks are not allowed", filepath.Join(p, name))
		case f.IsDir():
			folders[name] = true
		case strings.HasSuffix(strings.ToLower(name), outlineExt):
		case strings.HasSuffix(strings.ToLower(name), textExt):
			docs[name[:len(name)-len(textExt)]] = true
		}
	}
	order, err := s.readOrder(dir)
	if err != nil {
		return err
	}
	changed := false
	add := func(e orderEntry) {
		if validID(e.ID) != nil || used[e.ID] {
			e.ID = freshID(used)
			changed = true
		}
		used[e.ID] = true
		if strings.TrimSpace(e.Title) == "" {
			e.Title = e.File
			changed = true
		}
		n := &Node{ID: e.ID, Title: e.Title, Folder: e.Folder, Parent: folder, file: e.File, dir: dir}
		folder.Children = append(folder.Children, n)
		t.byID[n.ID] = n
	}
	for _, e := range order {
		have := docs
		if e.Folder {
			have = folders
		}
		if !have[e.File] {
			changed = true // gone, or renamed outside: a new node below
			continue
		}
		delete(have, e.File)
		add(e)
	}
	for _, kind := range []struct {
		names  map[string]bool
		folder bool
	}{{folders, true}, {docs, false}} {
		var names []string
		for name := range kind.names {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			add(orderEntry{Title: name, File: name, Folder: kind.folder})
			changed = true
		}
	}
	if changed {
		if err = s.writeOrder(folder); err != nil {
			return err
		}
	}
	for _, n := range folder.Children {
		if n.Folder {
			if err = s.readDir(t, n, used); err != nil {
				return err
			}
		}
	}
	return nil
}

func freshID(used map[string]bool) string {
	id := NewID()
	for used[id] {
		id = NewID()
	}
	used[id] = true
	return id
}

func (s *Store) readOrder(dir []string) ([]orderEntry, error) {
	p, err := s.diskPath(append(slices.Clone(dir), orderFile)...)
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var order []orderEntry
	if err = strictjson.Decode(b, &order); err != nil {
		return nil, fmt.Errorf("%s: %w", p, err)
	}
	return order, nil
}

// writeOrder saves the order of what folder holds.
func (s *Store) writeOrder(folder *Node) error {
	order := []orderEntry{}
	for _, n := range folder.Children {
		order = append(order, orderEntry{ID: n.ID, Title: n.Title, File: n.file, Folder: n.Folder})
	}
	b, err := json.MarshalIndent(order, "", "  ")
	if err != nil {
		return err
	}
	p, err := s.diskPath(append(childDir(folder), orderFile)...)
	if err != nil {
		return err
	}
	return atomicWrite(p, append(b, '\n'), false)
}

// usedIDs returns the IDs of the tree and of the trash.
func (s *Store) usedIDs(t Tree) (map[string]bool, error) {
	used, err := s.trashedIDs()
	if err != nil {
		return nil, err
	}
	for id := range t.byID {
		used[id] = true
	}
	return used, nil
}
