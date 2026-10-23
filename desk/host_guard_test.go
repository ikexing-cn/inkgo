package desk

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestFeatureSourcesDoNotNameApp(t *testing.T) {
	for _, name := range []string{"export.go", "plan.go", "plan_layout.go"} {
		f, err := parser.ParseFile(token.NewFileSet(), name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && id.Name == "App" {
				t.Errorf("%s names App; feature state and actions use host", name)
			}
			return true
		})
	}
}
