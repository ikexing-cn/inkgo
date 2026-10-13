package desk

import (
	"os"
	"path/filepath"
	"testing"
)

// TestMain keeps a first shelf from finding the author's own works, and
// gives the tests a temporary folder inside the repository, .tmp/desk-tests.
//
// The folder is there for go test's memory of a passed run. Before it
// trusts a kept result it goes through every file the tests touched, some
// thirty thousand lines of them here. A file inside the module it only
// looks at. For one outside, as the system's temporary folder is, it first
// resolves the links of the whole path, to see whether it is inside after
// all: on Windows that took 25 seconds, longer than running the tests.
func TestMain(m *testing.M) {
	own, _ := filepath.Abs(filepath.Join("..", ".tmp", "desk-tests"))
	if err := os.MkdirAll(own, 0o755); err != nil {
		panic(err)
	}
	for _, name := range []string{"TMP", "TEMP", "TMPDIR"} {
		os.Setenv(name, own)
	}
	dir, err := os.MkdirTemp("", "library")
	if err != nil {
		panic(err)
	}
	Library = func() string { return dir }
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
