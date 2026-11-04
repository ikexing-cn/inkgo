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

// labelParams maps a function or method name to the positions of its
// parameters named label or ending in Label: what is passed there is a test
// hook, as with an argument to .Label. Function literals bound to a name count.
type labelParams map[string][]int

func (lp labelParams) add(name string, ft *ast.FuncType) {
	i := 0
	for _, field := range ft.Params.List {
		for _, id := range field.Names {
			if id.Name == "label" || strings.HasSuffix(id.Name, "Label") {
				lp[name] = append(lp[name], i)
			}
			i++
		}
		if len(field.Names) == 0 {
			i++
		}
	}
}

func (lp labelParams) collect(f *ast.File) {
	ast.Inspect(f, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.FuncDecl:
			lp.add(v.Name.Name, v.Type)
		case *ast.AssignStmt:
			for i, lhs := range v.Lhs {
				if id, ok := lhs.(*ast.Ident); ok && i < len(v.Rhs) {
					if lit, ok := v.Rhs[i].(*ast.FuncLit); ok {
						lp.add(id.Name, lit.Type)
					}
				}
			}
		}
		return true
	})
}

// chineseLiterals follows local assignments into error constructors, even in
// prompt-only files. The allowlist never exempts an error argument.
func chineseLiterals(name string, source []byte, allowed []string, wholeFile bool, labels labelParams) []string {
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
	hooks := map[token.Pos]bool{}
	var markHook func(ast.Node)
	marked := map[string]bool{}
	markHook = func(n ast.Node) {
		ast.Inspect(n, func(child ast.Node) bool {
			switch v := child.(type) {
			case *ast.BasicLit:
				hooks[v.Pos()] = true
			case *ast.Ident:
				// A name assigned only to pass it on as a hook, like
				// hook := "章纲" … .Label(hook + "-" + id).
				if !marked[v.Name] {
					marked[v.Name] = true
					for _, rhs := range bindings[v.Name] {
						if _, call := rhs.(*ast.CallExpr); !call {
							markHook(rhs)
						}
					}
				}
			}
			return true
		})
	}
	ast.Inspect(f, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.CallExpr:
			if sel, ok := v.Fun.(*ast.SelectorExpr); ok {
				hook := sel.Sel.Name == "Label"
				if id, ok := sel.X.(*ast.Ident); ok {
					hook = hook || id.Name == "c" && sel.Sel.Name == "Key"
				}
				if hook {
					for _, arg := range v.Args {
						markHook(arg)
					}
				}
			}
			callee := ""
			switch fun := v.Fun.(type) {
			case *ast.SelectorExpr:
				callee = fun.Sel.Name
			case *ast.Ident:
				callee = fun.Name
			}
			for _, i := range labels[callee] {
				if i < len(v.Args) {
					markHook(v.Args[i])
				}
			}
		case *ast.KeyValueExpr:
			if id, ok := v.Key.(*ast.Ident); ok && (id.Name == "label" || id.Name == "Label") {
				markHook(v.Value)
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
		allow := wholeFile || hooks[lit.Pos()]
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

// sourceFiles lists the repository's non-test, non-generated Go files.
func sourceFiles(t *testing.T) map[string][]byte {
	files := map[string][]byte{}
	err := filepath.WalkDir("../..", func(file string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if strings.HasPrefix(entry.Name(), ".") && file != "../.." {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(file, ".go") || strings.HasSuffix(file, "_test.go") || strings.HasSuffix(file, "_gen.go") {
			return nil
		}
		data, err := os.ReadFile(file)
		files[strings.TrimPrefix(filepath.ToSlash(file), "../../")] = data
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func TestRepositoryChineseLiteralsAreExplicitlyAllowed(t *testing.T) {
	files := sourceFiles(t)
	labels := labelParams{}
	for name, data := range files {
		f, err := parser.ParseFile(token.NewFileSet(), name, data, 0)
		if err != nil {
			t.Fatal(err)
		}
		labels.collect(f)
	}
	for name, data := range files {
		rule := bookLanguageExceptions[name]
		whole := rule.reason != "" && rule.fragments == nil || strings.HasPrefix(name, "cmd/sample/")
		for _, failure := range chineseLiterals(name, data, rule.fragments, whole, labels) {
			t.Error(failure)
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
		if got := chineseLiterals("prompt.go", []byte(source), nil, true, nil); len(got) == 0 {
			t.Fatalf("missed error: %s", source)
		}
	}
	if got := chineseLiterals("prompt.go", []byte("package p; // 中文\nvar prompt=`中文`"), nil, true, nil); len(got) != 0 {
		t.Fatal(got)
	}
}

func TestRepositoryGuardRejectsUIStringsAndAllowsOnlySyntacticHooks(t *testing.T) {
	for _, source := range []string{
		`package p; var title="中文"`,
		`package p; func f(){ ui.Text(c,"中文") }`,
		"package p; var title=`中文`",
	} {
		if got := chineseLiterals("editor/view.go", []byte(source), nil, false, nil); len(got) != 1 || !strings.Contains(got[0], "editor/view.go:") {
			t.Fatal(source, got)
		}
	}
	for _, source := range []string{
		`package p; func f(){ ui.Box(c).Label("中文"+suffix) }`,
		`package p; func f(){ ui.Box(c.Key(fmt.Sprint("中文",id))) }`,
		`package p; var x=Entry{label:"中文"};var y=Entry{Label:"中文"}`,
		"package p; // 中文\nvar title=\"English\"",
	} {
		if got := chineseLiterals("hooks.go", []byte(source), nil, false, nil); len(got) != 0 {
			t.Fatal(source, got)
		}
	}
	if got := chineseLiterals("sample.go", []byte(`package p; var x=errors.New("中文")`), nil, true, nil); len(got) == 0 {
		t.Fatal("sample directory bypassed the error guard")
	}
}
