package backup

import (
	"archive/zip"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func put(t *testing.T, root, rel, text string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(text), 0644); err != nil {
		t.Fatal(err)
	}
}

// A backup holds the work but .writer/, is skipped when nothing changed,
// keeps the latest few, and unpacks as it was.
func TestMakeListPruneUnpack(t *testing.T) {
	root, dir := t.TempDir(), filepath.Join(t.TempDir(), "备份")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	put(t, root, "work.yaml", "name: 雨城来信\n")
	put(t, root, "chapters/第一章.txt", "整条街的灯是同时灭的。")
	put(t, root, ".writer/desk.json", "{}")
	at := time.Date(2026, 10, 9, 10, 32, 0, 0, time.Local)
	make1 := func(now time.Time) (Info, bool) {
		t.Helper()
		fl, err := Read(root)
		if err != nil {
			t.Fatal(err)
		}
		info, made, err := Make(fl, dir, "雨城/来信", Meta{Work: "w1", Words: 11, Chapters: 1}, 2, now)
		if err != nil {
			t.Fatal(err)
		}
		return info, made
	}
	first, made := make1(at)
	if !made || filepath.Base(first.Path) != "雨城_来信 2026-10-09 10.32.00.zip" {
		t.Fatalf("made %v at %s", made, first.Path)
	}
	if _, made = make1(at.Add(time.Hour)); made {
		t.Fatal("backed up the same files again")
	}
	put(t, root, "chapters/第一章.txt", "整条街的灯是同时灭的。林晚站在巷口。")
	make1(at.Add(2 * time.Hour))
	put(t, root, "chapters/第二章.txt", "停电。")
	make1(at.Add(3 * time.Hour))
	// Another work's backup in the same folder is not this one's.
	put(t, dir, "雨城_来信 2026-10-01 09.00.00.zip", "not a zip")
	list, err := List(dir, "w1")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || !list[0].At.Equal(at.Add(3*time.Hour)) || list[0].Words != 11 || list[0].Size == 0 {
		t.Fatalf("list %+v", list)
	}
	if other, _ := List(dir, "w2"); len(other) != 0 {
		t.Fatalf("another work's: %+v", other)
	}
	// Renamed, the work still finds the backups it made.
	if fl, err := Read(root); err != nil {
		t.Fatal(err)
	} else if _, made, err := Make(fl, dir, "新名字", Meta{Work: "w1"}, 2, at.Add(4*time.Hour)); err != nil || made {
		t.Fatalf("made %v under a new name, %v", made, err)
	}
	into := t.TempDir()
	if err = Unpack(list[0].Path, into); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(into, "chapters", "第二章.txt"))
	if err != nil || string(b) != "停电。" {
		t.Fatalf("unpacked %q, %v", b, err)
	}
	if _, err = os.Stat(filepath.Join(into, ".writer", "desk.json")); !os.IsNotExist(err) {
		t.Fatal(".writer/ was backed up")
	}
	if st, err := os.Stat(filepath.Join(into, ".writer")); err != nil || !st.IsDir() {
		t.Fatal("no .writer/ to open it with")
	}
}

// A folder that is not there, as a disk unplugged, fails rather than
// being made on whatever disk now has the letter.
func TestMakeMissingFolder(t *testing.T) {
	root := t.TempDir()
	put(t, root, "work.yaml", "name: x\n")
	fl, err := Read(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = Make(fl, filepath.Join(t.TempDir(), "gone"), "x", Meta{Work: "w"}, 0, time.Now()); err == nil {
		t.Fatal("backed up into a folder that is not there")
	}
}

// A backup whose entries would be written outside the folder it is
// unpacked into, or into .writer/, is not unpacked: on Windows, the other
// separator and a drive's letter make a path of what is one name elsewhere.
func TestUnpackKeepsToItsFolder(t *testing.T) {
	for _, name := range []string{`..\evil.txt`, `chapters\..\..\evil.txt`, `.writer\desk.json`, `.WRITER/desk.json`, `C:evil.txt`, "../evil.txt", "/evil.txt"} {
		outer := t.TempDir()
		into := filepath.Join(outer, "work")
		if err := os.Mkdir(into, 0755); err != nil {
			t.Fatal(err)
		}
		p := filepath.Join(outer, "bad.zip")
		f, err := os.Create(p)
		if err != nil {
			t.Fatal(err)
		}
		z := zip.NewWriter(f)
		w, err := z.CreateHeader(&zip.FileHeader{Name: name})
		if err == nil {
			_, err = w.Write([]byte("x"))
		}
		if cerr := z.Close(); err == nil {
			err = cerr
		}
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			t.Fatal(err)
		}
		err = Unpack(p, into)
		var stray []string
		filepath.WalkDir(outer, func(at string, d fs.DirEntry, _ error) error {
			rel, _ := filepath.Rel(outer, at)
			in := strings.HasPrefix(rel, "work"+string(filepath.Separator))
			if d != nil && !d.IsDir() && rel != "bad.zip" && (!in || strings.HasPrefix(strings.ToLower(rel), filepath.Join("work", ".writer")+string(filepath.Separator))) {
				stray = append(stray, rel)
			}
			return nil
		})
		if len(stray) > 0 {
			t.Errorf("%q wrote %v (%v)", name, stray, err)
		}
		if runtime.GOOS == "windows" && err == nil {
			t.Errorf("%q was unpacked", name)
		}
	}
}
