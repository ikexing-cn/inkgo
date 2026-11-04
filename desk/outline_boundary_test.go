package desk

import (
	"github.com/egoist/mygo/ui"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
	"writer/ai"
	"writer/ext"
	"writer/ext/hostapi"
)

func TestPagesMergeHostAndContributionsByOrderHideUnavailableAndFallBackToWriting(t *testing.T) {
	available := true
	mod := ext.Module{ID: "fakepage", Setup: func(r *ext.Registrar) {
		ext.Contribute(r, hostapi.Pages, "page", hostapi.Page{Title: func() string { return "Fake page" }, Order: 25, Available: func() bool { return available }, Draw: func(c *ui.Context) { ui.Text(c, "Fake body") }})
	}}
	a := New(ai.New(""), append(testModules(), mod)...)
	defer a.Close()
	requireExtension(t, a.Open(extensionBook(t)))
	a.page = pageWrite
	tt := stillTester(a.View, 1266, 780)
	tt.Frame()
	var titles []string
	for _, id := range a.pageOrder {
		titles = append(titles, a.pages[id].title(a))
	}
	if !slices.Equal(titles, []string{"讨论", "正文", "Fake page", "大纲", "检查"}) {
		t.Fatal(titles)
	}
	available = false
	tt.Frame()
	if _, ok := tt.Find("Fake page"); ok {
		t.Fatal("unavailable page visible")
	}
	available = true
	tt.Frame()
	click(t, tt, "Fake page")
	if !tt.HasText("Fake body") {
		t.Fatal(tt.Texts())
	}
	requireExtension(t, a.extensions.Set("fakepage", false))
	tt.Frame()
	if a.page != pageWrite {
		t.Fatal("removed page did not fall back", a.page)
	}
}

func TestChapterHeadDrawsContributionsInRegistrationOrder(t *testing.T) {
	mod := ext.Module{ID: "fakehead", Setup: func(r *ext.Registrar) {
		for _, id := range []string{"first", "second"} {
			ext.Contribute(r, hostapi.ChapterHeadItems, id, hostapi.DrawItem{Draw: func(c *ui.Context) { ui.Text(c, id).Label(id) }})
		}
	}}
	a := New(ai.New(""), append(testModules(), mod)...)
	defer a.Close()
	requireExtension(t, a.Open(extensionBook(t)))
	id, err := a.book.NewChapter("", "第一章", false)
	if err != nil {
		t.Fatal(err)
	}
	a.openNode(id)
	a.page = pageWrite
	tt := stillTester(a.View, 1266, 780)
	tt.Frame()
	tt.Frame()
	first, ok := tt.Find("first")
	if !ok {
		t.Fatal(tt.Texts())
	}
	second, ok := tt.Find("second")
	if !ok || second.Y <= first.Y {
		t.Fatal(first, second)
	}
}

func TestChapterOutlineDraftSavesOnChapterChangeAndWorkLeave(t *testing.T) {
	a, tt, _ := writingFixture(t, "正文")
	a.book.Timing.SaveAfter = time.Hour
	id := a.write.node
	setChapterOutline(a, "换章前未保存")
	other, err := a.book.NewChapter("", "第二章", false)
	if err != nil {
		t.Fatal(err)
	}
	a.openNode(other)
	ch, err := a.Store.ReadChapter(id)
	if err != nil || ch.Outline != "换章前未保存" {
		t.Fatal(ch, err)
	}
	setChapterOutline(a, "离开前未保存")
	tt.Frame()
	a.leaveWork()
	ch, err = a.Store.ReadChapter(other)
	if err != nil || ch.Outline != "离开前未保存" {
		t.Fatal(ch, err)
	}
}

func TestDiscussionContextUsesUnsavedChapterOutlineDraft(t *testing.T) {
	a, _, _ := writingFixture(t, "正文")
	a.book.Timing.SaveAfter = time.Hour
	a.doc.attach = true
	setChapterOutline(a, "没有存盘的章纲")
	ch, err := a.Store.ReadChapter(a.write.node)
	if err != nil || ch.Outline == "没有存盘的章纲" {
		t.Fatal("draft already saved", err)
	}
	if got := a.beside(); got == nil || got.Outline != "没有存盘的章纲" {
		t.Fatal(got)
	}
}

func TestOutlineOnlyUsesHostAPIAndDeskOnlyCallsOutlineInAssembly(t *testing.T) {
	for _, dir := range []string{".", "outline"} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(dir, name), nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			if dir == "outline" {
				for _, imp := range file.Imports {
					if imp.Path.Value == "\"writer/desk\"" {
						t.Fatal(name, "imports desk")
					}
				}
				continue
			}
			if name == "outline_bridge.go" {
				continue
			}
			ast.Inspect(file, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				receiver, ok := sel.X.(*ast.SelectorExpr)
				if ok && receiver.Sel.Name == "plan" {
					t.Errorf("%s directly calls outline method %s", name, sel.Sel.Name)
				}
				return true
			})
		}
	}
}
