package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckDataLivesInItsExtensionDirectoryAndOldBooksAreNotMigrated(t *testing.T) {
	s := testStore(t)
	id := mustChapter(t, s, "", "chapter", false)
	legacy := filepath.Join(s.Root(), "chapters", "chapter.check.json")
	if err := os.WriteFile(legacy, []byte(`{"of":"legacy"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if got, err := s.ReadCheck(id); err != nil || got.Of != "" {
		t.Fatal("legacy data was migrated", got, err)
	}
	want := Check{Of: "new"}
	if err := s.WriteCheck(id, want); err != nil {
		t.Fatal(err)
	}
	dir, err := s.ExtDir("check")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, id+".json")); err != nil {
		t.Fatal(err)
	}
	if err := s.TrashChapter(id); err != nil {
		t.Fatal(err)
	}
	if err := s.PurgeChapter(id); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, id+".json")); !os.IsNotExist(err) {
		t.Fatal("purged check remained", err)
	}
}
