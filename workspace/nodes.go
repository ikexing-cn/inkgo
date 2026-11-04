package workspace

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
)

// NewChapter adds a document, or a folder, titled title at the end of the
// folder parent ("" for the top) and returns its ID.
func (s *Store) NewChapter(parent, title string, folder bool) (string, error) {
	unlock, err := s.lock()
	if err != nil {
		return "", err
	}
	defer unlock()
	t, err := s.tree()
	if err != nil {
		return "", err
	}
	p, err := t.folderOf(parent)
	if err != nil {
		return "", err
	}
	title = strings.TrimSpace(title)
	if title == "" {
		return "", ErrTitleEmpty
	}
	used, err := s.usedIDs(t)
	if err != nil {
		return "", err
	}
	n := &Node{ID: freshID(used), Title: title, Folder: folder, Parent: p, dir: childDir(p)}
	n.file = fileName(title, takenIn(p, nil))
	if folder {
		path, err := s.nodePath(n, "")
		if err != nil {
			return "", err
		}
		if err = os.Mkdir(path, 0755); err != nil {
			return "", err
		}
	} else {
		path, err := s.nodePath(n, textExt)
		if err != nil {
			return "", err
		}
		if err = atomicWrite(path, nil, true); err != nil {
			return "", err
		}
	}
	p.Children = append(p.Children, n)
	return n.ID, s.writeOrder(p)
}

// rename moves a file; a test makes it fail.
var rename = os.Rename

// moveAll moves each file of from to the path beside it in to, in order.
// Only the first has to be there. When one cannot be moved, those moved
// before it go back, so that a node is never half in one place.
func moveAll(from, to []string) error {
	for i := range from {
		err := rename(from[i], to[i])
		if err == nil || i > 0 && errors.Is(err, os.ErrNotExist) {
			continue
		}
		for j := i - 1; j >= 0; j-- {
			if back := rename(to[j], from[j]); back != nil && !errors.Is(back, os.ErrNotExist) {
				return errors.Join(err, back)
			}
		}
		return err
	}
	return nil
}

// moveFiles moves n's files from its place to file in dir.
func (s *Store) moveFiles(n *Node, dir []string, file string) error {
	for _, ext := range companions(n) {
		from, err := s.nodePath(n, ext)
		if err != nil {
			return err
		}
		to, err := s.diskPath(append(slices.Clone(dir), file+ext)...)
		if err != nil {
			return err
		}
		if err = os.Rename(from, to); err != nil && !(ext != textExt && ext != "" && errors.Is(err, os.ErrNotExist)) {
			return err
		}
	}
	return nil
}

// RenameChapter retitles node id, renaming its files to match.
func (s *Store) RenameChapter(id, title string) error {
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
	title = strings.TrimSpace(title)
	if title == "" {
		return ErrTitleEmpty
	}
	file := fileName(title, takenIn(n.Parent, n))
	if file != n.file {
		if err = s.moveFiles(n, n.dir, file); err != nil {
			return err
		}
		n.file = file
	}
	n.Title = title
	return s.writeOrder(n.Parent)
}

// MoveChapter puts node id into the folder parent ("" for the top) at
// index among what it holds, counted without the node itself.
func (s *Store) MoveChapter(id, parent string, index int) error {
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
	to, err := t.folderOf(parent)
	if err != nil {
		return err
	}
	if to.Within(n) {
		return ErrMoveIntoSelf
	}
	from := n.Parent
	from.Children = slices.DeleteFunc(from.Children, func(x *Node) bool { return x == n })
	index = clampIndex(index, len(to.Children))
	if to == from {
		to.Children = slices.Insert(to.Children, index, n)
		return s.writeOrder(to)
	}
	// The entry goes in first and out last: a crash between leaves the
	// node where its files are, as reading the tree drops an entry whose
	// files are gone.
	file := fileName(n.Title, takenIn(to, nil))
	moved := &Node{ID: n.ID, Title: n.Title, Folder: n.Folder, Parent: to, file: file, dir: childDir(to)}
	to.Children = slices.Insert(to.Children, index, moved)
	if err = s.writeOrder(to); err != nil {
		return err
	}
	if err = s.moveFiles(n, moved.dir, file); err != nil {
		return err
	}
	return s.writeOrder(from)
}

func clampIndex(i, n int) int { return max(0, min(i, n)) }
