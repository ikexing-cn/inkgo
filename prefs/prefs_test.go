package prefs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// What the desk remembers of a work is written whole: the file is put in
// place of the last, with nothing left beside it.
func TestPrefsAreWrittenWhole(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".writer", "desk.json")
	p := Prefs{Chapter: "chapter"}
	if err := p.Save(path); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(b), p.Chapter) {
		t.Fatalf("%s, %v", b, err)
	}
	if _, err = os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("a file left beside it: %v", err)
	}
}
