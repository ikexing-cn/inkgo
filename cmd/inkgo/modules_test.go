package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"writer/ext/export/exporttext"

	"writer/ext"
	"writer/ext/backup"
	"writer/ext/cats"
	"writer/ext/check"
	"writer/ext/context"
	continuation "writer/ext/continue"
	"writer/ext/draft"
	"writer/ext/export"
	"writer/ext/find"
	"writer/ext/focus"
	"writer/ext/goal"
	"writer/ext/hooks"
	"writer/ext/lore"
	"writer/ext/marks"
	"writer/ext/names"
	"writer/ext/notes"
	"writer/ext/outline"
	"writer/ext/phone"
	"writer/ext/propose"
	"writer/ext/snaps"
	"writer/ext/talk"
	"writer/ext/trash"
	"writer/ext/usage"
)

func testModules() []ext.Module {
	mod, _ := backup.New(false)
	return []ext.Module{focus.Module, goal.Module, names.Module, find.Module, notes.Module, lore.Module, outline.Module, hooks.Module, talk.Module, continuation.Module, draft.Module, propose.Module, check.Module, usage.Module, snaps.Module, trash.Module, cats.Module, mod, export.Module, phone.Module, context.Module, marks.Module}
}

func TestApplicationAssemblyIncludesExportFeature(t *testing.T) {
	for _, m := range modules {
		if m.ID == "export" {
			if m.Kind != ext.Feature || m.Name() != exporttext.ModExport() || m.Desc() != exporttext.ModExportAbout() {
				t.Fatal("incorrect export metadata")
			}
			return
		}
	}
	t.Fatal("export missing from application assembly")
}

func TestApplicationListsImplementedExtensionsInPrototypeGroupOrder(t *testing.T) {
	var ids []string
	for _, m := range assembledModules("", "") {
		ids = append(ids, m.ID)
	}
	want := []string{"focus", "goal", "names", "find", "notes", "lore", "outline", "hooks", "talk", "continue", "draft", "propose", "check", "usage", "snaps", "trash", "cats", "backup", "export", "phone", "context", "marks", "ai"}
	if !slices.Equal(ids, want) {
		t.Fatal(ids)
	}
}

// Check registration, including providers that might be disabled at runtime.
func TestContextPointsHaveSingleBundledProvider(t *testing.T) {
	counts := map[string]int{"Outlines": 0, "Entries": 0, "Hooks": 0}
	for _, m := range assembledModules("", "") {
		files, err := filepath.Glob(filepath.Join("..", "..", "ext", m.ID, "*.go"))
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range files {
			if strings.HasSuffix(name, "_test.go") {
				continue
			}
			file, err := parser.ParseFile(token.NewFileSet(), name, nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			ast.Inspect(file, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok || len(call.Args) < 2 {
					return true
				}
				fn, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || fn.Sel.Name != "Contribute" {
					return true
				}
				pkg, ok := fn.X.(*ast.Ident)
				if !ok || pkg.Name != "ext" {
					return true
				}
				point, ok := call.Args[1].(*ast.SelectorExpr)
				if !ok {
					return true
				}
				pkg, ok = point.X.(*ast.Ident)
				if ok && pkg.Name == "contextapi" {
					if _, ok := counts[point.Sel.Name]; ok {
						counts[point.Sel.Name]++
					}
				}
				return true
			})
		}
	}
	for point, count := range counts {
		if count != 1 {
			t.Errorf("context.%s: %d providers, want 1", point, count)
		}
	}
}
