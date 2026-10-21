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
		t.Fatal("the new work is not open", a.errorText())
	}
	s := readShelf(t, a)
	if r := s.Recent(); len(r) != 2 || r[0].Name != "雨城来信" || !sameFolder(r[1].Path, first) {
		t.Fatalf("%+v", r)
	}

	// The sidebar switches straight to another work: the one open is listed
	// first, and the top of the sidebar says its name alone.
	if err := tt.Click("切换作品"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if r := rows(tt); len(r) != 4 || r[0] != "雨城来信" || r[1] != "测试作品" {
		t.Fatalf("the menu of works lists %q", r)
	}
	kinds := 0
	for _, s := range tt.Texts() {
		if s == "网文" {
			kinds++
		}
	}
	if kinds != 2 {
		t.Fatalf("the kind of a work is said %d times, not once for each in the menu", kinds)
	}
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
	b := newApp(a.ai.ConfigFile())
	b.OpenLast()
	if b.Store == nil || !sameFolder(b.Store.Root(), first) {
		t.Fatal("did not reopen", b.errorText())
	}
	b.Close()
}

// The new-work dialog says what is at the place before the work is made: a
// folder with files in it is said in red and 创建 cannot be pressed, so the
// author is not told only after pressing it.
func TestANewWorkSaysWhereItGoes(t *testing.T) {
	a, tt, _ := fixture(t)
	old, empty := filepath.Join(Library(), "旧稿"), filepath.Join(Library(), "空文件夹")
	for _, dir := range []string{old, empty} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(old, "笔记.txt"), []byte("雨"), 0o644); err != nil {
		t.Fatal(err)
	}
	red := func(s string) bool {
		r, _ := tt.Find(s)
		img := tt.Image()
		for y := int(r.Y); y < int(r.Y+r.H); y++ {
			for x := int(r.X); x < int(r.X+r.W); x++ {
				if c := img.RGBAAt(x, y); int(c.R) > int(c.G)+60 && int(c.R) > int(c.B)+60 {
					return true
				}
			}
		}
		return false
	}
	a.openCreate()
	for _, c := range []struct {
		name, say string
		fits      bool
	}{
		{"新书", "将新建文件夹「新书」。", true},
		{"空文件夹", "文件夹「空文件夹」为空，作品将建在其中。", true},
		{"旧稿", "文件夹「旧稿」中已有文件，请更换名称或位置。", false},
	} {
		a.name = c.name
		tt.Frame()
		tt.Frame()
		if !tt.HasText(c.say) || red(c.say) == c.fits {
			t.Fatalf("%s: red %v, %q", c.name, red(c.say), tt.Texts())
		}
	}
	if err := tt.Click("创建作品"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.books.creating || a.errorText() != "" || a.Store.Work().Name != "测试作品" {
		t.Fatal("pressed where there are files:", a.errorText())
	}
	a.name = "空文件夹"
	tt.Frame()
	if err := tt.Click("创建作品"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.books.creating || !sameFolder(a.Store.Root(), empty) {
		t.Fatal("not made in the empty folder:", a.errorText())
	}
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
		t.Fatal("not renamed", a.errorText())
	}

	a.addBook(t.TempDir())
	if a.errorText() == "" {
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
	a := newApp(config)
	if a.errorText() == "" {
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
