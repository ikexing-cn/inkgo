package desk

import (
	"go/build"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestLoreImportsOnlyPublicHostContracts(t *testing.T) {
	err := filepath.WalkDir("lore", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		for _, imp := range file.Imports {
			pkg, _ := strconv.Unquote(imp.Path.Value)
			switch pkg {
			case "writer/ext", "writer/ext/hostapi", "writer/desk/kit", "writer/desk/book", "writer/workspace", "writer/locales":
				continue
			}
			if strings.HasPrefix(pkg, "github.com/egoist/mygo/") {
				continue
			}
			if p, err := build.Default.Import(pkg, "", build.FindOnly); err == nil && p.Goroot {
				continue
			}
			t.Errorf("%s imports %s", path, pkg)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestDeskReachesLoreOnlyInAssembly(t *testing.T) {
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path == "lore" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") || path == "lore_bridge.go" {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		for _, imp := range file.Imports {
			pkg, _ := strconv.Unquote(imp.Path.Value)
			if pkg == "writer/desk/lore" || strings.HasPrefix(pkg, "writer/desk/lore/") {
				t.Errorf("%s imports %s outside assembly", path, pkg)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
