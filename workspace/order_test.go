package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

// Files the author adds or renames outside become nodes of their own, and
// entries whose files are gone drop out.
func TestChaptersChangedOutside(t *testing.T) {
	s := testStore(t)
	ch := filepath.Join(s.Root(), "chapters")
	one := mustChapter(t, s, "", "一", false)
	mustChapter(t, s, "", "二", false)
	if err := os.WriteFile(filepath.Join(ch, "三.txt"), []byte("　　外面写的。\r\n第二段"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(ch, "二.txt"), filepath.Join(ch, "贰.txt")); err != nil {
		t.Fatal(err)
	}
	if got := shape(t, s); got != "一\n三\n贰\n" {
		t.Fatalf("tree:\n%s", got)
	}
	tree, _ := s.Chapters()
	var three *Node
	tree.Walk(func(n *Node, _ int) {
		if n.Title == "三" {
			three = n
		}
	})
	c, err := s.ReadChapter(three.ID)
	if err != nil {
		t.Fatal(err)
	}
	if c.Text != "外面写的。\n第二段" {
		t.Fatalf("indent and CRLF kept: %q", c.Text)
	}
	again, _ := s.Chapters()
	if n, ok := again.Node(three.ID); !ok || n.Title != "三" {
		t.Fatal("a file added outside got a new ID each read")
	}
	if _, ok := again.Node(one); !ok {
		t.Fatal("the first node lost its ID")
	}
}
