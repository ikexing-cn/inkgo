package desk

import (
	"os"
	"strings"
	"testing"
)

// What the desk remembers of a work is written whole: the file is put in
// place of the last, with nothing left beside it.
func TestPrefsAreWrittenWhole(t *testing.T) {
	a, _, _ := writingFixture(t, "原来的。")
	a.savePrefs()
	b, err := os.ReadFile(a.prefsPath())
	if err != nil || !strings.Contains(string(b), a.write.node) {
		t.Fatalf("%s, %v", b, err)
	}
	if _, err = os.Stat(a.prefsPath() + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("a file left beside it: %v", err)
	}
}
