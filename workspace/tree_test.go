package workspace

import (
	"strings"
	"testing"
)

// shape draws the tree as titles indented by depth.
func shape(t *testing.T, s *Store) string {
	t.Helper()
	tree, err := s.Chapters()
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	tree.Walk(func(n *Node, depth int) {
		b.WriteString(strings.Repeat("  ", depth) + n.Title)
		if n.Folder {
			b.WriteString("/")
		}
		b.WriteString("\n")
	})
	return b.String()
}

// A tree that was never read holds nothing, and can be walked.
func TestATreeNeverReadHoldsNothing(t *testing.T) {
	var tree Tree
	tree.Walk(func(*Node, int) { t.Fatal("a node in a tree never read") })
	if _, ok := tree.Node("x"); ok {
		t.Fatal("a node in a tree never read")
	}
}

func TestPrevious(t *testing.T) {
	root := &Node{Folder: true}
	volume := &Node{ID: "v", Folder: true, Parent: root}
	one := &Node{ID: "one", Parent: volume}
	nested := &Node{ID: "nested", Folder: true, Parent: volume}
	two := &Node{ID: "two", Parent: nested}
	three := &Node{ID: "three", Parent: root}
	root.Children = []*Node{volume, three}
	volume.Children = []*Node{one, nested}
	nested.Children = []*Node{two}
	tree := Tree{Root: root}
	for _, c := range []struct{ node, want *Node }{
		{volume, nil}, {one, nil}, {nested, one}, {two, one}, {three, two},
		{root, nil}, {nil, nil}, {&Node{ID: "three"}, nil},
	} {
		if got, ok := tree.Previous(c.node); got != c.want || ok != (c.want != nil) {
			t.Errorf("Previous(%p) = %p, %v; want %p", c.node, got, ok, c.want)
		}
	}
	if got, ok := (Tree{}).Previous(one); got != nil || ok {
		t.Fatalf("unread tree = %p, %v", got, ok)
	}
}

func TestVolume(t *testing.T) {
	root := &Node{Folder: true}
	volume := &Node{ID: "v", Folder: true, Parent: root}
	nested := &Node{ID: "nested", Folder: true, Parent: volume}
	for _, c := range []struct{ node, want *Node }{
		{root, nil},
		{volume, nil},
		{&Node{Parent: root}, nil},
		{&Node{Parent: volume}, volume},
		{&Node{Parent: nested}, nested},
	} {
		if got := c.node.Volume(); got != c.want {
			t.Errorf("Volume(%p) = %p; want %p", c.node, got, c.want)
		}
	}
}
