package workspace

import "fmt"

// Node is a node of the chapter tree: a folder of nodes, or a document of
// prose. Either may have an outline.
type Node struct {
	ID       string
	Title    string
	Folder   bool
	Children []*Node
	Parent   *Node // nil for the root
	file     string
	dir      []string // the directory holding it, under chapters/
}

// Tree is the chapter tree as the disk had it when it was read.
type Tree struct {
	Root *Node
	byID map[string]*Node
}

func (t Tree) Node(id string) (*Node, bool) {
	n, ok := t.byID[id]
	return n, ok
}

// Walk calls fn with each node in reading order, folders before what they
// hold, and its depth from 0. A tree that was never read holds nothing.
func (t Tree) Walk(fn func(n *Node, depth int)) {
	if t.Root == nil {
		return
	}
	var walk func(ns []*Node, depth int)
	walk = func(ns []*Node, depth int) {
		for _, n := range ns {
			fn(n, depth)
			walk(n.Children, depth+1)
		}
	}
	walk(t.Root.Children, 0)
}

// Previous is the document before n in the order of the tree.
func (t Tree) Previous(n *Node) (*Node, bool) {
	var last, found *Node
	t.Walk(func(m *Node, _ int) {
		if m == n {
			found = last
		}
		if !m.Folder {
			last = m
		}
	})
	return found, found != nil
}

// Within reports whether n is ancestor or inside it.
func (n *Node) Within(ancestor *Node) bool {
	for p := n; p != nil; p = p.Parent {
		if p == ancestor {
			return true
		}
	}
	return false
}

// Volume is the folder holding n, or nil when its parent is the tree root.
func (n *Node) Volume() *Node {
	vol := n.Parent
	if vol != nil && vol.Parent == nil {
		vol = nil // the root holds the chapters of no volume
	}
	return vol
}

func (s *Store) Chapters() (Tree, error) {
	unlock, err := s.lock()
	if err != nil {
		return Tree{}, err
	}
	defer unlock()
	return s.tree()
}

// ChaptersAndStamp reads the tree and one document's stamp with one scan.
// Empty, missing and folder IDs return a zero stamp; callers use the tree
// to distinguish them. Any read failure returns no tree and no stamp.
func (s *Store) ChaptersAndStamp(id string) (Tree, Stamp, error) {
	unlock, err := s.lock()
	if err != nil {
		return Tree{}, Stamp{}, err
	}
	defer unlock()
	t, err := s.tree()
	if err != nil {
		return Tree{}, Stamp{}, err
	}
	var st Stamp
	if n, ok := t.Node(id); ok && !n.Folder {
		st, err = s.stamp(n)
		if err != nil {
			return Tree{}, Stamp{}, err
		}
	}
	return t, st, nil
}

// folderOf returns the folder id, the root for "".
func (t Tree) folderOf(id string) (*Node, error) {
	if id == "" {
		return t.Root, nil
	}
	n, ok := t.byID[id]
	if !ok {
		return nil, fmt.Errorf("chapters: unknown node %q", id)
	}
	if !n.Folder {
		return nil, fmt.Errorf("chapters: %q is not a folder", n.Title)
	}
	return n, nil
}
