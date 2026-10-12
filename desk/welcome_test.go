package desk

import (
	"os"
	"path/filepath"
	"testing"

	"writer/shelf"
	"writer/workspace"
)

func readShelf(t *testing.T, a *App) shelf.Shelf {
	t.Helper()
	s, fresh, err := shelf.Load(a.books.file)
	if err != nil || fresh {
		t.Fatal(fresh, err)
	}
	return s
}

// Works opened or made here stay on the shelf, last opened first, and the
// shelf remembers how the author likes to see them.
func TestShelfKeepsTheWorks(t *testing.T) {
	a, tt, _ := fixture(t)
	first := a.Store.Root()
	if s := readShelf(t, a); len(s.Books) != 1 || s.Books[0].Name != "测试作品" || s.Books[0].Opened.IsZero() {
		t.Fatalf("%+v", s.Books)
	}
	if err := tt.Click("切换作品"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := pick(tt, "新建作品…"); err != nil {
		t.Fatal(err, rows(tt))
	}
	tt.Frame()
	tt.Type("雨城来信")
	tt.Frame()
	tt.Frame()
	if want := filepath.Join(Library(), "雨城来信"); a.books.at != want {
		t.Fatalf("placed at %q, not %q", a.books.at, want)
	}
	if err := tt.Click("创建作品"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.Store == nil || a.Store.Work().Name != "雨城来信" || a.books.creating {
		t.Fatal("the new work is not open", a.Error)
	}
	s := readShelf(t, a)
	if r := s.Recent(); len(r) != 2 || r[0].Name != "雨城来信" || !sameFolder(r[1].Path, first) {
		t.Fatalf("%+v", r)
	}

	// The sidebar switches straight to another work.
	if err := tt.Click("切换作品"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := pick(tt, "测试作品"); err != nil {
		t.Fatal(err, rows(tt))
	}
	tt.Frame()
	if !sameFolder(a.Store.Root(), first) {
		t.Fatal("did not switch", a.Store.Root())
	}

	if err := tt.Click("切换作品"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := pick(tt, "全部作品…"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.Store != nil || !tt.HasText("我的作品") || !tt.HasText("雨城来信") {
		t.Fatal(tt.Texts())
	}
	if err := tt.Click("书架-卡片"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !readShelf(t, a).Cards {
		t.Fatal("cards not kept")
	}
	if err := tt.Click("更多-雨城来信"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := pick(tt, "从书架移除"); err != nil {
		t.Fatal(err, rows(tt))
	}
	tt.Frame()
	if _, shown := tt.Find("作品-雨城来信"); len(readShelf(t, a).Books) != 1 || shown {
		t.Fatalf("%+v", readShelf(t, a).Books)
	}
	if !shelf.IsWork(filepath.Join(Library(), "雨城来信")) {
		t.Fatal("removing from the shelf touched the folder")
	}

	// Opening the app again returns to the work last opened.
	b := NewWithConfig(a.ConfigFile)
	b.OpenLast()
	if b.Store == nil || !sameFolder(b.Store.Root(), first) {
		t.Fatal("did not reopen", b.Error)
	}
	b.Close()
}

// A work whose folder moved shows greyed until it is found again or taken
// off; renaming changes its name, not its folder.
func TestShelfFindsMovedWorks(t *testing.T) {
	a, tt, _ := fixture(t)
	root := a.Store.Root()
	a.closeWork()
	tt.Frame()
	moved := root + "-moved"
	if err := os.Rename(root, moved); err != nil {
		t.Fatal(err)
	}
	a.lookForBooks()
	tt.Frame()
	if !tt.HasText("找不到文件夹") {
		t.Fatal(tt.Texts())
	}
	if err := tt.Click("作品-测试作品"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.Store != nil {
		t.Fatal("opened a missing work")
	}
	a.relocate(root, moved)
	tt.Frame()
	if tt.HasText("找不到文件夹") || len(a.shelf.Books) != 1 || a.shelf.Books[0].Path != moved {
		t.Fatalf("%+v", a.shelf.Books)
	}

	if err := tt.Click("更多-测试作品"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := pick(tt, "重命名…"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	a.books.name = "雨夜"
	if err := tt.Click("保存作品名"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	s, err := workspace.Open(moved)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if s.Work().Name != "雨夜" || readShelf(t, a).Books[0].Name != "雨夜" || a.books.renaming != "" {
		t.Fatal("not renamed", a.Error)
	}

	a.addBook(t.TempDir())
	if a.Error == "" {
		t.Fatal("added a folder without a work")
	}
	a.removeBook(moved)
	tt.Frame()
	if !tt.HasText("从一个想法开始") {
		t.Fatal(tt.Texts())
	}
}

// A shelf that does not read is reported and left as it was.
func TestBrokenShelfIsNotOverwritten(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "providers.yaml")
	if err := os.WriteFile(shelf.File(config), []byte("{"), 0644); err != nil {
		t.Fatal(err)
	}
	a := NewWithConfig(config)
	if a.Error == "" {
		t.Fatal("no error")
	}
	s, err := workspace.Create(filepath.Join(dir, "work"), "雨夜", "网文")
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	if err = a.Open(filepath.Join(dir, "work")); err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if b, _ := os.ReadFile(shelf.File(config)); string(b) != "{" {
		t.Fatalf("overwritten: %s", b)
	}
}
