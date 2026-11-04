package ext_test

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// extensionImport checks public API imports against the owning Module's graph.
func extensionImport(pkg, imp string, deps []string) error {
	if !strings.HasPrefix(imp, "writer/") && !strings.Contains(strings.Split(imp, "/")[0], ".") {
		if p, err := build.Default.Import(imp, "", build.FindOnly); err == nil && p.Goroot {
			return nil
		}
	}
	for _, allowed := range []string{"writer/ext", "writer/ext/hostapi", "writer/desk/kit", "writer/editor", "writer/locales", "writer/workspace", "writer/desk/book", "writer/internal/atomicfile"} {
		if imp == allowed {
			return nil
		}
	}
	if strings.HasPrefix(imp, "github.com/egoist/mygo/") {
		return nil
	}
	owner := strings.Split(strings.TrimPrefix(pkg, "writer/ext/"), "/")[0]
	if strings.HasPrefix(imp, "writer/ext/"+owner+"/") {
		return nil
	}
	parts := strings.Split(imp, "/")
	if len(parts) == 4 && parts[0] == "writer" && parts[1] == "ext" && parts[3] == parts[2]+"api" {
		for _, id := range deps {
			if id == parts[2] {
				return nil
			}
		}
	}
	return fmt.Errorf("%s imports forbidden or undeclared dependency %s", pkg, imp)
}

func moduleDependencies(dir string) ([]string, error) {
	files, err := parser.ParseDir(token.NewFileSet(), dir, func(info fs.FileInfo) bool { return !strings.HasSuffix(info.Name(), "_test.go") }, 0)
	if err != nil {
		return nil, err
	}
	var deps []string
	for _, pkg := range files {
		for _, file := range pkg.Files {
			aliases := map[string]bool{}
			for _, imp := range file.Imports {
				path, _ := strconv.Unquote(imp.Path.Value)
				if path == "writer/ext" {
					name := "ext"
					if imp.Name != nil {
						name = imp.Name.Name
					}
					aliases[name] = true
				}
			}
			var scanErr error
			ast.Inspect(file, func(node ast.Node) bool {
				if scanErr != nil {
					return false
				}
				lit, ok := node.(*ast.CompositeLit)
				if !ok {
					return true
				}
				typ, ok := lit.Type.(*ast.SelectorExpr)
				if !ok || typ.Sel.Name != "Module" {
					return true
				}
				qualifier, ok := typ.X.(*ast.Ident)
				if !ok || !aliases[qualifier.Name] {
					return true
				}
				for _, element := range lit.Elts {
					kv, ok := element.(*ast.KeyValueExpr)
					if !ok {
						continue
					}
					key, ok := kv.Key.(*ast.Ident)
					if !ok || key.Name != "Requires" && key.Name != "Optional" {
						continue
					}
					list, ok := kv.Value.(*ast.CompositeLit)
					if !ok {
						scanErr = fmt.Errorf("dependency list must be literal in %s", dir)
						return false
					}
					for _, item := range list.Elts {
						value, ok := item.(*ast.BasicLit)
						if !ok {
							scanErr = fmt.Errorf("dependency ID must be literal in %s", dir)
							return false
						}
						id, err := strconv.Unquote(value.Value)
						if err != nil {
							scanErr = err
							return false
						}
						deps = append(deps, id)
					}
				}
				return true
			})
			if scanErr != nil {
				return nil, scanErr
			}
		}
	}
	return deps, nil
}

