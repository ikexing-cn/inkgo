package ai

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"writer/discussion"
	"writer/workspace"
	"writer/writing"
)

// Off, nothing that sends is built: the catalog of a service, the talk,
// the pen and the bookkeeper fail at once, and a bookkeeper already
// running takes nothing in. On again, they are built as before.
func TestOffBuildsNothing(t *testing.T) {
	st, err := workspace.Create(filepath.Join(t.TempDir(), "work"), "测试作品", "网文")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	s := New("")
	var built atomic.Int32
	s.NewWriter = func(*workspace.Store) (*writing.Writer, error) {
		built.Add(1)
		return &writing.Writer{Store: st}, nil
	}
	s.NewBookkeeper = func(*workspace.Store) (*discussion.Bookkeeper, error) {
		built.Add(1)
		return &discussion.Bookkeeper{Store: st}, nil
	}
	s.SetOn(false)
	if _, err := s.Catalog("deepseek"); !errors.Is(err, ErrOff) {
		t.Fatalf("catalog: %v", err)
	}
	if _, err := s.Talk(st, nil, nil); !errors.Is(err, ErrOff) {
		t.Fatalf("talk: %v", err)
	}
	if _, err := s.Book(st); !errors.Is(err, ErrOff) {
		t.Fatalf("book: %v", err)
	}
	if _, err := s.Pen(st); !errors.Is(err, ErrOff) {
		t.Fatalf("pen: %v", err)
	}
	b := s.Bookkeep(st, nil)
	defer b.Stop()
	log := filepath.Join(st.Root(), ".writer", "bookkeeper.log")
	for deadline := time.Now().Add(2 * time.Second); ; time.Sleep(10 * time.Millisecond) {
		if data, _ := os.ReadFile(log); strings.Contains(string(data), ErrOff.Error()) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the bookkeeper did not stop at the switch")
		}
	}
	if n := built.Load(); n != 0 {
		t.Fatalf("built %d with the AI off", n)
	}
	s.SetOn(true)
	if _, err := s.Pen(st); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Book(st); err != nil {
		t.Fatal(err)
	}
	if n := built.Load(); n != 2 {
		t.Fatalf("built %d with the AI on again", n)
	}
}
