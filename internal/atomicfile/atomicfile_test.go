package atomicfile

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestWritePermissionsAndReplacement(t *testing.T) {
	for _, perm := range []os.FileMode{0600, 0644} {
		dir := t.TempDir()
		path, reference := filepath.Join(dir, "target"), filepath.Join(dir, "reference")
		if err := os.WriteFile(reference, []byte("old"), perm); err != nil {
			t.Fatal(err)
		}
		if err := Write(path, []byte("first"), perm); err != nil {
			t.Fatal(err)
		}
		if err := Write(path, []byte("完整\nnext"), perm); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(path)
		if err != nil || string(data) != "完整\nnext" {
			t.Fatalf("read = %q, %v", data, err)
		}
		got, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		want, err := os.Stat(reference)
		if err != nil || got.Mode().Perm() != want.Mode().Perm() {
			t.Fatalf("permissions: %v, reference %v, %v", got, want, err)
		}
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) != 2 {
			t.Fatalf("temporary file left: %v, %v", entries, err)
		}
	}
}

func TestStreamCommitAndDiscard(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "chosen-after-writing")
	if err := os.WriteFile(path, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, commit := range []bool{false, true} {
		f, err := Create(dir, 0600)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Discard()
		for _, part := range []string{"new", "\n完整"} {
			if _, err := f.Write([]byte(part)); err != nil {
				t.Fatal(err)
			}
		}
		data, err := os.ReadFile(path)
		if err != nil || string(data) != "old" {
			t.Fatalf("destination changed before commit: %q, %v", data, err)
		}
		if commit {
			if err := f.Commit(path); err != nil {
				t.Fatal(err)
			}
			data, err = os.ReadFile(path)
			if err != nil || string(data) != "new\n完整" {
				t.Fatalf("committed = %q, %v", data, err)
			}
		} else {
			f.Discard()
		}
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) != 1 {
			t.Fatalf("temporary file left: %v, %v", entries, err)
		}
	}
}

func TestFailuresPreserveDestination(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "target")
	if err := os.Mkdir(path, 0755); err != nil {
		t.Fatal(err)
	}
	check := func(err error, op string) {
		t.Helper()
		var stage *Error
		if !errors.As(err, &stage) || stage.Op != op || err.Error() != stage.Err.Error() || !errors.Is(err, stage.Err) {
			t.Fatalf("%s error = %v", op, err)
		}
	}
	check(Write(filepath.Join(dir, "missing", "target"), nil, 0600), "create")
	check(Write(path, []byte("new"), 0600), "rename")
	f, err := Create(dir, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.file.Close(); err != nil {
		t.Fatal(err)
	}
	check(f.Commit(path), "write")
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 || !entries[0].IsDir() {
		t.Fatalf("failed replacement changed destination: %v, %v", entries, err)
	}
}

func TestConcurrentStreams(t *testing.T) {
	dir := t.TempDir()
	var wg sync.WaitGroup
	var files [16]*File
	for i := range files {
		wg.Go(func() {
			f, err := Create(dir, 0600)
			if err != nil {
				t.Error(err)
				return
			}
			files[i] = f
			data := bytes.Repeat([]byte{byte(i)}, 4096)
			if _, err := f.Write(data); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	seen := map[string]bool{}
	for i, f := range files {
		if f == nil {
			t.Fatal("stream not created")
		}
		defer f.Discard()
		if seen[f.path] {
			t.Fatalf("temporary path reused: %s", f.path)
		}
		seen[f.path] = true
		if err := f.Commit(filepath.Join(dir, "target")); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(filepath.Join(dir, "target"))
		if err != nil || !bytes.Equal(data, bytes.Repeat([]byte{byte(i)}, 4096)) {
			t.Fatalf("mixed or partial write: %d bytes, %v", len(data), err)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("temporary files left: %v, %v", entries, err)
	}
}
