package shelf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func work(t *testing.T, dir string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "work.yaml"), []byte("schema: 1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// The shelf is saved and read back whole, and lists the works last opened
// first.
func TestShelfRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := File(filepath.Join(dir, "providers.yaml"))
	if _, fresh, err := Load(path); err != nil || !fresh {
		t.Fatal(fresh, err)
	}
	now := time.Now().Truncate(time.Second)
	s := Shelf{Library: dir, Cards: true}
	s.Put(Book{Path: filepath.Join(dir, "a"), Name: "甲", Opened: now.Add(-time.Hour)})
	s.Put(Book{Path: filepath.Join(dir, "b"), Name: "乙", Opened: now})
	s.Put(Book{Path: filepath.Join(dir, "c"), Name: "丙"})
	s.Put(Book{Path: filepath.Join(dir, "a"), Name: "甲改", Words: 12}) // keeps when it was opened
	if err := s.Save(path); err != nil {
		t.Fatal(err)
	}
	got, fresh, err := Load(path)
	if err != nil || fresh || !got.Cards || got.Library != dir || len(got.Books) != 3 {
		t.Fatalf("%+v %v %v", got, fresh, err)
	}
	var names []string
	for _, b := range got.Recent() {
		names = append(names, b.Name)
	}
	if strings.Join(names, "|") != "乙|甲改|丙" {
		t.Fatal(names)
	}
	if b := got.Books[got.Find(filepath.Join(dir, "a"))]; b.Words != 12 || !b.Opened.Equal(now.Add(-time.Hour)) {
		t.Fatalf("%+v", b)
	}

	got.Relocate(filepath.Join(dir, "a"), filepath.Join(dir, "d"))
	if got.Find(filepath.Join(dir, "a")) >= 0 || got.Books[got.Find(filepath.Join(dir, "d"))].Name != "甲改" {
		t.Fatalf("%+v", got.Books)
	}
	got.Relocate(filepath.Join(dir, "d"), filepath.Join(dir, "b")) // already there: one entry left
	got.Remove(filepath.Join(dir, "c"))
	if len(got.Books) != 1 || got.Books[0].Name != "乙" {
		t.Fatalf("%+v", got.Books)
	}
}

// A shelf that does not read is an error, and is never taken for an empty
// one.
func TestBrokenShelf(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "works.json")
	for _, bad := range []string{`{"schema":1,"books":[`, `{"schema":2,"books":[]}`, `{"schema":1,"shelves":[]}`, `{"schema":1,"books":[{"path":"rel"}]}`} {
		if err := os.WriteFile(path, []byte(bad), 0644); err != nil {
			t.Fatal(err)
		}
		if _, fresh, err := Load(path); err == nil || fresh {
			t.Fatalf("%s: %v %v", bad, fresh, err)
		}
	}
}

// A first shelf holds the works directly inside the library.
func TestStartScansTheLibrary(t *testing.T) {
	lib := t.TempDir()
	a := work(t, filepath.Join(lib, "雨城来信"))
	work(t, filepath.Join(lib, "深处", "旧城"))
	if err := os.Mkdir(filepath.Join(lib, "杂物"), 0755); err != nil {
		t.Fatal(err)
	}
	s := Start(lib)
	if s.Library != lib || len(s.Books) != 1 || s.Books[0].Path != a || s.Books[0].Name != "雨城来信" {
		t.Fatalf("%+v", s)
	}
	if len(Start(filepath.Join(lib, "没有")).Books) != 0 {
		t.Fatal("a missing library has works")
	}
}
