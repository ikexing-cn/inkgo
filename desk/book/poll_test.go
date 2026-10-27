package book

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPollChapterChangesVersionOnlyWithTree(t *testing.T) {
	s := makeWork(t)
	id, err := s.NewChapter("", "章", false)
	if err != nil {
		t.Fatal(err)
	}
	b := Open(s)
	v := b.Version()
	if _, err := b.PollChapter(id); err != nil || b.Version() != v {
		t.Fatal("unchanged tree changed version", err)
	}
	if err := os.WriteFile(filepath.Join(s.Root(), "chapters", "章.txt"), []byte("外面改了正文"), 0644); err != nil {
		t.Fatal(err)
	}
	st, err := b.PollChapter(id)
	if err != nil || st.Size == 0 || b.Version() != v {
		t.Fatal("text change changed tree version", st, err)
	}
	if err := os.WriteFile(filepath.Join(s.Root(), "chapters", "新章.txt"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := b.PollChapter(id); err != nil || b.Version() != v+1 {
		t.Fatal("new chapter did not change version", err)
	}
	// A failed scan leaves the last usable tree in place.
	if err := os.WriteFile(filepath.Join(s.Root(), "work.yaml"), []byte("[broken"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := b.PollChapter(id); err == nil {
		t.Fatal("broken metadata was accepted")
	}
	if _, ok := b.Tree().Node(id); !ok || b.Version() != v+1 {
		t.Fatal("failed poll replaced the tree")
	}
}
