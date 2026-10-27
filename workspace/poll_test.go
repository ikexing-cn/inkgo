package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestChaptersAndStampTracksCurrentPaths(t *testing.T) {
	s := testStore(t)
	folder := mustChapter(t, s, "", "卷", true)
	id := mustChapter(t, s, "", "章", false)
	want, err := s.WriteChapter(id, "正文", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.MoveChapter(id, folder, 0); err != nil {
		t.Fatal(err)
	}
	tree, st, err := s.ChaptersAndStamp(id)
	if err != nil {
		t.Fatal(err)
	}
	n, ok := tree.Node(id)
	if !ok || n.Parent.ID != folder || st.Size != want.Size || !st.Mod.Equal(want.Mod) {
		t.Fatalf("moved chapter: node=%+v stamp=%+v", n, st)
	}
	for _, key := range []string{"", "missing", folder} {
		tree, st, err := s.ChaptersAndStamp(key)
		if err != nil || tree.Root == nil || st != (Stamp{}) {
			t.Fatalf("non-document %q: tree=%+v stamp=%+v err=%v", key, tree, st, err)
		}
	}
	if err := os.Remove(filepath.Join(s.Root(), "chapters", "卷", "章.txt")); err != nil {
		t.Fatal(err)
	}
	tree, st, err = s.ChaptersAndStamp(id)
	if _, ok := tree.Node(id); ok || err != nil || st != (Stamp{}) {
		t.Fatalf("removed chapter: stamp=%+v err=%v", st, err)
	}
}
