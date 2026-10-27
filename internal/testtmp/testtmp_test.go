package testtmp

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestTemporaryFilesUseMain(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root, err := repositoryRoot(cwd)
	if err != nil {
		t.Fatal(err)
	}
	type usage struct{ temporary, main bool }
	packages := map[string]usage{}
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if path == root {
				return nil
			}
			name := entry.Name()
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "prototype" || path == filepath.Join(root, "internal", "testtmp") {
				return filepath.SkipDir
			}
			if _, err := os.Stat(filepath.Join(path, "go.mod")); err == nil {
				return filepath.SkipDir
			} else if !os.IsNotExist(err) {
				return err
			}
			return nil
		}
		if !strings.HasSuffix(entry.Name(), "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		dir := filepath.Dir(path)
		u := packages[dir]
		source := string(data)
		for _, call := range []string{"TempDir(", "MkdirTemp(", "CreateTemp("} {
			u.temporary = u.temporary || strings.Contains(source, call)
		}
		u.main = u.main || strings.Contains(source, "testtmp.Main(")
		packages[dir] = u
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var missing []string
	for dir, u := range packages {
		if u.temporary && !u.main {
			rel, err := filepath.Rel(root, dir)
			if err != nil {
				t.Fatal(err)
			}
			missing = append(missing, filepath.ToSlash(rel))
		}
	}
	slices.Sort(missing)
	if len(missing) > 0 {
		t.Fatalf("packages using temporary files without testtmp.Main: %s\nAdd func TestMain(m *testing.M) { testtmp.Main(m) } to each package's main_test.go.", strings.Join(missing, ", "))
	}
}
