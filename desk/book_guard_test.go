package desk

import (
	"go/ast"
	"go/parser"
	"go/token"
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

// No source of desk reads or writes the book's content except through Book.
// Book's own package and the tests are not sources here.
func TestDeskLeavesTheBooksContentToBook(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			store, ok := sel.X.(*ast.SelectorExpr)
			if !ok || store.Sel.Name != "Store" {
				return true
			}
			if storeBookMethods[sel.Sel.Name] {
				t.Errorf("%s calls Store.%s; it goes through Book", name, sel.Sel.Name)
			}
			return true
		})
	}
}
