package desk

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The store's methods that read or write the book's content. desk goes
// through Book for them; the methods left off are the prose (ReadChapter,
// Texts, Pieces, ChapterStamp, which Book does not hold), and what is not
// the book's content: Work and SaveWork, talks, records, snapshots, the
// trash list, preferences, and closing.
var storeBookMethods = map[string]bool{
	"Chapters": true, "Counts": true, "Marks": true, "Setting": true,
	"Plan": true, "Outlines": true, "OutlinesOf": true,
	"Checks": true, "ReadCheck": true,

	"WriteChapter": true, "WriteChapterOver": true,
	"WriteOutline": true, "WriteCheck": true, "WriteCheckOver": true,
	"NewChapter": true, "RenameChapter": true, "MoveChapter": true,
	"TrashChapter": true, "RestoreChapter": true, "PurgeChapter": true,
	"Revise": true, "Unrevise": true, "RemoveCategory": true,

	"SaveCard": true, "DeleteCard": true, "PlaceCard": true,
	"DropCard": true, "SaveTime": true, "SaveLine": true,
	"MoveLine": true, "DeleteLine": true,
}

// Check the method's declaring type, not a variable's spelling: aliases,
// embedded stores and method values must obey the same content boundary.
func bookBypasses(info *types.Info) []token.Pos {
	var found []token.Pos
	for expr, sel := range info.Selections {
		method, ok := sel.Obj().(*types.Func)
		if !ok || method.Pkg() == nil || method.Pkg().Path() != "writer/workspace" || !storeBookMethods[method.Name()] {
			continue
		}
		recv := method.Type().(*types.Signature).Recv().Type()
		if ptr, ok := recv.(*types.Pointer); ok {
			recv = ptr.Elem()
		}
		if named, ok := recv.(*types.Named); ok && named.Obj().Name() == "Store" {
			found = append(found, expr.Pos())
		}
	}
	return found
}

// Go's export data supplies dependencies without adding a package-loading
// framework. GoFiles respects build constraints; test fixtures are excluded.
func TestDeskLeavesTheBooksContentToBook(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", "-export", "-json", "writer/desk/...").CombinedOutput()
	if err != nil {
		t.Fatal(err, string(out))
	}
	type sourcePackage struct {
		ImportPath, Dir, Export string
		GoFiles                 []string
	}
	exports := map[string]string{}
	var packages []sourcePackage
	decoder := json.NewDecoder(bytes.NewReader(out))
	for decoder.More() {
		var p sourcePackage
		if err := decoder.Decode(&p); err != nil {
			t.Fatal(err)
		}
		exports[p.ImportPath] = p.Export
		if (p.ImportPath == "writer/desk" || strings.HasPrefix(p.ImportPath, "writer/desk/")) && p.ImportPath != "writer/desk/book" {
			packages = append(packages, p)
		}
	}
	if len(packages) == 0 {
		t.Fatal("no desk packages checked")
	}
	fset := token.NewFileSet()
	imports := importer.ForCompiler(fset, "gc", func(path string) (io.ReadCloser, error) {
		return os.Open(exports[path])
	})
	check := func(t *testing.T, path string, files []*ast.File) []token.Pos {
		t.Helper()
		info := &types.Info{Selections: map[*ast.SelectorExpr]*types.Selection{}}
		cfg := types.Config{Importer: imports}
		if _, err := cfg.Check(path, fset, files, info); err != nil {
			t.Fatal(err)
		}
		return bookBypasses(info)
	}
	for _, p := range packages {
		t.Run(p.ImportPath, func(t *testing.T) {
			var files []*ast.File
			for _, name := range p.GoFiles {
				f, err := parser.ParseFile(fset, filepath.Join(p.Dir, name), nil, 0)
				if err != nil {
					t.Fatal(err)
				}
				files = append(files, f)
			}
			for _, pos := range check(t, p.ImportPath, files) {
				t.Errorf("%s: Store content goes through Book", fset.Position(pos))
			}
		})
	}
	// Deliberately forbidden accesses, including in a page package. Valid
	// Book methods and uncached Store reads must remain available.
	for _, tc := range []struct {
		body string
		want int
	}{
		{`_, _ = s.Chapters()`, 1},
		{`alias := s; _, _ = alias.Plan()`, 1},
		{`read := s.Setting; _, _ = read()`, 1},
		{`read := (*workspace.Store).Checks; _, _ = read(s)`, 1},
		{`wrapped := struct{ *workspace.Store }{s}; _, _ = wrapped.Counts()`, 1},
		{`_ = b.Tree(); _ = b.Plan(); _, _ = s.ReadChapter("chapter")`, 0},
	} {
		t.Run(tc.body, func(t *testing.T) {
			src := fmt.Sprintf(`package probe
import ("writer/workspace"; "writer/desk/book")
func probe(s *workspace.Store, b *book.Book) { %s }
`, tc.body)
			f, err := parser.ParseFile(fset, "probe.go", src, 0)
			if err != nil {
				t.Fatal(err)
			}
			if got := len(check(t, "writer/desk/boundaryprobe", []*ast.File{f})); got != tc.want {
				t.Fatalf("found %d content bypasses, want %d", got, tc.want)
			}
		})
	}
}
