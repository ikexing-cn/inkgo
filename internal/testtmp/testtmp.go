// Package testtmp gives tests a temporary folder inside the repository.
//
// The folder is there for go test's memory of a passed run. Before it
// trusts a kept result it goes through every file the tests touched.
// A file inside the module it only looks at. For one outside, as the
// system's temporary folder is, it first resolves the links of the whole
// path, to see whether it is inside after all: on Windows that took
// longer than running the tests.
//
// Each run has a folder of its own below .tmp/tests, gone when the run ends.
// go test also looks at when a folder the tests opened last changed, so with
// one folder shared, any other run of these tests would throw this run's
// kept result away.
package testtmp

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// Main runs the tests with their temporary files in a folder of their own.
// Call it from TestMain. It exits the process after removing the folder.
func Main(m *testing.M) {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	root, err := repositoryRoot(cwd)
	if err != nil {
		panic(err)
	}
	parent := filepath.Join(root, ".tmp", "tests")
	if err := os.MkdirAll(parent, 0o755); err != nil {
		panic(err)
	}
	own, err := os.MkdirTemp(parent, filepath.Base(cwd)+"-")
	if err != nil {
		panic(err)
	}
	for _, name := range []string{"TMP", "TEMP", "TMPDIR", "GOTMPDIR"} {
		os.Setenv(name, own)
	}
	code := m.Run()
	os.RemoveAll(own) // a folder left behind, as when a scanner holds a file, is only git's to ignore
	os.Exit(code)
}

func repositoryRoot(dir string) (string, error) {
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		} else if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("cannot find go.mod above %s", dir)
		}
		dir = parent
	}
}
