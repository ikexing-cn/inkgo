package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsProsePath(t *testing.T) {
	for _, tc := range []struct {
		path  string
		prose bool
	}{
		{"chapters/一.txt", true},
		{"chapters/卷/一.txt", true},
		{"chapters/一.outline.txt", false},
		{"chapters/卷.outline.txt", false},
		{"chapters/一.marks.json", false},
		{"trash/一.txt", false},
		{"chapters-other/一.txt", false},
		{"一.txt", false},
		{`chapters\一.txt`, false},
		{"chapters/一.TXT", false},
	} {
		if got := IsProsePath(tc.path); got != tc.prose {
			t.Fatalf("%q: got %v, want %v", tc.path, got, tc.prose)
		}
	}
}

func exists(t *testing.T, parts ...string) bool {
	t.Helper()
	_, err := os.Stat(filepath.Join(parts...))
	return err == nil
}

// Titles that clash on disk, letter case aside, or that file systems
// refuse, still save and read back as the author typed them.
func TestChapterTitlesFileSystemsRefuse(t *testing.T) {
	s := testStore(t)
	a := mustChapter(t, s, "", "Chapter", false)
	b := mustChapter(t, s, "", "chapter", false)
	c := mustChapter(t, s, "", "CON", false)
	d := mustChapter(t, s, "", ".order", false)
	e := mustChapter(t, s, "", "a/b\\c*?", false)
	f := mustChapter(t, s, "", "注.outline", false)
	ch := filepath.Join(s.Root(), "chapters")
	for _, name := range []string{"Chapter.txt", "chapter (2).txt", "CON_.txt", "．order.txt", "a／b＼c＊？.txt", "注.outline_.txt"} {
		if !exists(t, ch, name) {
			t.Errorf("no %s", name)
		}
	}
	tree, err := s.Chapters()
	if err != nil {
		t.Fatal(err)
	}
	for id, want := range map[string]string{a: "Chapter", b: "chapter", c: "CON", d: ".order", e: "a/b\\c*?", f: "注.outline"} {
		if n, ok := tree.Node(id); !ok || n.Title != want {
			t.Errorf("node %s: %+v", want, n)
		}
	}
	if got := shape(t, s); strings.Count(got, "\n") != 6 {
		t.Fatalf("an outline-like name was taken for a companion file:\n%s", got)
	}
}