func TestProductionDependencyBoundaries(t *testing.T) {
	root := filepath.Clean("..")
	deps := map[string][]string{}
	for _, tree := range []string{"ext", "desk"} {
		err := filepath.WalkDir(filepath.Join(root, tree), func(dir string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() {
				return nil
			}
			p, err := build.Default.ImportDir(dir, 0)
			if _, ok := err.(*build.NoGoError); ok {
				return nil
			}
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(root, dir)
			pkg := "writer/" + filepath.ToSlash(rel)
			for _, imp := range p.Imports {
				switch {
				case pkg == "writer/ext" || pkg == "writer/ext/hostapi":
					if imp == "writer/shelf" || imp == "writer/desk" || strings.HasPrefix(imp, "writer/desk/") && imp != "writer/desk/kit" && imp != "writer/desk/book" {
						t.Errorf("%s imports %s", pkg, imp)
					}
					if pkg == "writer/ext" {
						if p, err := build.Default.Import(imp, "", build.FindOnly); err != nil || !p.Goroot {
							t.Errorf("ext must use only standard library: %s", imp)
						}
					} else {
						allowed := imp == "writer/ext" || imp == "github.com/egoist/mygo/ui" || imp == "writer/desk/kit" || imp == "writer/editor" || imp == "writer/workspace" || imp == "writer/desk/book"
						if !allowed {
							p, err := build.Default.Import(imp, "", build.FindOnly)
							if err != nil || !p.Goroot {
								t.Errorf("hostapi imports outside its contracts: %s", imp)
							}
						}
					}
				case strings.HasPrefix(pkg, "writer/desk"):
					if strings.HasPrefix(imp, "writer/ext/") && imp != "writer/ext/hostapi" && !deskContextImport(pkg, imp) {
						t.Errorf("%s imports concrete extension %s", pkg, imp)
					}
				default:
					id := strings.Split(strings.TrimPrefix(pkg, "writer/ext/"), "/")[0]
					if _, ok := deps[id]; !ok {
						list, err := moduleDependencies(filepath.Join(root, "ext", id))
						if err != nil {
							return err
						}
						deps[id] = list
					}
					if err := extensionImport(pkg, imp, deps[id]); err != nil {
						t.Error(err)
					}
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestPublicAPIImportWithoutDeclaredDependencyFails(t *testing.T) {
	imp := "writer/ext/context/contextapi"
	if extensionImport("writer/ext/fake", imp, nil) == nil {
		t.Fatal("undeclared API import accepted")
	}
	if err := extensionImport("writer/ext/fake", imp, []string{"context"}); err != nil {
		t.Fatal(err)
	}
	if extensionImport("writer/ext/fake", "writer/ext/context/internal", []string{"context"}) == nil {
		t.Fatal("private import accepted")
	}
}

func TestExtensionsMayImportOwnSubpackagesAndBookData(t *testing.T) {
	for _, imp := range []string{"writer/ext/export/format", "writer/ext/export/format/internal", "writer/workspace", "writer/desk/book", "writer/internal/atomicfile"} {
		if err := extensionImport("writer/ext/export/format", imp, nil); err != nil {
			t.Error(err)
		}
	}
}

func TestExtensionsCannotImportOtherPrivateSubpackages(t *testing.T) {
	if extensionImport("writer/ext/phone", "writer/ext/export/format", []string{"export"}) == nil {
		t.Fatal("private subpackage accepted")
	}
}

func TestExtensionsCannotImportDeskOrShelf(t *testing.T) {
	for _, imp := range []string{"writer/desk", "writer/shelf"} {
		if extensionImport("writer/ext/export", imp, nil) == nil {
			t.Errorf("accepted %s", imp)
		}
	}
}

func TestModuleDependenciesIncludeLiteralsReturnedFromFunctions(t *testing.T) {
	dir := t.TempDir()
	source := `package fake
import "writer/ext"
var Other = ext.Module{Optional: []string{"lore"}}
func New() ext.Module {
 return ext.Module{Requires: []string{"context"}, Optional: []string{"marks"}}
}
`
	if err := os.WriteFile(filepath.Join(dir, "module.go"), []byte(source), 0644); err != nil {
		t.Fatal(err)
	}
	deps, err := moduleDependencies(dir)
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(deps)
	if !slices.Equal(deps, []string{"context", "lore", "marks"}) {
		t.Fatal(deps)
	}
	if extensionImport("writer/ext/fake", "writer/ext/undeclared/undeclaredapi", deps) == nil {
		t.Fatal("undeclared import accepted")
	}
	if err := extensionImport("writer/ext/fake", "writer/ext/context/contextapi", deps); err != nil {
		t.Fatal(err)
	}
}

func TestModuleDependenciesRejectNonliteralListsInsideFunctions(t *testing.T) {
	for _, field := range []string{"Requires", "Optional"} {
		t.Run(field, func(t *testing.T) {
			dir := t.TempDir()
			source := "package fake\nimport \"writer/ext\"\nfunc New() ext.Module { return ext.Module{" + field + ": dependencies} }\n"
			if err := os.WriteFile(filepath.Join(dir, "module.go"), []byte(source), 0644); err != nil {
				t.Fatal(err)
			}
			if _, err := moduleDependencies(dir); err == nil {
				t.Fatal("nonliteral dependencies accepted")
			}
		})
	}
}

// deskContextImport is temporary: remove after discussion, drafting and continuation
// move out of desk. Only the host package may consume the context public contract.
func deskContextImport(pkg, imp string) bool {
	return pkg == "writer/desk" && imp == "writer/ext/context/contextapi"
}

func TestDeskContextExceptionDoesNotAllowConcreteOutline(t *testing.T) {
	if !deskContextImport("writer/desk", "writer/ext/context/contextapi") || deskContextImport("writer/desk", "writer/ext/outline") || deskContextImport("writer/desk/book", "writer/ext/context/contextapi") {
		t.Fatal("context exception escaped its named boundary")
	}
}
