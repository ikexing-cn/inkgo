package shelf

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"writer/workspace"
)

func TestDescribeAndShelfWorkDecisions(t *testing.T) {
	root := filepath.Join(t.TempDir(), "book")
	w, err := workspace.Create(root, "雨城", "网文")
	if err != nil {
		t.Fatal(err)
	}
	id, err := w.NewChapter("", "第一章", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = w.WriteChapter(id, "一二三", nil); err != nil {
		t.Fatal(err)
	}
	w.Close()
	var s Shelf
	b, added, err := s.Add(root)
	if err != nil || !added || b.Name != "雨城" || b.Genre != "网文" || b.Words != 3 || len(s.Books) != 1 {
		t.Fatal(b, added, err)
	}
	if _, added, err = s.Add(filepath.Join(root, ".")); err != nil || added || len(s.Books) != 1 {
		t.Fatal("duplicate", added, err)
	}
	if !SameFolder(root, filepath.Join(root, ".")) || SameFolder(root, root+"-other") {
		t.Fatal("folder identity")
	}
	op := time.Date(2026, 10, 22, 9, 0, 0, 0, time.UTC)
	s.Books[0].Opened = op
	s.Books[0].MovedBackup = []byte("{\"dir\":\"backup\"}")
	moved := root + "-moved"
	if err = os.Rename(root, moved); err != nil {
		t.Fatal(err)
	}
	if err = s.RelocateWork(root, moved); err != nil {
		t.Fatal(err)
	}
	if len(s.Books) != 1 || s.Books[0].Path != moved || !s.Books[0].Opened.Equal(op) || string(s.Books[0].MovedBackup) != "{\"dir\":\"backup\"}" {
		t.Fatal(s.Books)
	}
	s.Remove(moved)
	if len(s.Books) != 0 || !workspace.IsWork(moved) {
		t.Fatal("forget removed files")
	}
}

func TestShelfWorkFailuresDoNotChangeShelf(t *testing.T) {
	var s Shelf
	s.Put(Book{Path: filepath.Join(t.TempDir(), "kept"), Name: "保留"})
	before := append([]Book(nil), s.Books...)
	missing := filepath.Join(t.TempDir(), "missing")
	broken := work(t, filepath.Join(t.TempDir(), "broken"))
	for _, dir := range []string{missing, broken} {
		if _, _, err := s.Add(dir); err == nil {
			t.Fatal("added invalid work", dir)
		}
		if err := s.RelocateWork(s.Books[0].Path, dir); err == nil {
			t.Fatal("relocated to invalid work", dir)
		}
		if !reflect.DeepEqual(s.Books, before) {
			t.Fatal("failure changed shelf")
		}
	}
	if _, _, err := s.Add(missing); err == nil || !strings.Contains(err.Error(), "no work.yaml") {
		t.Fatal(err)
	}
}

func TestRelocateToExistingWork(t *testing.T) {
	root := filepath.Join(t.TempDir(), "work")
	w, err := workspace.Create(root, "当前名字", "网文")
	if err != nil {
		t.Fatal(err)
	}
	w.Close()
	var s Shelf
	s.Put(Book{Path: root + "-old", Name: "旧位置"})
	op := time.Date(2026, 10, 22, 9, 0, 0, 0, time.UTC)
	s.Put(Book{Path: root, Name: "旧名字", Opened: op, MovedBackup: []byte("{\"dir\":\"keep\"}")})
	if err = s.RelocateWork(root+"-old", root); err != nil {
		t.Fatal(err)
	}
	if len(s.Books) != 1 || s.Books[0].Name != "当前名字" || !s.Books[0].Opened.Equal(op) || string(s.Books[0].MovedBackup) != "{\"dir\":\"keep\"}" {
		t.Fatal(s.Books)
	}
}
