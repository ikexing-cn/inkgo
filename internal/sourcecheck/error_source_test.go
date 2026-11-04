package sourcecheck

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"unicode"
)

func hasHan(s string) bool {
	return strings.ContainsFunc(s, func(r rune) bool { return unicode.Is(unicode.Han, r) })
}

// chineseLiterals follows local assignments into error constructors, even in
// prompt-only files. The allowlist never exempts an error argument.
func chineseLiterals(name string, source []byte, allowed []string, wholeFile bool) []string {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, name, source, 0)
	if err != nil {
		return []string{err.Error()}
	}
	bindings := map[string][]ast.Expr{}
	errorTypes := map[string]bool{}
	ast.Inspect(f, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.FuncDecl:
			if v.Name.Name == "Error" && v.Recv != nil {
				t := v.Recv.List[0].Type
				if star, ok := t.(*ast.StarExpr); ok {
					t = star.X
				}
				if id, ok := t.(*ast.Ident); ok {
					errorTypes[id.Name] = true
				}
			}
		case *ast.AssignStmt:
			for i, lhs := range v.Lhs {
				if id, ok := lhs.(*ast.Ident); ok && i < len(v.Rhs) {
					bindings[id.Name] = append(bindings[id.Name], v.Rhs[i])
				}
			}
		case *ast.ValueSpec:
			for i, id := range v.Names {
				if i < len(v.Values) {
					bindings[id.Name] = append(bindings[id.Name], v.Values[i])
				}
			}
		}
		return true
	})
	bad := map[token.Pos]bool{}
	var taint func(ast.Node, map[string]bool)
	taint = func(n ast.Node, seen map[string]bool) {
		ast.Inspect(n, func(child ast.Node) bool {
			switch v := child.(type) {
			case *ast.BasicLit:
				if v.Kind == token.STRING {
					if s, e := strconv.Unquote(v.Value); e == nil && hasHan(s) {
						bad[v.Pos()] = true
					}
				}
			case *ast.Ident:
				if !seen[v.Name] {
					seen[v.Name] = true
					for _, rhs := range bindings[v.Name] {
						// An error returned by a store operation is not made from
						// its book-content arguments. Follow string formatting and
						// error construction, rather than arbitrary call results.
						if call, ok := rhs.(*ast.CallExpr); ok {
							follow := false
							switch fun := call.Fun.(type) {
							case *ast.SelectorExpr:
								if id, ok := fun.X.(*ast.Ident); ok {
									follow = id.Name == "fmt" || id.Name == "strings" || id.Name == "errors"
								}
							case *ast.Ident:
								follow = errorTypes[fun.Name] || fun.Name == "string"
							}
							if !follow {
								continue
							}
						}
						taint(rhs, seen)
					}
				}
			}
			return true
		})
	}
	ast.Inspect(f, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.ValueSpec:
			if id, ok := v.Type.(*ast.Ident); ok && errorTypes[id.Name] {
				for _, value := range v.Values {
					taint(value, map[string]bool{})
				}
			}
		case *ast.FuncDecl:
			if v.Name.Name == "Error" && v.Recv != nil && v.Body != nil {
				taint(v.Body, map[string]bool{})
			}
		case *ast.CallExpr:
			isError := false
			switch fun := v.Fun.(type) {
			case *ast.SelectorExpr:
				if id, ok := fun.X.(*ast.Ident); ok {
					isError = (id.Name == "errors" && fun.Sel.Name == "New") || (id.Name == "fmt" && fun.Sel.Name == "Errorf")
				}
			case *ast.Ident:
				isError = errorTypes[fun.Name]
			}
			if isError {
				for _, arg := range v.Args {
					taint(arg, map[string]bool{})
				}
			}
		case *ast.KeyValueExpr:
			if id, ok := v.Key.(*ast.Ident); ok && id.Name == "Err" {
				taint(v.Value, map[string]bool{})
			}
		}
		return true
	})
	var failures []string
	ast.Inspect(f, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		s, e := strconv.Unquote(lit.Value)
		if e != nil || !hasHan(s) {
			return true
		}
		allow := wholeFile
		for _, snippet := range allowed {
			if strings.Contains(s, snippet) {
				allow = true
			}
		}
		if bad[lit.Pos()] || !allow {
			failures = append(failures, fmt.Sprintf("%s: %s", fset.Position(lit.Pos()), lit.Value))
		}
		return true
	})
	return failures
}

func TestAIPackagesKeepChineseOnlyInBookPrompts(t *testing.T) {
	for _, pkg := range []string{"model", "ai", "inference", "writing", "discussion"} {
		files, err := filepath.Glob("../../" + pkg + "/*.go")
		if err != nil {
			t.Fatal(err)
		}
		for _, file := range files {
			if strings.HasSuffix(file, "_test.go") {
				continue
			}
			name := strings.TrimPrefix(filepath.ToSlash(file), "../../")
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			rule := bookLanguageExceptions[name]
			for _, failure := range chineseLiterals(name, data, rule.fragments, rule.reason != "" && rule.fragments == nil) {
				t.Error(failure)
			}
		}
	}
}

func TestChineseErrorGuardRejectsWhitelistedAndIndirectErrors(t *testing.T) {
	for _, source := range []string{
		`package p; import "errors"; var err=errors.New("中文")`,
		"package p; import \"fmt\"; var hint=`中文`; var err=fmt.Errorf(\"failed: %s\",hint)",
		`package p; type setup struct{Err error}; var err=setup{Err:build("中文")}`,
		`package p; type modelError string; func(e modelError)Error()string{return string(e)};var err=modelError("中文")`,
		`package p; type modelError string; func(e modelError)Error()string{return string(e)};const err modelError="中文"`,
		`package p; type failure struct{};func(e failure)Error()string{return "中文"}`,
	} {
		if got := chineseLiterals("prompt.go", []byte(source), nil, true); len(got) == 0 {
			t.Fatalf("missed error: %s", source)
		}
	}
	if got := chineseLiterals("prompt.go", []byte("package p; // 中文\nvar prompt=`中文`"), nil, true); len(got) != 0 {
		t.Fatal(got)
	}
}
