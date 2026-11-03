package ext_test

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
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
	for _, allowed := range []string{"writer/ext", "writer/ext/hostapi", "writer/desk/kit", "writer/editor", "writer/locales"} {
		if imp == allowed {
			return nil
		}
	}
	if strings.HasPrefix(imp, "github.com/egoist/mygo/") {
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
			for _, decl := range file.Decls {
				gen, ok := decl.(*ast.GenDecl)
				if !ok {
					continue
				}
				for _, spec := range gen.Specs {
					v, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}
					for i, name := range v.Names {
						if name.Name != "Module" || i >= len(v.Values) {
							continue
						}
						lit, ok := v.Values[i].(*ast.CompositeLit)
						if !ok {
							return nil, fmt.Errorf("Module must declare its dependencies explicitly in %s", dir)
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
								return nil, fmt.Errorf("dependency list must be literal in %s", dir)
							}
							for _, item := range list.Elts {
								value, ok := item.(*ast.BasicLit)
								if !ok {
									return nil, fmt.Errorf("dependency ID must be literal in %s", dir)
								}
								id, err := strconv.Unquote(value.Value)
								if err != nil {
									return nil, err
								}
								deps = append(deps, id)
							}
						}
					}
				}
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
					if imp == "writer/shelf" || imp == "writer/workspace" || imp == "writer/desk" || strings.HasPrefix(imp, "writer/desk/") && imp != "writer/desk/kit" {
						t.Errorf("%s imports %s", pkg, imp)
					}
					if pkg == "writer/ext" {
						if p, err := build.Default.Import(imp, "", build.FindOnly); err != nil || !p.Goroot {
							t.Errorf("ext must use only standard library: %s", imp)
						}
					} else {
						allowed := imp == "writer/ext" || imp == "github.com/egoist/mygo/ui" || imp == "writer/desk/kit" || imp == "writer/editor"
						if !allowed {
							p, err := build.Default.Import(imp, "", build.FindOnly)
							if err != nil || !p.Goroot {
								t.Errorf("hostapi imports outside its contracts: %s", imp)
							}
						}
					}
				case strings.HasPrefix(pkg, "writer/desk"):
					if strings.HasPrefix(imp, "writer/ext/") && imp != "writer/ext/hostapi" {
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
