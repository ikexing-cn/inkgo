package workspace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Prefetch may finish out of order; ID repair and error reporting still
// follow the author's reading order, including across batches and depths.
func TestDirectoryReadsKeepIDRepairAndErrorsInOrder(t *testing.T) {
	s := testStore(t)
	id := NewID()
	for i := range 6 {
		dir := filepath.Join(s.Root(), chaptersDir, fmt.Sprintf("卷%d", i))
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "章.txt"), []byte("正文"), 0644); err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal([]orderEntry{{ID: id, Title: "章", File: "章"}})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, orderFile), data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	// A deeper externally added chapter must also be reconciled and stay stable.
	deep := filepath.Join(s.Root(), chaptersDir, "卷0", "子卷")
	if err := os.Mkdir(deep, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(deep, "外稿.txt"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	tree, err := s.Chapters()
	if err != nil {
		t.Fatal(err)
	}
	first, ok := tree.Node(id)
	if !ok || first.Parent.Title != "卷0" {
		t.Fatal("first duplicate did not keep its ID", first)
	}
	ids := map[string]string{}
	tree.Walk(func(n *Node, _ int) {
		if ids[n.ID] != "" {
			t.Fatal("duplicate ID survived", n.ID)
		}
		ids[n.ID] = n.Title
	})
	again, err := s.Chapters()
	if err != nil {
		t.Fatal(err)
	}
	again.Walk(func(n *Node, _ int) {
		if ids[n.ID] != n.Title {
			t.Fatal("repaired ID changed on the next read", n.ID)
		}
	})
	for _, name := range []string{"卷1", "卷3", "卷5"} {
		if err := os.WriteFile(filepath.Join(s.Root(), chaptersDir, name, orderFile), []byte("[broken"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.Chapters(); err == nil || !strings.Contains(err.Error(), filepath.Join("卷1", orderFile)) {
		t.Fatal("did not report the first broken directory in reading order", err)
	}
}

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
